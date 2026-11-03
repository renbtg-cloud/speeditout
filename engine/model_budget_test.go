package engine

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestModelBudgetBoundsSlowProviderAndKeepsApproval(t *testing.T) {
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(400 * time.Millisecond):
			fmt.Fprint(w, `{"message":{"content":"{\"reported\":\"\",\"direct\":\"\"}"}}`)
		}
	}))
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv.Listener = ln
	srv.Start()
	defer srv.Close()
	model, err := NewLocalModelWithProfile(srv.URL+"/api/chat", "local", 2, time.Second, "qwen3-1.7b-evidence")
	if err != nil {
		t.Fatal(err)
	}
	mem := mustMemory(t)
	e := &Engine{Memory: mem, Model: model, ModelBudget: 35 * time.Millisecond}
	req := Request{Text: "A instrutora falou: escreva para o cliente que volto segunda.", Context: map[string]string{"domain": "work"}}
	if baseline := e.interpretCore(context.Background(), req); baseline.Decision != "propose_draft" {
		t.Fatalf("fixture changed: %+v", baseline)
	}
	start := time.Now()
	result := e.Interpret(req)
	if took := time.Since(start); took > 200*time.Millisecond {
		t.Fatalf("budget ignored: %v", took)
	}
	if result.Decision != "propose_draft" || !result.RequiresApproval || !strings.Contains(strings.Join(result.Reasons, ","), "unavailable") {
		t.Fatalf("fallback/approval lost: %+v", result)
	}
	if len(model.slots) != 0 {
		t.Fatal("inference semaphore held after budget expiry")
	}
}

func TestModelBudgetConcurrencyAndNoCrossCallBleed(t *testing.T) {
	mem := mustMemory(t)
	e := &Engine{Memory: mem, ModelBudget: 10 * time.Millisecond}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got := e.Interpret(Request{Text: "Não quero enviar mensagem ao chefe. Só quero desabafar.", Context: map[string]string{"domain": "work"}})
			if got.Decision != "reply_and_wait" {
				t.Errorf("intent changed: %+v", got)
			}
		}()
	}
	wg.Wait()
}
