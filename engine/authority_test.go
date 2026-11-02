package engine

import (
	"strings"
	"sync"
	"testing"
)

func TestAuthorityScopeReportsAreNotCommands(t *testing.T) {
	mem, err := NewMemory("")
	if err != nil {
		t.Fatal(err)
	}
	defer mem.Close()
	e := Engine{Memory: mem}
	cases := []string{
		`Meu chefe disse: responda ao cliente na segunda-feira.`,
		`O gerente pediu: escreva pro diretor que volto segunda.`,
		`"Escreva pro chefe que eu volto segunda"`,
		`Suponha que eu diga: responda ao chefe que volto segunda.`,
		`O professor citou: responda ao diretor amanhã.`,
	}
	for _, utterance := range cases {
		t.Run(utterance, func(t *testing.T) {
			got := e.Interpret(Request{Text: utterance, Context: map[string]string{"domain": "work"}})
			if got.Decision != "clarify" || got.Intent != "unknown" || got.RequiresApproval {
				t.Fatalf("third party/hypothetical misread as user authority: %+v", got)
			}
			if got.Semantic == nil || got.Semantic.Resolution != "blocked" || len(got.Semantic.Hypotheses) == 0 ||
				len(got.Semantic.Hypotheses[0].Blockers) == 0 {
				t.Fatalf("missing blocking trace: %+v", got)
			}
		})
	}
}

func TestAuthorityScopeIndependentDirectiveAndRefusal(t *testing.T) {
	mem, err := NewMemory("")
	if err != nil {
		t.Fatal(err)
	}
	defer mem.Close()
	e := Engine{Memory: mem}
	cases := []struct{ text, intent, decision string }{
		{`Meu chefe disse que devo responder segunda. Escreva pro chefe que retorno segunda.`, "defer_work", "propose_draft"},
		{`Gerente pediu isso agora. Fica para segunda.`, "defer_work", "propose_draft"},
		{`O supervisor disse: "responda amanhã". Eu não quero mandar nada.`, "do_not_draft", "reply_and_wait"},
		{`Meu chefe disse: "responda amanhã". E qual é sua opinião?`, "question_about_work", "clarify"},
	}
	for _, tc := range cases {
		t.Run(tc.text, func(t *testing.T) {
			got := e.Interpret(Request{Text: tc.text, Context: map[string]string{"domain": "work"}})
			if got.Decision != tc.decision || got.Intent != tc.intent {
				t.Fatalf("%+v", got)
			}
			if strings.Contains(tc.decision, "draft") && !got.RequiresApproval {
				t.Fatal("approval lost")
			}
		})
	}
}

func TestAuthorityPreflightConcurrent(t *testing.T) {
	mem, err := NewMemory("")
	if err != nil {
		t.Fatal(err)
	}
	defer mem.Close()
	e := Engine{Memory: mem}
	var wg sync.WaitGroup
	for k := 0; k < 16; k++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 250; i++ {
				got := e.Interpret(Request{Text: `Meu gerente disse: escreva pro chefe que eu volto segunda.`, Context: map[string]string{"domain": "work"}})
				if got.Decision != "clarify" || got.RequiresApproval {
					t.Errorf("bad cross-thread authority: %+v", got)
					return
				}
			}
		}()
	}
	wg.Wait()
}
