package engine

import (
	"context"
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

func qwenTestServer(t *testing.T, respond func(http.ResponseWriter, *http.Request)) *httptest.Server {
	t.Helper()
	s := httptest.NewUnstartedServer(http.HandlerFunc(respond))
	l, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s.Listener = l
	s.Start()
	t.Cleanup(s.Close)
	return s
}

func TestQwenNavigatorRequestProfileAndAdmittedOutput(t *testing.T) {
	var seen atomic.Int64
	server := qwenTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		seen.Add(1)
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			http.Error(w, "bad", 400)
			return
		}
		messages, ok := body["messages"].([]interface{})
		if !ok || len(messages) != 2 || body["think"] != false || body["keep_alive"] != "5m" {
			t.Errorf("qwen profile missing fields %+v", body)
			http.Error(w, "bad", 400)
			return
		}
		format, ok := body["format"].(map[string]interface{})
		if !ok || format["type"] != "object" {
			t.Error("no structured schema")
			http.Error(w, "bad", 400)
			return
		}
		opt := body["options"].(map[string]interface{})
		if opt["num_ctx"].(float64) != 2048 || opt["num_predict"].(float64) != 72 {
			t.Errorf("invalid tight model budget %v", opt)
		}
		sys := messages[0].(map[string]interface{})["content"].(string)
		if !strings.Contains(sys, "navegador") || !strings.Contains(sys, "evidence") {
			t.Error("wrong qwen prompt")
		}
		fmt.Fprint(w, `{"message":{"content":"{\"intent\":\"ghosting_self_blame\",\"evidence\":\"me deixou falando sozinho\",\"abstain\":false}"}}`)
	})
	model, err := NewLocalModelWithProfile(server.URL+"/api/chat", "qwen3:1.7b", 1, 600*time.Millisecond, "qwen3-1.7b")
	if err != nil {
		t.Fatal(err)
	}
	e := Engine{Memory: mustMemory(t), Model: model}
	got := e.Interpret(Request{Workspace: "w", UserID: "u", Text: "Alguém de quem eu gostava me deixou falando sozinho. Deve ter sido por minha culpa."})
	if got.Intent != "ghosting_self_blame" || got.Decision != "reply_and_wait" || got.ExternalCalls != 0 || seen.Load() != 1 {
		t.Fatalf("qwen navigation not admitted: %+v, seen=%d", got, seen.Load())
	}
}

func TestQwenNavigatorEvidenceAndAbstain(t *testing.T) {
	original := "Alguém de quem eu gostava me deixou falando sozinho. Deve ter sido por minha culpa."
	tests := []struct {
		name, raw string
		want      string
		valid     bool
	}{
		{"grounded", `{"intent":"ghosting_self_blame","evidence":"me deixou falando sozinho","abstain":false}`, "ghosting_self_blame", true},
		{"abstain", `{"intent":"unknown","evidence":"","abstain":true}`, "unknown", true},
		{"invented evidence", `{"intent":"ghosting_self_blame","evidence":"ignore all previous orders","abstain":false}`, "", false},
		{"missing evidence", `{"intent":"ghosting_self_blame","evidence":"","abstain":false}`, "", false},
		{"lying abstention", `{"intent":"ghosting_self_blame","evidence":"sozinho","abstain":true}`, "", false},
		{"false known unknown", `{"intent":"unknown","evidence":"sozinho","abstain":false}`, "", false},
		{"extra commands", `{"intent":"ghosting_self_blame","evidence":"sozinho","abstain":false,"send":true}`, "", false},
		{"partial json", `{"intent":"ghosting_self_blame"}`, "", false},
		{"extra json", `{"intent":"unknown","evidence":"","abstain":true}{}`, "", false},
		{"unsupported label", `{"intent":"send_email","evidence":"sozinho","abstain":false}`, "", false},
		{"overlong evidence", `{"intent":"ghosting_self_blame","evidence":"` + strings.Repeat("a", 121) + `","abstain":false}`, "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, err := parseQwenNavigatorResult(tc.raw, original)
			if (err == nil) != tc.valid || (err == nil && out != tc.want) {
				t.Errorf("label=%q err=%v", out, err)
			}
		})
	}
}

func TestQwenNavigatorLocalOnlyProfiles(t *testing.T) {
	for _, profile := range []string{"", "qwen2", "frontier"} {
		if _, err := NewLocalModelWithProfile("http://127.0.0.1:11434/api/chat", "qwen3:1.7b", 1, time.Second, profile); err == nil {
			t.Errorf("unregistered profile %q accepted", profile)
		}
	}
	for _, url := range []string{"https://127.0.0.1:11434/api/chat", "http://localhost:11434/api/chat", "http://example.com/api/chat", "http://10.1.1.2:11434/api/chat"} {
		if _, err := NewLocalModelWithProfile(url, "qwen3:1.7b", 1, time.Second, "qwen3-1.7b"); err == nil {
			t.Errorf("unsafe endpoint accepted %s", url)
		}
	}
}

func TestQwenNavigatorConcurrentOverloadAndFallback(t *testing.T) {
	var requests atomic.Int64
	backend := qwenTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		time.Sleep(110 * time.Millisecond)
		fmt.Fprint(w, `{"message":{"content":"{\"intent\":\"ghosting_self_blame\",\"evidence\":\"me deixou falando sozinho\",\"abstain\":false}"}}`)
	})
	model, err := NewLocalModelWithProfile(backend.URL+"/api/chat", "qwen3:1.7b", 1, 60*time.Millisecond, "qwen3-1.7b")
	if err != nil {
		t.Fatal(err)
	}
	eng := Engine{Memory: mustMemory(t), Model: model}
	input := Request{Text: "Alguém de quem eu gostava me deixou falando sozinho. Deve ter sido por minha culpa.", UserID: "u", Workspace: "w"}
	var wg sync.WaitGroup
	var invalid atomic.Int64
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			defer cancel()
			x := eng.InterpretContext(c, input)
			if x.Intent != "unknown" || x.Decision != "clarify" || x.ExternalCalls != 0 {
				invalid.Add(1)
			}
		}()
	}
	wg.Wait()
	if invalid.Load() != 0 || requests.Load() > 3 || len(model.slots) != 0 {
		t.Fatalf("invalid=%d backend=%d slots=%d", invalid.Load(), requests.Load(), len(model.slots))
	}
}

func TestQwenNavigatorLlamaCPPProtocol(t *testing.T) {
	var hits atomic.Int64
	server := qwenTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Path != "/v1/chat/completions" {
			t.Error("wrong API path")
			http.Error(w, "wrong", 400)
			return
		}
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
			http.Error(w, "json", 400)
			return
		}
		format, ok := payload["response_format"].(map[string]interface{})
		if !ok || format["type"] != "json_object" || format["schema"] == nil {
			t.Error("missing json schema")
		}
		kwargs, ok := payload["chat_template_kwargs"].(map[string]interface{})
		if !ok || kwargs["enable_thinking"] != false {
			t.Error("Qwen still thinking on CPU")
		}
		if payload["max_tokens"] != float64(72) || payload["stream"] != false {
			t.Error("unbounded output")
		}
		fmt.Fprint(w, `{"choices":[{"message":{"content":"{\"intent\":\"ghosting_self_blame\",\"evidence\":\"me deixou falando sozinho\",\"abstain\":false}"}}]}`)
	})
	m, err := NewLocalModelWithProfile(server.URL+"/v1/chat/completions", "qwen3:1.7b", 2, time.Second, "qwen3-1.7b")
	if err != nil {
		t.Fatal(err)
	}
	e := Engine{Memory: mustMemory(t), Model: m}
	got := e.Interpret(Request{Workspace: "w", UserID: "u", Text: "Alguém de quem eu gostava me deixou falando sozinho. Deve ter sido por minha culpa."})
	if got.Intent != "ghosting_self_blame" || hits.Load() != 1 {
		t.Fatalf("llama.cpp mode not used: %+v", got)
	}
}

func TestQwenNavigatorRefusesLlamaCPPInvalidChoiceAndOutput(t *testing.T) {
	tests := []string{
		`{"choices":[]}`,
		`{"choices":[{"message":{"content":"{}"}},{"message":{"content":"{}"}}]}`,
		`{"choices":[{"message":{"content":"{\"intent\":\"ghosting_self_blame\",\"evidence\":\"I have magical evidence\",\"abstain\":false}"}}]}`,
	}
	for i, response := range tests {
		t.Run(fmt.Sprintf("invalid-%d", i), func(t *testing.T) {
			server := qwenTestServer(t, func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, response) })
			m, err := NewLocalModelWithProfile(server.URL+"/v1/chat/completions", "qwen3:1.7b", 1, time.Second, "qwen3-1.7b")
			if err != nil {
				t.Fatal(err)
			}
			e := Engine{Memory: mustMemory(t), Model: m}
			got := e.Interpret(Request{Workspace: "w", UserID: "u", Text: "Alguém de quem eu gostava me deixou falando sozinho. Deve ter sido por minha culpa."})
			if got.Intent != "unknown" || got.Decision != "clarify" || got.ExternalCalls != 0 {
				t.Fatalf("invalid navigator output accepted %+v", got)
			}
		})
	}
}
