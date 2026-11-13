package scanstore

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/larsartmann/go-codec"
	"github.com/larsartmann/go-cqrs-lite/event/v4"

	"github.com/larsartmann/google-domain-detector/internal/classify"
)

// DNS records are attacker-controlled bytes; a hostile TXT record can carry
// invalid UTF-8, and the CBOR codec (correctly) refuses to decode text
// strings that are not valid UTF-8. One such record (bide-et-musique.com's
// google-site-verification TXT, wave tranco1m-b 2026-11-13) made its event
// undecodable and with it every journal fold that reached it.
//
// Defense is two-sided: encode-side, every DNS-derived string is coerced to
// valid UTF-8 before the event is built (sanitizeObservedPayload); decode-side,
// a payload that fails to decode is repaired once by rewriting invalid text
// strings in place (sanitizeCBORText) and decoding the repaired bytes — the
// journal stays append-only, no event is rewritten.

var (
	errCBORTruncated   = errors.New("cbor rewrite: truncated item")
	errCBORIndefinite  = errors.New("cbor rewrite: indefinite items unsupported")
	errCBOROverrun     = errors.New("cbor rewrite: item overruns payload")
	errCBORReserved    = errors.New("cbor rewrite: reserved length argument")
	errCBORUnsupported = errors.New("cbor rewrite: unsupported CBOR head")
	errCBORTooLarge    = errors.New("cbor rewrite: argument exceeds platform int")
)

// decodeObserved decodes a DomainObservedPayload, repairing poisoned payloads.
// The original decode error wins when repair is impossible.
func decodeObserved(evt event.Event) (DomainObservedPayload, error) {
	p, err := event.DecodePayloadAuto[DomainObservedPayload](evt)
	if err == nil {
		return p, nil
	}

	repaired, rerr := sanitizeCBORText(evt.Payload())
	if rerr != nil {
		return DomainObservedPayload{}, err
	}

	c, cerr := codec.ForEncoding(evt.Encoding())
	if cerr != nil {
		return DomainObservedPayload{}, err
	}

	if derr := c.Decode(repaired, &p); derr != nil {
		return DomainObservedPayload{}, err
	}

	return p, nil
}

// sanitizeObservedPayload coerces every DNS-derived string in the payload to
// valid UTF-8 before encoding. Call it immediately before event.New.
func sanitizeObservedPayload(p *DomainObservedPayload) {
	p.Domain = strings.ToValidUTF8(p.Domain, "�")
	p.RunID = strings.ToValidUTF8(p.RunID, "�")

	p.Err = strings.ToValidUTF8(p.Err, "�")
	if len(p.Evidence) > 0 {
		evs := make([]classify.Evidence, len(p.Evidence))
		copy(evs, p.Evidence)

		for i := range evs {
			evs[i].Host = strings.ToValidUTF8(evs[i].Host, "�")
			evs[i].Record = strings.ToValidUTF8(evs[i].Record, "�")
		}

		p.Evidence = evs
	}
}

// sanitizeCBORText rewrites every CBOR text string in payload that is not
// valid UTF-8, replacing invalid bytes with U+FFFD. It reports whether the
// payload changed. Truncated or malformed CBOR is an error: the caller must
// not mistake unrepairable garbage for a repaired payload.
func sanitizeCBORText(payload []byte) ([]byte, error) {
	out, changed, _, err := rewriteCBORItem(payload, 0)
	if err != nil {
		return nil, err
	}

	if !changed {
		return payload, nil
	}

	return out, nil
}

// rewriteCBORItem rewrites the single CBOR item starting at data[pos],
// returning the rewritten item, whether any byte changed, and the next
// position after the item.
func rewriteCBORItem(data []byte, pos int) (out []byte, changed bool, next int, err error) {
	if pos >= len(data) {
		return nil, false, pos, fmt.Errorf("at byte %d: %w", pos, errCBORTruncated)
	}

	head := data[pos]
	major := head >> 5
	arg := head & 0x1f

	argVal, headLen, indefinite, err := decodeCBORHead(data, pos)
	if err != nil {
		return nil, false, pos, err
	}

	switch major {
	case 0, 1, 7:
		end := pos + headLen
		if arg == 31 && major == 7 {
			end = pos + headLen // break/simple: head only
		}

		return data[pos:end], false, end, nil
	case 2:
		if indefinite {
			return nil, false, pos, fmt.Errorf("byte string at byte %d: %w", pos, errCBORIndefinite)
		}

		end := pos + headLen + argVal
		if end > len(data) {
			return nil, false, pos, fmt.Errorf("byte string at byte %d: %w", pos, errCBOROverrun)
		}

		return data[pos:end], false, end, nil
	case 3:
		if indefinite {
			return nil, false, pos, fmt.Errorf("text string at byte %d: %w", pos, errCBORIndefinite)
		}

		end := pos + headLen + argVal
		if end > len(data) {
			return nil, false, pos, fmt.Errorf("text string at byte %d: %w", pos, errCBOROverrun)
		}

		raw := data[pos+headLen : end]
		if utf8.Valid(raw) {
			return data[pos:end], false, end, nil
		}

		valid := strings.ToValidUTF8(string(raw), "�")

		return append(cborTextHead(len(valid)), valid...), true, end, nil
	case 4:
		if indefinite {
			return nil, false, pos, fmt.Errorf("array at byte %d: %w", pos, errCBORIndefinite)
		}

		cursor := pos + headLen

		var inner []byte

		for range argVal {
			item, itemChanged, nextPos, err := rewriteCBORItem(data, cursor)
			if err != nil {
				return nil, false, pos, err
			}

			inner = append(inner, item...)
			changed = changed || itemChanged
			cursor = nextPos
		}

		if !changed {
			return data[pos:cursor], false, cursor, nil
		}

		return append(cborHead(4, argVal), inner...), true, cursor, nil
	case 5:
		if indefinite {
			return nil, false, pos, fmt.Errorf("map at byte %d: %w", pos, errCBORIndefinite)
		}

		cursor := pos + headLen

		var inner []byte

		for i := 0; i < 2*argVal; i++ {
			item, itemChanged, nextPos, err := rewriteCBORItem(data, cursor)
			if err != nil {
				return nil, false, pos, err
			}

			inner = append(inner, item...)
			changed = changed || itemChanged
			cursor = nextPos
		}

		if !changed {
			return data[pos:cursor], false, cursor, nil
		}

		return append(cborHead(5, argVal), inner...), true, cursor, nil
	case 6:
		item, itemChanged, nextPos, err := rewriteCBORItem(data, pos+headLen)
		if err != nil {
			return nil, false, pos, err
		}

		if !itemChanged {
			return data[pos:nextPos], false, nextPos, nil
		}

		return append(cborHead(6, argVal), item...), true, nextPos, nil
	default:
		return nil, false, pos, fmt.Errorf("major type %d at byte %d: %w", major, pos, errCBORUnsupported)
	}
}

// cborExtraLen maps a head's length argument (24..27) to the number of
// argument bytes that follow the head byte.
var cborExtraLen = [32]int{24: 1, 25: 2, 26: 4, 27: 8}

const maxInt = int(^uint(0) >> 1)

// decodeCBORHead parses the head at data[pos]: its argument value, the head's
// byte length, and whether the item is indefinite (arg 31).
func decodeCBORHead(data []byte, pos int) (argVal, headLen int, indefinite bool, err error) {
	arg := int(data[pos] & 0x1f)

	switch {
	case arg < 24:
		return arg, 1, false, nil
	case arg == 31:
		return 0, 1, true, nil
	case arg >= 28:
		return 0, 0, false, fmt.Errorf("arg %d at byte %d: %w", arg, pos, errCBORReserved)
	}

	extra := cborExtraLen[arg]
	if pos+1+extra > len(data) {
		return 0, 0, false, fmt.Errorf("%d-byte argument at byte %d: %w", extra, pos, errCBORTruncated)
	}

	var v uint64
	for _, b := range data[pos+1 : pos+1+extra] {
		v = v<<8 | uint64(b)
	}

	if v > uint64(maxInt) {
		return 0, 0, false, fmt.Errorf("arg %d at byte %d: %w", v, pos, errCBORTooLarge)
	}

	return int(v), 1 + extra, false, nil
}

// cborHead encodes a minimal CBOR head for the given major type and length.
func cborHead(major, length int) []byte {
	mb := byte(major << 5)

	switch {
	case length < 24:
		return []byte{mb | byte(length)}
	case length <= 0xff:
		return []byte{mb | 24, byte(length)}
	case length <= 0xffff:
		return []byte{mb | 25, byte(length >> 8), byte(length)}
	case int(uint32(length)) == length:
		return []byte{mb | 26, byte(length >> 24), byte(length >> 16), byte(length >> 8), byte(length)}
	default:
		return []byte{
			mb | 27,
			byte(uint64(length) >> 56),
			byte(uint64(length) >> 48),
			byte(uint64(length) >> 40),
			byte(
				uint64(length) >> 32,
			),
			byte(uint64(length) >> 24),
			byte(uint64(length) >> 16),
			byte(uint64(length) >> 8),
			byte(uint64(length)),
		}
	}
}

func cborTextHead(length int) []byte { return cborHead(3, length) }
