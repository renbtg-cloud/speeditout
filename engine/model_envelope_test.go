package engine

import (
	"strings"
	"testing"
)

func TestModelEnvelopeNoSilentTruncationOrDuplicateKeys(t *testing.T) {
	good := []string{`{"message":{"content":"{}"}}`, `{"choices":[{"message":{"content":"{}"}}],"timings":{"prompt_ms":12}}`}
	for _, raw := range good {
		var got interface{}
		if err := decodeModelEnvelope(strings.NewReader(raw), &got); err != nil {
			t.Errorf("valid response rejected: %v", err)
		}
	}
	bad := []string{
		`{"message":{"content":"yes"}}{"message":{"content":"no"}}`,
		`{"message":{"content":"yes"},"message":{"content":"no"}}`,
		`{"message":{"content":"yes","content":"no"}}`,
		`{"choices":[{"message":{"content":"yes","content":"no"}}]}`,
		strings.Repeat(" ", modelEnvelopeLimit+1) + `{}`,
		`{"message":{"content":"yes"}}garbage`,
	}
	for _, raw := range bad {
		var got interface{}
		if err := decodeModelEnvelope(strings.NewReader(raw), &got); err == nil {
			t.Errorf("accepted model envelope: %.80q", raw)
		}
	}
}
