package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func sampleAuthority(choice string) string {
	return fmt.Sprintf(`{"hypotheses":[{"kind":"reported","evidence":"O coordenador comentou"},{"kind":"direct","evidence":"avise o cliente"}],"choice":%q,"abstain":false}`, choice)
}
func TestRankAuthorityValidation(t *testing.T) {
	original := "O coordenador comentou: avise o cliente segunda."
	tests := []struct {
		name, raw string
		valid     bool
	}{
		{"reported", sampleAuthority("reported"), true},
		{"direct", sampleAuthority("direct"), true},
		{"invented evidence", `{"hypotheses":[{"kind":"reported","evidence":"O mago instruiu"},{"kind":"direct","evidence":"avise o cliente"}],"choice":"reported","abstain":false}`, false},
		{"repeated kinds", `{"hypotheses":[{"kind":"reported","evidence":"O coordenador comentou"},{"kind":"reported","evidence":"escreva"}],"choice":"reported","abstain":false}`, false},
		{"missing hypothesis", `{"hypotheses":[{"kind":"reported","evidence":"O coordenador comentou"}],"choice":"reported","abstain":false}`, false},
		{"unselected", `{"hypotheses":[{"kind":"reported","evidence":"O coordenador comentou"},{"kind":"direct","evidence":"avise o cliente"}],"choice":"refusal","abstain":false}`, false},
		{"inconsistent abstention", `{"hypotheses":[{"kind":"reported","evidence":"O coordenador comentou"},{"kind":"uncertain","evidence":""}],"choice":"uncertain","abstain":false}`, false},
		{"extra field", `{"hypotheses":[{"kind":"reported","evidence":"O coordenador comentou"},{"kind":"direct","evidence":"avise o cliente"}],"choice":"reported","abstain":false,"send":true}`, false},
		{"trailing", sampleAuthority("reported") + `{}`, false},
		{"missing abstain", `{"hypotheses":[{"kind":"reported","evidence":"O coordenador comentou"},{"kind":"direct","evidence":"avise o cliente"}],"choice":"reported"}`, false},
		{"missing evidence key", `{"hypotheses":[{"kind":"reported","evidence":"O coordenador comentou"},{"kind":"direct"}],"choice":"reported","abstain":false}`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseAuthorityRanking(tt.raw, original)
			if (err == nil) != tt.valid {
				t.Fatalf("valid=%v err=%v", tt.valid, err)
			}
		})
	}
}
func TestQwenAuthorityModelVetoAndNoGrant(t *testing.T) {
	var calls atomic.Int64
	server := qwenTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
			return
		}
		msgs := payload["messages"].([]interface{})
		if !strings.Contains(msgs[0].(map[string]interface{})["content"].(string), "MODE_AUTHORITY_V012") {
			t.Error("wrong prompt")
		}
		fmt.Fprintf(w, `{"message":{"content":%q}}`, sampleAuthority("reported"))
	})
	model, err := NewLocalModelWithProfile(server.URL+"/api/chat", "qwen-local", 1, time.Second, "qwen3-1.7b")
	if err != nil {
		t.Fatal(err)
	}
	e := &Engine{Memory: mustMemory(t), Model: model}
	quoted := Request{Text: "O coordenador comentou: avise o cliente segunda.", Context: map[string]string{"domain": "work"}}
	// Baseline lexical path incorrectly drafts. The model can veto, not grant.
	base := e.interpretCore(context.Background(), quoted)
	if base.Decision != "propose_draft" {
		t.Fatalf("fixture changed: %+v", base)
	}
	got := e.Interpret(quoted)
	if got.Decision != "clarify" || got.RequiresApproval || got.Intent != "unknown" || got.Semantic == nil || got.Semantic.Resolution != "blocked" {
		t.Fatalf("bad veto %+v", got)
	}
	if calls.Load() != 1 {
		t.Fatalf("calls=%d", calls.Load())
	}
	// A separate explicit request must not be vetoed by the same model claim.
	direct := Request{Text: "O coordenador comentou: avise o cliente segunda. Agora escreva ao meu chefe que volto segunda.", Context: map[string]string{"domain": "work"}}
	got = e.Interpret(direct)
	if got.Decision != "propose_draft" || !got.RequiresApproval {
		t.Fatalf("model was granted authority to override direct user instruction: %+v", got)
	}
}
func TestAuthorityModelUnavailableNeverLosesFallback(t *testing.T) {
	server := qwenTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		http.Error(w, "offline", 500)
	})
	m, err := NewLocalModelWithProfile(server.URL+"/api/chat", "qwen-local", 1, 30*time.Millisecond, "qwen3-1.7b")
	if err != nil {
		t.Fatal(err)
	}
	e := &Engine{Memory: mustMemory(t), Model: m}
	q := Request{Text: "O coordenador comentou: avise o cliente segunda.", Context: map[string]string{"domain": "work"}}
	x := e.Interpret(q)
	if x.Decision != "propose_draft" || !x.RequiresApproval {
		t.Fatalf("fallback broken %+v", x)
	}
}
func TestAuthorityVetoRequiresActualReportCue(t *testing.T) {
	a := AuthorityRanking{Choice: "reported", Hypotheses: []AuthorityHypothesis{{Kind: "reported", Evidence: "avise o cliente"}, {Kind: "direct", Evidence: "segunda"}}}
	if authorityVetoSupported("Escreva ao cliente segunda.", a) {
		t.Fatal("model invented reporting relationship")
	}
	a.Hypotheses[0].Evidence = "O coordenador comentou"
	if authorityVetoSupported("O coordenador comentou: escreva segunda. Agora escreva ao chefe segunda.", a) {
		t.Fatal("explicit user instruction overridden")
	}
}
func TestAuthorityModelParallelSlotLimit(t *testing.T) {
	var seen atomic.Int64
	server := qwenTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		seen.Add(1)
		time.Sleep(100 * time.Millisecond)
		fmt.Fprintf(w, `{"message":{"content":%q}}`, sampleAuthority("reported"))
	})
	model, err := NewLocalModelWithProfile(server.URL+"/api/chat", "qwen", 1, 300*time.Millisecond, "qwen3-1.7b")
	if err != nil {
		t.Fatal(err)
	}
	e := &Engine{Memory: mustMemory(t), Model: model}
	var wg sync.WaitGroup
	var invalid atomic.Int64
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			x := e.Interpret(Request{Text: "O coordenador comentou: avise o cliente segunda.", Context: map[string]string{"domain": "work"}})
			if x.Decision != "clarify" && x.Decision != "propose_draft" {
				invalid.Add(1)
			}
		}()
	}
	wg.Wait()
	if invalid.Load() > 0 || seen.Load() > 2 || len(model.slots) != 0 {
		t.Fatalf("invalid=%d seen=%d slots=%d", invalid.Load(), seen.Load(), len(model.slots))
	}
}
