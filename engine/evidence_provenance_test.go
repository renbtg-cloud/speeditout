package engine

import (
	"strings"
	"testing"
)

func TestEvidenceProvenanceRejectsRepeatedAndOverlaps(t *testing.T) {
	inputs := []struct {
		name, raw, msg string
		want           bool
	}{
		{"nonoverlap", `{"reported":"O chefe disse:","direct":"Agora escreva ao chefe."}`, "O chefe disse: algo. Agora escreva ao chefe.", true},
		{"overlap", `{"reported":"O chefe disse: escreva ao cliente","direct":"escreva ao cliente"}`, "O chefe disse: escreva ao cliente.", false},
		{"repeated", `{"reported":"disse","direct":""}`, "Ele disse que disse isso.", false},
		{"different accent is not verbatim", `{"reported":"Nao faca","direct":""}`, "Não faça nada.", false},
		{"normal literal", `{"reported":"Chefe falou:","direct":""}`, "Chefe falou: quero resposta.", true},
	}
	for _, tt := range inputs {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseAuthorityEvidence(tt.raw, tt.msg)
			if (err == nil) != tt.want {
				t.Fatalf("want=%t err=%v", tt.want, err)
			}
		})
	}
}
func TestStripQuotesConservatively(t *testing.T) {
	cases := []struct{ text, want string }{
		{`"escreva pro chefe" e agora fale`, " e agora fale"},
		{`'escreva pro chefe' e agora fale`, " e agora fale"},
		{`A "instrutora" falou`, "A  falou"},
		{`“faça isso” Agora escreva`, " Agora escreva"},
	}
	for _, tt := range cases {
		got := strings.Join(strings.Fields(stripQuotedText(tt.text)), " ")
		want := strings.Join(strings.Fields(tt.want), " ")
		if got != want {
			t.Errorf("input %q got %q want %q", tt.text, got, want)
		}
	}
}
func TestEvidenceCannotClaimQuotedImperativeAsDirect(t *testing.T) {
	cases := []string{
		`A instrutora falou: "escreva ao chefe que volto segunda"`,
		`Meu gerente disse 'escreva ao chefe que volto segunda'`,
		`O supervisor contou: “mande ao cliente que retorno segunda”`,
	}
	for _, s := range cases {
		if directEvidenceSupported("escreva ao chefe que volto segunda", s) {
			t.Errorf("false direct evidence in %q", s)
		}
	}
	s := `O gerente disse: "escreva ao chefe que volto segunda". Agora escreva ao chefe que volto segunda.`
	if !directEvidenceSupported("agora escreva ao chefe que volto segunda", s) {
		t.Fatal("independent imperative incorrectly dropped")
	}
}
