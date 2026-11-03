package engine

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestModelCircuitBreakerSkipsRepeatedFailuresAndRecovers(t *testing.T) {
	var requests atomic.Int32
	var broken atomic.Bool
	broken.Store(true)
	backend := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if broken.Load() {
			http.Error(w, "offline", 503)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":{"content":"{\"intent\":\"unknown\"}"}}`))
	}))
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	backend.Listener = ln
	backend.Start()
	defer backend.Close()
	m, err := NewLocalModel(backend.URL+"/api/chat", "test", 1, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if err = m.ConfigureFailureCooldown(2, 65*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	req := Request{Text: "oi"}
	for i := 0; i < 2; i++ {
		if _, err := m.Classify(context.Background(), req); err == nil {
			t.Fatal("expected provider failure")
		}
	}
	if requests.Load() != 2 || !m.ModelStats().Open {
		t.Fatalf("breaker failed: %+v requests=%d", m.ModelStats(), requests.Load())
	}
	if _, err := m.Classify(context.Background(), req); err == nil {
		t.Fatal("open breaker should fail immediately")
	}
	if requests.Load() != 2 || m.ModelStats().Skipped != 1 {
		t.Fatal("open breaker still touched provider")
	}
	time.Sleep(80 * time.Millisecond)
	broken.Store(false)
	if intent, err := m.Classify(context.Background(), req); err != nil || intent != "unknown" {
		t.Fatalf("recovery failed intent=%s err=%v", intent, err)
	}
	if m.ModelStats().Open || requests.Load() != 3 {
		t.Fatalf("breaker did not recover: %+v", m.ModelStats())
	}
}

func TestModelCircuitLimitsRejected(t *testing.T) {
	m, err := NewLocalModel("http://127.0.0.1:19999/api/chat", "test", 1, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		n uint32
		d time.Duration
	}{{1, time.Second}, {2, 0}, {2, 2 * time.Hour}, {101, time.Second}} {
		if m.ConfigureFailureCooldown(tt.n, tt.d) == nil {
			t.Errorf("invalid: %+v", tt)
		}
	}
}
