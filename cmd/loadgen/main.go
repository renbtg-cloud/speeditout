// High-concurrency HTTP load generator. Emits throughput, latency and correctness.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type hist struct {
	mu        sync.Mutex
	latencies []int64
}

func (h *hist) add(v int64) { h.mu.Lock(); h.latencies = append(h.latencies, v); h.mu.Unlock() }

type testCase struct {
	ID               string            `json:"id"`
	Text             string            `json:"text"`
	Context          map[string]string `json:"context"`
	ExpectedIntent   string            `json:"expected_intent"`
	ExpectedDecision string            `json:"expected_decision"`
	Required         []string          `json:"required"`
	Forbidden        []string          `json:"forbidden"`
}
type workload struct {
	body []byte
	tc   testCase
}
type reply struct {
	Decision      string `json:"decision"`
	Intent        string `json:"intent"`
	Reply         string `json:"reply"`
	ExternalCalls int    `json:"external_calls"`
}
type report struct {
	Version       string   `json:"version"`
	Workload      string   `json:"workload"`
	Cases         int      `json:"cases"`
	URL           string   `json:"url"`
	Concurrency   int      `json:"concurrency"`
	DurationSec   float64  `json:"duration_sec"`
	Requests      int64    `json:"requests"`
	Failures      int64    `json:"failures"`
	Passed        int64    `json:"passed"`
	ThroughputRPS float64  `json:"throughput_rps"`
	P50MS         float64  `json:"p50_ms"`
	P95MS         float64  `json:"p95_ms"`
	P99MS         float64  `json:"p99_ms"`
	MaxMS         float64  `json:"max_ms"`
	ErrorExamples []string `json:"error_examples"`
	Disclaimer    string   `json:"disclaimer"`
}

func main() {
	url := flag.String("url", "http://127.0.0.1:18761/v1/interpret", "endpoint")
	clients := flag.Int("c", 32, "number of concurrent clients")
	duration := flag.Duration("duration", 3*time.Second, "test duration")
	casesPath := flag.String("cases", "", "optional path to mixed scenario JSON corpus")
	output := flag.String("out", "", "optional JSON report path")
	flag.Parse()
	if *clients < 1 || *clients > 2000 || *duration <= 0 {
		fmt.Fprintln(os.Stderr, "invalid concurrency or duration")
		os.Exit(2)
	}
	cases := []testCase{{ID: "fixed_work_monday", Text: "Fala pro meu chefe tomar no cu, ele espera até segunda!", Context: map[string]string{"domain": "work", "recipient": "boss"}, ExpectedIntent: "defer_work", ExpectedDecision: "propose_draft", Required: []string{"segunda-feira"}, Forbidden: []string{"tomar no cu"}}}
	mode := "fixed"
	if *casesPath != "" {
		content, err := os.ReadFile(*casesPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		if err = json.Unmarshal(content, &cases); err != nil || len(cases) == 0 {
			fmt.Fprintln(os.Stderr, "invalid scenario corpus", err)
			os.Exit(2)
		}
		mode = "mixed_corpus"
	}
	work := make([]workload, 0, len(cases))
	for _, tc := range cases {
		b, err := json.Marshal(map[string]any{"user_id": "load", "workspace": "bench", "text": tc.Text, "context": tc.Context})
		if err != nil {
			panic(err)
		}
		work = append(work, workload{b, tc})
	}
	tr := &http.Transport{MaxIdleConns: *clients + 16, MaxIdleConnsPerHost: *clients + 16, MaxConnsPerHost: *clients + 16, IdleConnTimeout: 15 * time.Second, DialContext: (&net.Dialer{Timeout: 2 * time.Second}).DialContext}
	defer tr.CloseIdleConnections()
	cli := &http.Client{Timeout: 3 * time.Second, Transport: tr}
	runOne := func(w workload) (int64, string) {
		req, _ := http.NewRequest("POST", *url, bytes.NewReader(w.body))
		req.Header.Set("Content-Type", "application/json")
		t0 := time.Now()
		resp, err := cli.Do(req)
		elapsed := time.Since(t0).Nanoseconds()
		if err != nil {
			return elapsed, err.Error()
		}
		var got reply
		de := json.NewDecoder(io.LimitReader(resp.Body, 8192)).Decode(&got)
		_ = resp.Body.Close()
		if resp.StatusCode != 200 || de != nil {
			return elapsed, fmt.Sprintf("case=%s status=%d decode=%v", w.tc.ID, resp.StatusCode, de)
		}
		if got.Decision != w.tc.ExpectedDecision || got.Intent != w.tc.ExpectedIntent || got.ExternalCalls != 0 {
			return elapsed, fmt.Sprintf("case=%s decision=%s intent=%s externalCalls=%d", w.tc.ID, got.Decision, got.Intent, got.ExternalCalls)
		}
		for _, r := range w.tc.Required {
			if !strings.Contains(got.Reply, r) {
				return elapsed, fmt.Sprintf("case=%s required missing %q", w.tc.ID, r)
			}
		}
		for _, r := range w.tc.Forbidden {
			if strings.Contains(strings.ToLower(got.Reply), strings.ToLower(r)) {
				return elapsed, fmt.Sprintf("case=%s forbidden present %q", w.tc.ID, r)
			}
		}
		return elapsed, ""
	}
	// Warm up TCP connections and handlers; not part of measurements.
	for i := 0; i < *clients; i++ {
		runOne(work[i%len(work)])
	}
	var passed, failed atomic.Int64
	samples := &hist{latencies: make([]int64, 0, 50000)}
	var errorsMu sync.Mutex
	errorsList := make([]string, 0, 8)
	recordError := func(msg string) {
		failed.Add(1)
		errorsMu.Lock()
		if len(errorsList) < 8 {
			errorsList = append(errorsList, msg)
		}
		errorsMu.Unlock()
	}
	start := time.Now()
	stop := start.Add(*duration)
	var wg sync.WaitGroup
	for j := 0; j < *clients; j++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			n := 0
			for time.Now().Before(stop) {
				index := ((worker * 37) + (n * 17)) % len(work)
				n++
				elapsed, msg := runOne(work[index])
				samples.add(elapsed)
				if msg != "" {
					recordError(msg)
				} else {
					passed.Add(1)
				}
			}
		}(j)
	}
	wg.Wait()
	secs := time.Since(start).Seconds()
	sort.Slice(samples.latencies, func(i, j int) bool { return samples.latencies[i] < samples.latencies[j] })
	pct := func(p float64) float64 {
		if len(samples.latencies) == 0 {
			return 0
		}
		idx := int(math.Ceil(float64(len(samples.latencies))*p)) - 1
		if idx < 0 {
			idx = 0
		}
		return math.Round(float64(samples.latencies[idx])/1e4) / 100
	}
	r := report{Version: "0.16.0", Workload: mode, Cases: len(cases), URL: *url, Concurrency: *clients, DurationSec: math.Round(secs*1000) / 1000, Requests: passed.Load() + failed.Load(), Failures: failed.Load(), Passed: passed.Load(), ThroughputRPS: math.Round(float64(passed.Load()+failed.Load())/secs*10) / 10, P50MS: pct(.5), P95MS: pct(.95), P99MS: pct(.99), MaxMS: pct(1), ErrorExamples: errorsList, Disclaimer: "HTTP+JSON+rule-based interpretation only, not audio/LLM; single host contends for client/server CPU; predefined cases are not independent holdout evaluation."}
	b, _ := json.MarshalIndent(r, "", "  ")
	fmt.Println(string(b))
	if *output != "" {
		if err := os.WriteFile(*output, append(b, '\n'), 0644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	if failed.Load() > 0 {
		os.Exit(1)
	}
}
