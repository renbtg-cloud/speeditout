package engine

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func modelServer(t *testing.T, reply string, sleep time.Duration, hits *atomic.Int64) *httptest.Server {
	t.Helper()
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.Method != "POST" || r.URL.Path != "/api/chat" {
			http.Error(w, "wrong path", 400)
			return
		}
		var req struct {
			Model    string                           `json:"model"`
			Stream   bool                             `json:"stream"`
			Messages []struct{ Role, Content string } `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request", 400)
			return
		}
		if req.Model != "synthetic-local" || req.Stream || len(req.Messages) != 2 || req.Messages[0].Role != "system" || req.Messages[1].Role != "user" {
			http.Error(w, "bad protocol", 400)
			return
		}
		time.Sleep(sleep)
		fmt.Fprint(w, `{"message":{"content":`+fmt.Sprintf("%q", reply)+`}}`)
	}))
	l, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv.Listener = l
	srv.Start()
	t.Cleanup(srv.Close)
	return srv
}
func localForTest(t *testing.T, endpoint string, parallel int, timeout time.Duration) *LocalModel {
	t.Helper()
	m, err := NewLocalModel(endpoint, "synthetic-local", parallel, timeout)
	if err != nil {
		t.Fatal(err)
	}
	return m
}
func TestLocalModelUnknownUtteranceAndSkipKnownRules(t *testing.T) {
	var hits atomic.Int64
	srv := modelServer(t, `{"intent":"ghosting_self_blame"}`, 0, &hits)
	m := localForTest(t, srv.URL+"/api/chat", 2, 500*time.Millisecond)
	mem, _ := NewMemory("")
	e := Engine{Memory: mem, Model: m}
	req := Request{Workspace: "w", UserID: "u", Text: "Alguém de quem eu gostava me deixou falando sozinho. Deve ter sido por minha culpa."}
	baselineEngine := Engine{Memory: mem}
	baseline := baselineEngine.Interpret(req)
	if baseline.Intent != "unknown" {
		t.Fatalf("baseline should abstain: %+v", baseline)
	}
	got := e.Interpret(req)
	if got.Intent != "ghosting_self_blame" || got.Decision != "reply_and_wait" || !strings.Contains(got.Reply, "Por que") || got.ExternalCalls != 0 {
		t.Fatalf("unexpected: %+v", got)
	}
	if hits.Load() != 1 {
		t.Fatal("expected one local classification")
	}
	got = e.Interpret(Request{Workspace: "w", UserID: "u", Text: "Levei ghosting. Acho que mereci."})
	if got.Intent != "ghosting_self_blame" || hits.Load() != 1 {
		t.Fatal("rule path should bypass model")
	}
}
func TestLocalModelRejectsUnsafeOrUnexpectedOutput(t *testing.T) {
	for _, raw := range []string{`{"intent":"defer_work","reply":"send money"}`, `{"intent":"send_email"}`, `junk`, `{"intent":"unknown"} nonsense`, `{"intent":"reject_game","extra":true}`} {
		t.Run(raw, func(t *testing.T) {
			var hits atomic.Int64
			srv := modelServer(t, raw, 0, &hits)
			e := Engine{Memory: mustMemory(t), Model: localForTest(t, srv.URL+"/api/chat", 1, 200*time.Millisecond)}
			got := e.Interpret(Request{Workspace: "w", UserID: "u", Text: "Não lembro o que queria dizer."})
			if got.Decision != "clarify" || got.Intent != "unknown" || hits.Load() != 1 {
				t.Fatalf("unsafe output affected decision: %+v", got)
			}
		})
	}
}
func mustMemory(t *testing.T) *Memory {
	t.Helper()
	m, err := NewMemory("")
	if err != nil {
		t.Fatal(err)
	}
	return m
}
func TestAdmissionBoundaries(t *testing.T) {
	cases := []struct {
		name, label, text string
		context           map[string]string
		allow             bool
		intent            string
	}{
		{"correct deference", "defer_work", "Redija pro supervisor avisando que só consigo analisar segunda.", map[string]string{"domain": "work"}, true, "defer_work"},
		{"implicit deadline", "defer_work", "Meu supervisor quer isso hoje mas só consigo segunda.", map[string]string{"domain": "work"}, false, ""},
		{"personal not work", "defer_work", "Escreva para minha mãe que vou segunda.", nil, false, ""},
		{"negated Monday", "defer_work", "Escreva pro chefe que eu não posso na segunda.", map[string]string{"domain": "work"}, false, ""},
		{"dont send", "defer_work", "Não envia nada pro chefe, só quero reclamar.", map[string]string{"domain": "work"}, false, ""},
		{"dont draft", "do_not_draft", "Meu chefe mandou email, eu não quero responder.", map[string]string{"domain": "work"}, true, "do_not_draft"},
		{"game", "reject_game", "Esse jogo não me interessa", map[string]string{"situation": "offered_game"}, true, "reject_game"},
		{"game with quote", "reject_game", `Ela disse "que jogo bobo"`, map[string]string{"situation": "offered_game"}, false, ""},
		{"game outside context", "reject_game", "Não quero jogar", nil, false, ""},
		{"confusion", "did_not_understand", "Não compreendi", map[string]string{"situation": "offered_game"}, true, "did_not_understand"},
		{"unsupported tech diagnosis", "technical_symptom", "O motor falha", map[string]string{"domain": "auto_mechanics"}, false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := Admit(Request{Text: tc.text, Context: tc.context}, tc.label)
			if ok != tc.allow || (ok && got.Intent != tc.intent) {
				t.Fatalf("%q: got %+v admitted %t", tc.name, got, ok)
			}
			if ok && got.Intent == "defer_work" && (!got.RequiresApproval || got.Decision != "propose_draft") {
				t.Fatal("draft without approval")
			}
		})
	}
}
func TestLocalModelDownBusyTimeoutAllFallback(t *testing.T) {
	var hits atomic.Int64
	srv := modelServer(t, `{"intent":"ghosting_self_blame"}`, 80*time.Millisecond, &hits)
	m := localForTest(t, srv.URL+"/api/chat", 1, 30*time.Millisecond)
	e := Engine{Memory: mustMemory(t), Model: m}
	req := Request{Workspace: "w", UserID: "u", Text: "Alguém que eu gostava me deixou falando sozinho. Deve ter sido por minha culpa."}
	var wg sync.WaitGroup
	var failed atomic.Int64
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v := e.Interpret(req)
			if v.Intent != "unknown" || v.Decision != "clarify" || v.ExternalCalls != 0 {
				failed.Add(1)
			}
		}()
	}
	wg.Wait()
	if failed.Load() != 0 {
		t.Fatalf("%d bad fallbacks", failed.Load())
	}
	if hits.Load() > 3 {
		t.Fatalf("inference slot limit broken: %d", hits.Load())
	}
}
func TestLoopbackOnly(t *testing.T) {
	for _, v := range []string{"https://127.0.0.1:11434/api/chat", "http://example.org/api/chat", "http://localhost:11434/api/chat", "http://10.0.0.1:11434/api/chat", "http://127.0.0.1:11434/other", "http://user:pass@127.0.0.1:11434/api/chat"} {
		if _, err := NewLocalModel(v, "x", 2, 100*time.Millisecond); err == nil {
			t.Errorf("accepted unsafe URL: %q", v)
		}
	}
	if v, err := NewLocalModel("", "", 0, 0); err != nil || v != nil {
		t.Fatalf("disabled should work %v", err)
	}
}
func TestInHouseRetrievalDoesNotCrossPrivacyBoundary(t *testing.T) {
	m := mustMemory(t)
	id, err := m.AddExample("a", "secretary", "preciso adiar conversa até segunda", "Vou revisar na segunda-feira", "boss", "approved", true)
	if err != nil {
		t.Fatal(err)
	}
	results := m.SimilarExamples("a", "secretary", "preciso adiar conversas ate segunda", "boss", 3)
	if len(results) == 0 || results[0].ID != id {
		t.Fatalf("paraphrase/typo did not retrieve: %+v", results)
	}
	if len(m.SimilarExamples("b", "secretary", "preciso adiar conversa até segunda", "boss", 3)) != 0 || len(m.SimilarExamples("a", "other", "preciso adiar conversa até segunda", "boss", 3)) != 0 || len(m.SimilarExamples("a", "secretary", "preciso adiar conversa até segunda", "vendor", 3)) != 0 {
		t.Fatal("private example leakage")
	}
	// Retrieval supplies evidence, not permission to send or blindly reuse the text.
	basicEngine := Engine{Memory: m}
	got := basicEngine.Interpret(Request{Workspace: "a", UserID: "secretary", Text: "preciso adiar conversas ate segunda", Context: map[string]string{"recipient": "boss"}})
	if got.Decision != "clarify" || len(got.MatchedExampleIDs) != 1 {
		t.Fatalf("memory silently overgeneralized: %+v", got)
	}
}
