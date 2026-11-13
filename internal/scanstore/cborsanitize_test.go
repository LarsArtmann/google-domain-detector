package scanstore

import (
	"os"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/larsartmann/go-codec"
	"github.com/larsartmann/go-cqrs-lite/event/v4"

	"github.com/larsartmann/google-domain-detector/internal/classify"
)

// The real wave-tranco1m-b payload (event seq 190202, Domain:bide-et-musique.com):
// its google-site-verification TXT record carried raw invalid-UTF-8 bytes,
// making the event undecodable and with it every journal fold that reached it.
const poisonFixture = "testdata/poison-utf8-payload.bin"

func poisonPayload(t *testing.T) []byte {
	t.Helper()

	blob, err := os.ReadFile(poisonFixture)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	return blob
}

func TestSanitizeCBORTextRepairsRealPoisonPayload(t *testing.T) {
	t.Parallel()

	blob := poisonPayload(t)
	if err := (codec.CBORCodec{}).Decode(blob, new(DomainObservedPayload)); err == nil {
		t.Fatal("precondition failed: fixture decodes cleanly, it is no longer the poison case")
	}

	repaired, err := sanitizeCBORText(blob)
	if err != nil {
		t.Fatalf("sanitizeCBORText: %v", err)
	}

	if string(repaired) == string(blob) {
		t.Fatal("sanitizeCBORText left the payload unchanged")
	}

	var repairedPayload DomainObservedPayload
	if err := (codec.CBORCodec{}).Decode(repaired, &repairedPayload); err != nil {
		t.Fatalf("repaired payload decodes: %v", err)
	}

	if repairedPayload.Domain != "bide-et-musique.com" {
		t.Errorf("domain = %q, want the fixture's domain", repairedPayload.Domain)
	}

	for _, ev := range repairedPayload.Evidence {
		if !utf8.ValidString(ev.Record) {
			t.Errorf("evidence record still invalid UTF-8: %q", ev.Record)
		}

		if strings.ContainsRune(ev.Record, '�') && !strings.Contains(ev.Record, "google-site-verification=") {
			t.Errorf("sanitization lost the record prefix: %q", ev.Record)
		}
	}

	if len(repairedPayload.Evidence) == 0 {
		t.Fatal("repaired payload lost all evidence")
	}
}

func TestSanitizeCBORTextLeavesCleanPayloadsByteIdentical(t *testing.T) {
	t.Parallel()

	payload := DomainObservedPayload{
		Domain: "example.ch",
		RunID:  "01TEST",
		Evidence: []classify.Evidence{
			{
				Kind: classify.EvidenceMXGoogle, Platform: classify.PlatformWorkspace, Weight: classify.WeightStrong,
				Host: "example.ch", Record: "10 aspmx.l.google.com.",
			},
		},
		ErrKind:    classify.ErrorNone,
		Ruleset:    classify.CurrentRuleset,
		ObservedAt: event.NewInstantNow(),
	}

	var raw []byte

	c := codec.CBORCodec{}

	raw, err := c.Encode(payload)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	got, err := sanitizeCBORText(raw)
	if err != nil {
		t.Fatalf("sanitizeCBORText: %v", err)
	}

	if string(got) != string(raw) {
		t.Fatal("clean payload was rewritten; sanitizer must be a no-op on valid CBOR")
	}
}

func TestSanitizeObservedPayloadCoercesDNSStrings(t *testing.T) {
	t.Parallel()

	bad := "\xff\xfe hostile \x80"
	payload := DomainObservedPayload{
		Domain: "example.com",
		RunID:  "01TEST",
		Evidence: []classify.Evidence{
			{
				Kind: classify.EvidenceMXGoogle, Platform: classify.PlatformWorkspace, Weight: classify.WeightStrong,
				Host: "example.com", Record: "10 aspmx.l.google.com." + bad,
			},
		},
		Err: bad,
	}
	sanitizeObservedPayload(&payload)

	if !utf8.ValidString(payload.Err) {
		t.Errorf("Err still invalid UTF-8: %q", payload.Err)
	}

	if !utf8.ValidString(payload.Evidence[0].Record) {
		t.Errorf("Record still invalid UTF-8: %q", payload.Evidence[0].Record)
	}

	if !strings.HasPrefix(payload.Evidence[0].Record, "10 aspmx.l.google.com.") {
		t.Errorf("Record lost its prefix: %q", payload.Evidence[0].Record)
	}

	raw, err := (codec.CBORCodec{}).Encode(payload)
	if err != nil {
		t.Fatalf("encode sanitized payload: %v", err)
	}

	var back DomainObservedPayload
	if err := (codec.CBORCodec{}).Decode(raw, &back); err != nil {
		t.Fatalf("decode sanitized payload: %v", err)
	}
}

func TestSanitizeCBORTextRejectsGarbage(t *testing.T) {
	t.Parallel()

	for name, blob := range map[string][]byte{
		"truncated head":     {0xa5},
		"text overruns":      {0x63, 'a', 'b'},
		"reserved length":    {0x5c},
		"array overruns":     {0x82, 0x01},
		"indefinite text":    {0x7f, 0x62, 'o', 'k', 0xff},
		"tag with bad inner": {0xc0, 0x63, 'a'},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if _, err := sanitizeCBORText(blob); err == nil {
				t.Fatalf("sanitizeCBORText(%s) = nil error, want rejection", name)
			}
		})
	}
}
