package scanstore

import (
	"encoding/binary"
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

// CBOR major types and head arguments (RFC 8949 section 3.1).
const (
	cborMajorUint = 0
	cborMajorNeg  = 1
	cborMajorByte = 2
	cborMajorText = 3

	cborMajorArray = 4
	cborMajorMap   = 5
	cborMajorTag   = 6
	cborMajorMisc  = 7

	cborMajorShift = 5
	cborArgMask    = 0x1f

	cborArgOneByte    = 24
	cborArgTwoBytes   = 25
	cborArgFourBytes  = 26
	cborArgEightBytes = 27
	cborArgReservedLo = 28
	cborArgIndefinite = 31

	cborLenMaxOneByte = 0xff
	cborLenMaxTwoByte = 0xffff

	cborBitsPerByte  = 8
	cborMapPairCount = 2

	cborEightByteArgWidth = 8
)

var (
	errCBORTruncated   = errors.New("truncated item")
	errCBORIndefinite  = errors.New("indefinite items unsupported")
	errCBOROverrun     = errors.New("item overruns payload")
	errCBORReserved    = errors.New("reserved length argument")
	errCBORUnsupported = errors.New("unsupported CBOR head")
	errCBORTooLarge    = errors.New("argument exceeds platform int")
)

// decodeObserved decodes a DomainObservedPayload, repairing poisoned payloads.
// The original decode error wins when repair is impossible.
func decodeObserved(evt event.Event) (DomainObservedPayload, error) {
	observation, err := event.DecodePayloadAuto[DomainObservedPayload](evt)
	if err == nil {
		return observation, nil
	}

	repaired, rerr := sanitizeCBORText(evt.Payload())
	if rerr != nil {
		return DomainObservedPayload{}, err
	}

	c, cerr := codec.ForEncoding(evt.Encoding())
	if cerr != nil {
		return DomainObservedPayload{}, err
	}

	if derr := c.Decode(repaired, &observation); derr != nil {
		return DomainObservedPayload{}, err
	}

	return observation, nil
}

// sanitizeObservedPayload coerces every DNS-derived string in the payload to
// valid UTF-8 before encoding. Call it immediately before event.New. Evidence
// is copied first: the slice aliases the scan result, which other sinks read
// concurrently.
func sanitizeObservedPayload(payload *DomainObservedPayload) {
	payload.Domain = strings.ToValidUTF8(payload.Domain, "\ufffd")
	payload.RunID = strings.ToValidUTF8(payload.RunID, "\ufffd")
	payload.Err = strings.ToValidUTF8(payload.Err, "\ufffd")

	if len(payload.Evidence) == 0 {
		return
	}

	evidences := make([]classify.Evidence, 0, len(payload.Evidence))
	for _, ev := range payload.Evidence {
		ev.Host = strings.ToValidUTF8(ev.Host, "\ufffd")
		ev.Record = strings.ToValidUTF8(ev.Record, "\ufffd")
		evidences = append(evidences, ev)
	}

	payload.Evidence = evidences
}

// sanitizeCBORText rewrites every CBOR text string in payload that is not
// valid UTF-8, replacing invalid bytes with U+FFFD. Truncated or malformed
// CBOR is an error: the caller must not mistake unrepairable garbage for a
// repaired payload.
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
func rewriteCBORItem(data []byte, pos int) ([]byte, bool, int, error) {
	if pos >= len(data) {
		return nil, false, pos, fmt.Errorf("at byte %d: %w", pos, errCBORTruncated)
	}

	major, argVal, headLen, indefinite, err := decodeCBORHead(data, pos)
	if err != nil {
		return nil, false, pos, err
	}

	switch major {
	case cborMajorUint, cborMajorNeg, cborMajorMisc:
		return data[pos : pos+headLen], false, pos + headLen, nil
	case cborMajorByte, cborMajorText:
		return rewriteCBORString(data, pos, headLen, argVal, indefinite)
	case cborMajorArray, cborMajorMap:
		return rewriteCBORContainer(data, pos, headLen, major, argVal, indefinite)
	case cborMajorTag:
		return rewriteCBORTag(data, pos, headLen, argVal)
	default:
		return nil, false, pos, fmt.Errorf("major %d at byte %d: %w", major, pos, errCBORUnsupported)
	}
}

// rewriteCBORString handles byte and text strings; only text strings can
// change (invalid UTF-8 becomes U+FFFD).
func rewriteCBORString(
	data []byte, pos, headLen, argVal int, indefinite bool,
) ([]byte, bool, int, error) {
	if indefinite {
		return nil, false, pos, fmt.Errorf("string at byte %d: %w", pos, errCBORIndefinite)
	}

	end := pos + headLen + argVal
	if end > len(data) {
		return nil, false, pos, fmt.Errorf("string at byte %d: %w", pos, errCBOROverrun)
	}

	raw := data[pos+headLen : end]

	isText := data[pos]>>cborMajorShift == cborMajorText
	if !isText || utf8.Valid(raw) {
		return data[pos:end], false, end, nil
	}

	valid := strings.ToValidUTF8(string(raw), "\ufffd")

	return append(cborHead(cborMajorText, len(valid)), valid...), true, end, nil
}

// rewriteCBORContainer handles arrays and maps: argVal elements (twice that
// for maps), each rewritten so a changed child re-headers the container.
func rewriteCBORContainer(
	data []byte, pos, headLen, major, argVal int, indefinite bool,
) ([]byte, bool, int, error) {
	if indefinite {
		return nil, false, pos, fmt.Errorf("container at byte %d: %w", pos, errCBORIndefinite)
	}

	entries := argVal
	if major == cborMajorMap {
		entries = cborMapPairCount * argVal
	}

	changed := false
	cursor := pos + headLen
	inner := make([]byte, 0, len(data)-cursor)

	for range entries {
		item, itemChanged, nextPos, itemErr := rewriteCBORItem(data, cursor)
		if itemErr != nil {
			return nil, false, pos, itemErr
		}

		inner = append(inner, item...)
		changed = itemChanged || changed
		cursor = nextPos
	}

	if !changed {
		return data[pos:cursor], false, cursor, nil
	}

	return append(cborHead(major, argVal), inner...), true, cursor, nil
}

// rewriteCBORTag handles major 6: the tag number is the head argument, the
// tagged item follows.
func rewriteCBORTag(data []byte, pos, headLen, argVal int) ([]byte, bool, int, error) {
	item, itemChanged, nextPos, itemErr := rewriteCBORItem(data, pos+headLen)
	if itemErr != nil {
		return nil, false, pos, itemErr
	}

	if !itemChanged {
		return data[pos:nextPos], false, nextPos, nil
	}

	return append(cborHead(cborMajorTag, argVal), item...), true, nextPos, nil
}

const (
	maxInt    = int(^uint(0) >> 1)
	maxUint32 = int(^uint32(0))
)

// Argument-byte counts for the length arguments 24..27.
const (
	cborExtraOneByte  = 1
	cborExtraTwoBytes = 2
	cborExtraFourByte = 4

	cborHeadSizeFour  = 5 // head byte + 4 argument bytes
	cborHeadSizeEight = 9 // head byte + 8 argument bytes
)

// cborHeadExtraLen reports how many argument bytes follow the head byte for
// the given length argument (24..27).
func cborHeadExtraLen(arg int) int {
	switch arg {
	case cborArgOneByte:
		return cborExtraOneByte
	case cborArgTwoBytes:
		return cborExtraTwoBytes
	case cborArgFourBytes:
		return cborExtraFourByte
	default:
		return cborEightByteArgWidth
	}
}

// decodeCBORHead parses the head at data[pos]: its major type, argument
// value, the head's byte length, and whether the item is indefinite (arg 31).
func decodeCBORHead(data []byte, pos int) (int, int, int, bool, error) {
	head := data[pos]
	major := int(head >> cborMajorShift)
	arg := int(head & cborArgMask)

	switch {
	case arg < cborArgOneByte:
		return major, arg, 1, false, nil
	case arg == cborArgIndefinite:
		return major, 0, 1, true, nil
	case arg >= cborArgReservedLo:
		return major, 0, 0, false, fmt.Errorf("arg %d at byte %d: %w", arg, pos, errCBORReserved)
	}

	extra := cborHeadExtraLen(arg)
	if pos+1+extra > len(data) {
		return major, 0, 0, false, fmt.Errorf("%d-byte argument at byte %d: %w", extra, pos, errCBORTruncated)
	}

	argBytes := data[pos+1 : pos+1+extra]

	var value uint64
	for _, b := range argBytes {
		value = value<<cborBitsPerByte | uint64(b)
	}

	if value > uint64(maxInt) {
		return major, 0, 0, false, fmt.Errorf("arg %d at byte %d: %w", value, pos, errCBORTooLarge)
	}

	return major, int(value), 1 + extra, false, nil
}

// cborHead encodes a minimal CBOR head for the given major type and length.
func cborHead(major, length int) []byte {
	headByte := byte(major << cborMajorShift)

	switch {
	case length < cborArgOneByte:
		return []byte{headByte | byte(length)}
	case length <= cborLenMaxOneByte:
		return []byte{headByte | cborArgOneByte, byte(length)}
	case length <= cborLenMaxTwoByte:
		return append([]byte{headByte | cborArgTwoBytes}, byte(length>>cborBitsPerByte), byte(length))
	case length <= maxUint32:
		out := make([]byte, 0, cborHeadSizeFour)

		return append(append(out, headByte|cborArgFourBytes), binary.BigEndian.AppendUint32(nil, uint32(length))...)
	default:
		out := make([]byte, 0, cborHeadSizeEight)

		return append(append(out, headByte|cborArgEightBytes), binary.BigEndian.AppendUint64(nil, uint64(length))...)
	}
}
