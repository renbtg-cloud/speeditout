package engine

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"
)

func TestSemanticTraceAttributedSpeechAndNoCommand(t *testing.T) {
	mem, _ := NewMemory("")
	e := Engine{Memory: mem}
	req := Request{Text: `O supervisor disse "responda amanhã". Não quero mandar nada.`, Context: map[string]string{"domain": "work"}}
	got := e.Interpret(req)
	if got.Decision != "reply_and_wait" || got.Intent != "do_not_draft" || got.RequiresApproval {
		t.Fatalf("should not draft: %+v", got)
	}
	if got.Semantic == nil || len(got.Semantic.Clauses) == 0 {
		t.Fatalf("no trace: %+v", got)
	}
	if got.Semantic.Clauses[0].Source != "third_party" {
		t.Errorf("source should be third party: %+v", got.Semantic.Clauses)
	}
	if _, err := json.Marshal(got); err != nil {
		t.Fatal(err)
	}
}
func TestSemanticTraceExplicitTimeAndAuthorization(t *testing.T) {
	mem, _ := NewMemory("")
	e := Engine{Memory: mem}
	req := Request{UserID: "u", Workspace: "w", Text: "Escreva pro diretor que vou retomar segunda", Context: map[string]string{"domain": "work", "recipient": "boss"}}
	if err := mem.SetPreference("w", "u", "greeting", "omit", "boss", true); err != nil {
		t.Fatal(err)
	}
	got := e.Interpret(req)
	if got.Decision != "propose_draft" || !got.RequiresApproval || strings.Contains(got.Reply, "Recebi sua mensagem") {
		t.Fatalf("approval/consent violated: %+v", got)
	}
	if got.Semantic == nil || got.Semantic.Resolution != "accepted" {
		t.Fatal("missing structured evidence")
	}
	req.Text = "Redija sem prometer segunda-feira"
	got = e.Interpret(req)
	if got.Decision == "propose_draft" {
		t.Fatalf("promised forbidden deadline: %+v", got)
	}
}
func TestSemanticDisagreementAndSilence(t *testing.T) {
	mem, _ := NewMemory("")
	e := Engine{Memory: mem}
	got := e.Interpret(Request{Text: "Aquele jogo é chato?", Context: map[string]string{"situation": "offered_game"}})
	if got.Decision != "clarify" {
		t.Fatalf("question treated as user rejection: %+v", got)
	}
	got = e.Interpret(Request{Text: "Prefiro brincar de uma coisa diferente.", Context: map[string]string{"situation": "offered_game"}})
	if got.Decision != "reply_and_wait" || got.Intent != "reject_game" {
		t.Fatalf("explicit preference not respected: %+v", got)
	}
}
func TestSemanticParallelSafety(t *testing.T) {
	mem, _ := NewMemory("")
	e := Engine{Memory: mem}
	var wg sync.WaitGroup
	for g := 0; g < 12; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 150; i++ {
				x := e.Interpret(Request{Text: "Mande pro chefe exatamente: \"Vai se foder\"", Context: map[string]string{"domain": "work"}})
				if x.Intent != "literal_message" || !x.RequiresApproval {
					t.Errorf("bad parallel outcome %+v", x)
					return
				}
			}
		}()
	}
	wg.Wait()
}
