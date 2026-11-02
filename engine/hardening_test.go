package engine

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestMisclassifiedLocalModelCannotInventIntent(t *testing.T) {
	cases := []struct {
		label, text string
		ctx         map[string]string
	}{
		{"reject_game", "Gostei muito, quero brincar de novo!", map[string]string{"situation": "offered_game"}},
		{"did_not_understand", "Entendi tudo, explica o próximo", map[string]string{"situation": "offered_game"}},
		{"ghosting_self_blame", "Meu cachorro saiu do portão, culpa minha", nil},
		{"ghosting_self_blame", "Estou atrasado no relatório, culpa minha", map[string]string{"domain": "work"}},
		{"do_not_draft", "O chefe enviou um email, pode escrever uma resposta?", map[string]string{"domain": "work"}},
		{"defer_work", "Responda ao chefe que já terminei o relatório hoje", map[string]string{"domain": "work"}},
		{"defer_work", "Escreva ao chefe dizendo que segunda não serve", map[string]string{"domain": "work"}},
		{"defer_work", "Meu chefe escreveu, mas não quero responder", map[string]string{"domain": "work"}},
		{"reject_game", "Minha irmã não gostou do jogo, mas eu adorei", map[string]string{"situation": "offered_game"}},
	}
	for _, tc := range cases {
		t.Run(tc.label+"/"+tc.text, func(t *testing.T) {
			if out, ok := Admit(Request{Text: tc.text, Context: tc.ctx}, tc.label); ok {
				t.Fatalf("admitted unsupported label: %+v", out)
			}
		})
	}
}

func TestLocalModelRequestCancellationReleasesSlot(t *testing.T) {
	entered := make(chan struct{}, 4)
	release := make(chan struct{})
	var active atomic.Int64
	provider := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		active.Add(1)
		defer active.Add(-1)
		entered <- struct{}{}
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	provider.Listener = ln
	provider.Start()
	defer provider.Close()
	defer close(release)
	model, err := NewLocalModel(provider.URL+"/api/chat", "test", 1, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	e := &Engine{Memory: mustMemory(t), Model: model}
	input := Request{Text: "Alguém de quem eu gostava me deixou falando sozinho. Deve ter sido por minha culpa."}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan Outcome, 1)
	go func() { done <- e.InterpretContext(ctx, input) }()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		cancel()
		t.Fatal("inference request did not reach provider")
	}
	started := time.Now()
	cancel()
	select {
	case got := <-done:
		if got.Intent != "unknown" || got.Decision != "clarify" || len(model.slots) != 0 {
			t.Fatalf("wrong fallback/slot after cancel: %+v", got)
		}
		if time.Since(started) > 500*time.Millisecond {
			t.Fatalf("cancellation too slow: %s", time.Since(started))
		}
	case <-time.After(1 * time.Second):
		t.Fatal("cancellation not propagated to model")
	}
	canceled, c := context.WithCancel(context.Background())
	c()
	got := e.InterpretContext(canceled, input)
	if got.Intent != "unknown" || len(model.slots) != 0 {
		t.Fatalf("canceled context consumed slot: %+v", got)
	}
	if active.Load() < 0 {
		t.Fatal(fmt.Sprint("invalid active counter", active.Load()))
	}
}
