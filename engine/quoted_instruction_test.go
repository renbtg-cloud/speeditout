package engine

import "testing"

func TestReportedQuoteGuardRespectsWhoSpoke(t *testing.T) {
	checks := []struct {
		text     string
		expected bool
	}{
		{`Na reunião o gerente disse: "escreva para o cliente amanhã".`, true},
		{`Uma professora comentou: 'responda ao chefe segunda'.`, true},
		{`Na palestra falaram: “mande ao diretor que voltarei amanhã”.`, true},
		{`A instrutora repetiu: “escreva ao supervisor que volto segunda”.`, true},
		{`Minha gerente disse: 'escreva ao cliente segunda'. Agora escreva ao chefe que retorno segunda.`, false},
		{`Eu mesmo digo: escreva ao meu chefe que volto segunda.`, false},
		{`Escreva ao meu chefe que volto segunda.`, false},
		{`Meu gerente falou: "escreva pro chefe segunda". Não prepare nada.`, false},
		{`Se eu dissesse: 'escreva ao chefe segunda', você enviaria?`, false},
		{`Na aula vi 'escreva pro chefe' mas quem pediu fui eu: escreva ao chefe que volto segunda.`, false},
	}
	for _, tc := range checks {
		_, ok := reportedQuoteGuard(Request{Text: tc.text, Context: map[string]string{"domain": "work"}})
		if ok != tc.expected {
			t.Errorf("guard=%v want=%v: %q", ok, tc.expected, tc.text)
		}
	}
}

func TestReportedQuoteGuardDoesNotProposeDraft(t *testing.T) {
	e := &Engine{Memory: mustMemory(t)}
	req := Request{Text: `A professora disse: "responda para o gerente amanhã".`, Context: map[string]string{"domain": "work"}}
	got := e.Interpret(req)
	if got.Decision != "clarify" || got.RequiresApproval || got.Semantic == nil || got.Semantic.Resolution != "blocked" {
		t.Fatalf("lost quoted boundary: %+v", got)
	}
}
