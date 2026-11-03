package engine

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestDecisionInvariantsHoldAcrossSeveralContexts(t *testing.T) {
	e := &Engine{Memory: mustMemory(t)}
	inputs := []Request{
		{Text: "Responda ao chefe que retomo segunda.", Context: map[string]string{"domain": "work"}},
		{Text: "Não responda, só quero desabafar.", Context: map[string]string{"domain": "work"}},
		{Text: "No treinamento ela disse: 'escreva ao chefe que volto segunda'.", Context: map[string]string{"domain": "work"}},
		{Text: "Não entendi essa brincadeira", Context: map[string]string{"situation": "offered_game"}},
		{Text: "Meu carro engasga em subida", Context: map[string]string{"domain": "auto_mechanics"}},
		{Text: ""},
	}
	for _, in := range inputs {
		got := e.Interpret(in)
		if got.DecisionAudit == nil || got.DecisionAudit.ExternalActionAuthorized {
			t.Fatalf("audit absent/unsafe: %+v", got)
		}
		if (got.Decision == "propose_draft") != got.RequiresApproval {
			t.Fatalf("approval invariant: %+v", got)
		}
		if got.DecisionAudit.ApprovalRequired != got.RequiresApproval {
			t.Fatalf("audit wrong: %+v", got)
		}
	}
}

func TestDecisionPolicyRejectsFabricatedActions(t *testing.T) {
	cases := []Outcome{
		{Decision: "propose_draft", Intent: "defer_work", Reply: "email sem permissão", RequiresApproval: false},
		{Decision: "send_email", Intent: "defer_work", Reply: "sent", RequiresApproval: true},
		{Decision: "clarify", Intent: "unknown", Reply: "maybe", ExternalCalls: 1},
		{Decision: "reply_and_wait", Intent: "unknown", Reply: ""},
	}
	for _, x := range cases {
		got := enforceDecisionPolicy(x)
		if got.Decision != "clarify" || got.RequiresApproval || got.DecisionAudit.ExternalActionAuthorized {
			t.Fatalf("unsafe policy: %+v", got)
		}
		if got.Reply == "" {
			t.Fatalf("empty reply after repair %+v", got)
		}
	}
}

func TestModelBudgetIncludesFallbackNavigator(t *testing.T) {
	backend := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(400 * time.Millisecond):
		}
	}))
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	backend.Listener = ln
	backend.Start()
	defer backend.Close()
	model, err := NewLocalModel(backend.URL+"/api/chat", "test", 1, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	e := &Engine{Memory: mustMemory(t), Model: model, ModelBudget: 30 * time.Millisecond}
	req := Request{Text: "Este assunto é meio indefinido, não sei o que fazer."}
	before := time.Now()
	outcome := e.InterpretContext(context.Background(), req)
	if spent := time.Since(before); spent > 175*time.Millisecond {
		t.Fatalf("navigator wasn't budgeted: %v", spent)
	}
	if outcome.Decision != "clarify" || outcome.DecisionAudit.ModelOutcome != "unavailable" {
		t.Fatalf("unexpected fallback: %+v", outcome)
	}
	if len(model.slots) != 0 {
		t.Fatal("model slot leak")
	}
}

func TestDecisionPolicyConcurrent(t *testing.T) {
	e := &Engine{Memory: mustMemory(t)}
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				out := e.Interpret(Request{Text: "Oi"})
				if out.DecisionAudit == nil || out.DecisionAudit.ExternalActionAuthorized {
					t.Errorf("invalid audit: %+v", out)
					return
				}
			}
		}()
	}
	wg.Wait()
}
