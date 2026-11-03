// Speedit out Viper: small, inspectable, offline Go HTTP service.
// Research-only: no authentication/TLS; bind loopback, use synthetic data.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/renbtg-cloud/speeditout/audioquality"
	"github.com/renbtg-cloud/speeditout/engine"
)

type server struct {
	e        *engine.Engine
	requests atomic.Uint64
	failures atomic.Uint64
}
type preferenceRequest struct {
	Workspace string `json:"workspace"`
	UserID    string `json:"user_id"`
	Recipient string `json:"recipient"`
	Key       string `json:"key"`
	Value     string `json:"value"`
	Consent   bool   `json:"consent"`
}
type correctionRequest struct {
	Workspace  string `json:"workspace"`
	UserID     string `json:"user_id"`
	Recipient  string `json:"recipient"`
	Utterance  string `json:"utterance"`
	Before     string `json:"before"`
	After      string `json:"after"`
	Consent    bool   `json:"consent"`
	Generalize bool   `json:"generalize"`
}

func respond(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		respond(w, 405, map[string]string{"error": "method not allowed"})
		return false
	}
	mediaType, _, mediaErr := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if mediaErr != nil || mediaType != "application/json" {
		respond(w, 415, map[string]string{"error": "application/json required"})
		return false
	}
	body, readErr := io.ReadAll(http.MaxBytesReader(w, r.Body, 65536))
	if readErr != nil {
		respond(w, 400, map[string]string{"error": "invalid JSON or request too large"})
		return false
	}
	if err := validateRequestJSON(body); err != nil {
		respond(w, 400, map[string]string{"error": "invalid or ambiguous JSON: " + err.Error()})
		return false
	}
	d := json.NewDecoder(strings.NewReader(string(body)))
	d.DisallowUnknownFields()
	if err := d.Decode(dst); err != nil {
		respond(w, 400, map[string]string{"error": "invalid JSON or request too large: " + err.Error()})
		return false
	}
	var extra any
	if err := d.Decode(&extra); !errors.Is(err, io.EOF) {
		respond(w, 400, map[string]string{"error": "exactly one JSON object required"})
		return false
	}
	return true
}
func (s *server) mux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		respond(w, 200, map[string]string{"status": "ok", "engine": "offline-viper-hybrid"})
	})
	mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, r *http.Request) {
		stats := engine.ModelCircuitStats{}
		if s.e.Model != nil {
			stats = s.e.Model.ModelStats()
		}
		respond(w, 200, map[string]any{"requests": s.requests.Load(), "failures": s.failures.Load(), "model_circuit": stats})
	})
	mux.HandleFunc("POST /v1/audio/assess", func(w http.ResponseWriter, r *http.Request) {
		var req audioquality.Request
		if !decode(w, r, &req) {
			return
		}
		result, err := audioquality.Assess(req)
		if err != nil {
			respond(w, 400, map[string]string{"error": err.Error()})
			return
		}
		respond(w, 200, result)
	})
	mux.HandleFunc("POST /v1/interpret", func(w http.ResponseWriter, r *http.Request) {
		s.requests.Add(1)
		var req engine.Request
		if !decode(w, r, &req) {
			s.failures.Add(1)
			return
		}
		if len(req.Text) > 8192 || len(req.UserID) > 128 || len(req.Workspace) > 128 || len(req.Context) > 32 {
			s.failures.Add(1)
			respond(w, 400, map[string]string{"error": "request fields exceed research limits"})
			return
		}
		if req.UserID == "" || req.Workspace == "" {
			s.failures.Add(1)
			respond(w, 400, map[string]string{"error": "user_id and workspace are required"})
			return
		}
		for k, v := range req.Context {
			if len(k) > 128 || len(v) > 2048 {
				s.failures.Add(1)
				respond(w, 400, map[string]string{"error": "context field too long"})
				return
			}
		}
		respond(w, 200, s.e.InterpretContext(r.Context(), req))
	})
	mux.HandleFunc("POST /v1/preferences", func(w http.ResponseWriter, r *http.Request) {
		var req preferenceRequest
		if !decode(w, r, &req) {
			return
		}
		if !req.Consent {
			respond(w, 403, map[string]string{"error": "explicit consent required"})
			return
		}
		if len(req.Workspace) > 128 || len(req.UserID) > 128 || len(req.Recipient) > 128 || len(req.Value) > 1024 {
			respond(w, 400, map[string]string{"error": "field too long"})
			return
		}
		if err := s.e.Memory.SetPreference(req.Workspace, req.UserID, req.Key, req.Value, req.Recipient, true); err != nil {
			respond(w, 400, map[string]string{"error": err.Error()})
			return
		}
		respond(w, 200, map[string]bool{"saved": true})
	})
	mux.HandleFunc("POST /v1/corrections", func(w http.ResponseWriter, r *http.Request) {
		var req correctionRequest
		if !decode(w, r, &req) {
			return
		}
		if !req.Consent {
			respond(w, 403, map[string]string{"error": "explicit consent required"})
			return
		}
		if req.Workspace == "" || req.UserID == "" || len(req.Workspace) > 128 || len(req.UserID) > 128 || len(req.Recipient) > 128 || len(req.Utterance) > 8192 || len(req.Before) > 8192 || len(req.After) > 8192 {
			respond(w, 400, map[string]string{"error": "invalid field lengths"})
			return
		}
		id, err := s.e.Memory.LearnGreetingCorrection(req.Workspace, req.UserID, req.Utterance, req.Before, req.After, req.Recipient, true, req.Generalize)
		if err != nil {
			respond(w, 400, map[string]string{"error": err.Error()})
			return
		}
		respond(w, 200, map[string]any{"saved": true, "example_id": id, "generalized": req.Generalize})
	})
	return mux
}
func main() {
	addr := flag.String("listen", "127.0.0.1:18761", "address; research default loopback-only")
	memoryPath := flag.String("memory", "", "JSONL memory path; empty is in-memory")
	flagModelURL := flag.String("local-model-url", "", "optional Ollama-compatible HTTP /api/chat endpoint on numeric loopback address only")
	flagModelName := flag.String("local-model-name", "", "local model identifier (required when enabled)")
	flagModelProfile := flag.String("model-profile", "generic", "generic or qwen3-1.7b; specialized navigation without controlling final output")
	flagModelParallel := flag.Int("model-parallel", 2, "maximum simultaneous local inference requests")
	flagModelTimeout := flag.Duration("model-timeout", 30*time.Second, "provider inference deadline; fallback on timeout")
	flagModelBudget := flag.Duration("model-budget", 0, "optional per-interpretation inference budget (e.g. 750ms); 0 keeps historical timeout")
	flagModelFailures := flag.Uint("model-failures", 0, "failures before temporarily skipping the local model; 0 disables breaker")
	flagModelCooldown := flag.Duration("model-cooldown", 0, "cooldown after repeated model errors (e.g. 30s); 0 disables breaker")
	flag.Parse()
	if err := researchListener(*addr); err != nil {
		log.Fatal(err)
	}
	if *flagModelBudget < 0 || *flagModelBudget > 120*time.Second {
		log.Fatal("-model-budget must be between 0 and 120s")
	}
	m, err := engine.NewMemory(*memoryPath)
	if err != nil {
		log.Fatal(err)
	}
	defer m.Close()
	modelURL := flagModelURL
	modelName := flagModelName
	localModel, err := engine.NewLocalModelWithProfile(*modelURL, *modelName, *flagModelParallel, *flagModelTimeout, *flagModelProfile)
	if err != nil {
		log.Fatal(err)
	}
	if *flagModelFailures != 0 || *flagModelCooldown != 0 {
		if localModel == nil {
			log.Fatal("cannot enable circuit without a model")
		}
		if err := localModel.ConfigureFailureCooldown(uint32(*flagModelFailures), *flagModelCooldown); err != nil {
			log.Fatal(err)
		}
	}
	s := &server{e: &engine.Engine{Memory: m, Model: localModel, ModelBudget: *flagModelBudget}}
	writeTimeout := 5 * time.Second
	if localModel != nil && *flagModelTimeout+2*time.Second > writeTimeout {
		writeTimeout = *flagModelTimeout + 2*time.Second
	}
	httpServer := &http.Server{Addr: *addr, Handler: s.mux(), ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: writeTimeout, IdleTimeout: 20 * time.Second, MaxHeaderBytes: 16384}
	done := make(chan os.Signal, 1)
	signal.Notify(done, syscall.SIGTERM, os.Interrupt)
	go func() {
		<-done
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(ctx)
	}()
	log.Printf("speeditout-viper listening on %s (no authentication/TLS; synthetic data only)", *addr)
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
