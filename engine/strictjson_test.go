package engine

import (
	"strings"
	"testing"
)

func TestModelJSONMustHaveUnambiguousKeyPaths(t *testing.T) {
	valid := []string{
		`{"reported":"a gerente disse","direct":""}`,
		`{"x":[{"a":true},{"a":false}],"other":null}`,
	}
	for _, s := range valid {
		if err := validateModelJSON(s, 5000); err != nil {
			t.Fatalf("reject valid model JSON: %v", err)
		}
	}
	invalid := []string{
		`{"reported":"","reported":"manager said","direct":""}`,
		`{"hypotheses":[{"kind":"direct","kind":"reported"}]}`,
		`{"a":[{"k":1,"k":2}]}`,
		`{"a":1} {"b":2}`,
		``, `{`, `[1,2`, `{"x":1,}`, `"some text" true`,
		strings.Repeat("[", 17) + "0" + strings.Repeat("]", 17),
	}
	for _, s := range invalid {
		if err := validateModelJSON(s, 5000); err == nil {
			t.Errorf("accepted ambiguous JSON: %q", s)
		}
	}
	if err := validateModelJSON("{}", 1); err == nil {
		t.Error("size limit ignored")
	}
	if err := validateModelJSON(string([]byte{'{', '"', 'x', '"', ':', '"', 0xff, '"', '}'}), 100); err == nil {
		t.Error("invalid UTF8 accepted")
	}
}

func TestEvidenceParserRejectsDuplicateKeys(t *testing.T) {
	input := "A coordenadora contou: escreva ao chefe amanhã."
	for _, s := range []string{
		`{"reported":"A coordenadora contou:","direct":"","direct":"escreva ao chefe amanhã."}`,
		`{"reported":"A coordenadora contou:","direct":"","reported":""}`,
	} {
		if _, err := parseAuthorityEvidence(s, input); err == nil {
			t.Errorf("allowed duplicate keys: %s", s)
		}
	}
}
