// Semantic AB comparison: same cases, same owned memory, optional local model.
// The report does not pretend a mock provider is an actual LLM. The CLI must
// only be connected to a model actually running on local loopback.
package main

import (
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/renbtg-cloud/speeditout/engine"
)

type TestCase struct {
	ID               string            `json:"id"`
	Text             string            `json:"text"`
	Context          map[string]string `json:"context"`
	ExpectedIntent   string            `json:"expected_intent"`
	ExpectedDecision string            `json:"expected_decision"`
}
type Observation struct {
	ID                  string   `json:"id"`
	ExpectedIntent      string   `json:"expected_intent"`
	ExpectedDecision    string   `json:"expected_decision"`
	ActualIntent        string   `json:"actual_intent"`
	ActualDecision      string   `json:"actual_decision"`
	Correct             bool     `json:"correct"`
	LatencyMicroseconds int64    `json:"latency_microseconds"`
	Reasons             []string `json:"reasons"`
}
type Summary struct {
	Total                   int           `json:"total"`
	Failed                  int           `json:"failed"`
	Passed                  int           `json:"passed"`
	P50MS                   float64       `json:"p50_ms"`
	P95MS                   float64       `json:"p95_ms"`
	P99MS                   float64       `json:"p99_ms"`
	EvidenceGateConsidered  int           `json:"evidence_gate_considered"`
	EvidenceGateUnavailable int           `json:"evidence_gate_unavailable"`
	EvidenceGateApplied     int           `json:"evidence_gate_applied"`
	EvidenceGateUnproven    int           `json:"evidence_gate_unproven"`
	Results                 []Observation `json:"results"`
}
type Report struct {
	ComparisonEnabled bool     `json:"comparison_enabled"`
	ModelProfile      string   `json:"model_profile,omitempty"`
	ModelName         string   `json:"model_name,omitempty"`
	Baseline          Summary  `json:"baseline"`
	Hybrid            *Summary `json:"hybrid,omitempty"`
	Improvements      int      `json:"improvements"`
	Regressions       int      `json:"regressions"`
	Changed           int      `json:"changed"`
	Notes             []string `json:"notes"`
}

func percent(nums []int64, percentile float64) float64 {
	if len(nums) == 0 {
		return 0
	}
	x := append([]int64{}, nums...)
	sort.Slice(x, func(i, j int) bool { return x[i] < x[j] })
	n := int(float64(len(x)-1)*percentile + 0.999999)
	if n < 0 {
		n = 0
	}
	if n >= len(x) {
		n = len(x) - 1
	}
	return float64(x[n]) / 1000
}
func run(cases []TestCase, eng *engine.Engine, label string, progress bool) Summary {
	result, err := runJournal(cases, eng, label, progress, nil)
	if err != nil {
		panic(err)
	}
	return result
}

// runJournal writes and syncs one observation immediately after completion.
// If a long live-model run is interrupted, the journal preserves the complete
// prefix of measured outcomes instead of silently losing the entire run.
func runJournal(cases []TestCase, eng *engine.Engine, label string, progress bool, journal *os.File) (Summary, error) {
	s := Summary{Total: len(cases), Results: make([]Observation, 0, len(cases))}
	delays := make([]int64, 0, len(cases))
	for i, tc := range cases {
		req := engine.Request{Workspace: "synthetic", UserID: "evaluation", Text: tc.Text, Context: tc.Context}
		start := time.Now()
		got := eng.Interpret(req)
		elapsed := time.Since(start).Microseconds()
		ok := got.Intent == tc.ExpectedIntent && got.Decision == tc.ExpectedDecision
		if ok {
			s.Passed++
		} else {
			s.Failed++
		}
		observation := Observation{ID: tc.ID, ExpectedIntent: tc.ExpectedIntent, ExpectedDecision: tc.ExpectedDecision, ActualIntent: got.Intent, ActualDecision: got.Decision, Correct: ok, LatencyMicroseconds: elapsed, Reasons: got.Reasons}
		s.Results = append(s.Results, observation)
		if journal != nil {
			event := struct {
				Phase       string      `json:"phase"`
				Index       int         `json:"index"`
				Total       int         `json:"total"`
				Observation Observation `json:"observation"`
			}{label, i + 1, len(cases), observation}
			if err := json.NewEncoder(journal).Encode(event); err != nil {
				return s, err
			}
			if err := journal.Sync(); err != nil {
				return s, err
			}
		}
		delays = append(delays, elapsed)
		gateStatus := ""
		for _, reason := range got.Reasons {
			switch reason {
			case "authority_evidence_unavailable":
				s.EvidenceGateUnavailable++
				gateStatus = "unavailable"
			case "authority_evidence_v013":
				s.EvidenceGateApplied++
				gateStatus = "applied"
			case "authority_evidence_unproven", "authority_evidence_independent_request":
				s.EvidenceGateUnproven++
				gateStatus = "unproven"
			}
		}
		if gateStatus != "" {
			s.EvidenceGateConsidered++
		}
		if progress {
			fmt.Fprintf(os.Stderr, "%s %d/%d id=%s correct=%t elapsed_ms=%.1f gate=%s\n", label, i+1, len(cases), tc.ID, ok, float64(elapsed)/1000, gateStatus)
		}
	}
	s.P50MS = percent(delays, 0.50)
	s.P95MS = percent(delays, 0.95)
	s.P99MS = percent(delays, 0.99)
	return s, nil
}
func compare(a, b Summary) (int, int, int) {
	improved, regressed, changed := 0, 0, 0
	for i := range a.Results {
		x, y := a.Results[i], b.Results[i]
		if x.ActualIntent != y.ActualIntent || x.ActualDecision != y.ActualDecision {
			changed++
		}
		if !x.Correct && y.Correct {
			improved++
		}
		if x.Correct && !y.Correct {
			regressed++
		}
	}
	return improved, regressed, changed
}
func main() {
	file := flag.String("cases", "cases/holdout-v1.json", "evaluation cases: do not train on holdout")
	output := flag.String("out", "", "JSON report path")
	endpoint := flag.String("local-model-url", "", "optional Ollama-compatible loopback /api/chat endpoint")
	name := flag.String("local-model-name", "", "model name")
	profile := flag.String("model-profile", "generic", "generic, qwen3-1.7b, or qwen3-1.7b-evidence")
	parallel := flag.Int("model-parallel", 1, "maximum concurrent model calls")
	timeout := flag.Duration("model-timeout", 10*time.Second, "local model timeout on CPU")
	budget := flag.Duration("model-budget", 0, "optional inference budget per sample; 0 for historical timeout")
	modelFailures := flag.Uint("model-failures", 0, "optional consecutive error threshold")
	modelCooldown := flag.Duration("model-cooldown", 0, "optional local model circuit cooldown")
	progress := flag.Bool("progress", false, "per-case status to stderr (no text content)")
	journalPath := flag.String("journal", "", "durable per-case JSONL journal; preserves partial live-model runs")
	flag.Parse()
	if *budget < 0 || *budget > 120*time.Second {
		fmt.Fprintln(os.Stderr, "-model-budget must be between 0 and 120s")
		os.Exit(2)
	}
	b, err := os.ReadFile(*file)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	var cases []TestCase
	if err := json.Unmarshal(b, &cases); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	mem, err := engine.NewMemory("")
	if err != nil {
		panic(err)
	}
	defer mem.Close()
	var journal *os.File
	if *journalPath != "" {
		journal, err = os.OpenFile(*journalPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		defer journal.Close()
		header := map[string]any{"type": "run_header", "corpus_sha256": fmt.Sprintf("%x", sha256.Sum256(b)), "case_count": len(cases), "model_profile": *profile, "model_enabled": *endpoint != ""}
		if err := json.NewEncoder(journal).Encode(header); err != nil {
			panic(err)
		}
		if err := journal.Sync(); err != nil {
			panic(err)
		}
	}
	r := Report{ModelProfile: *profile, ModelName: *name, Notes: []string{"Accuracy means exact agreement with the stated intent and decision. These are developer-authored scenarios, not a human clinical or usability evaluation.", "Local-model inference is counted only when an actual model endpoint is supplied; synthetic providers cannot establish LLM ability.", "Latency is sequential local elapsed time, NOT throughput under concurrency."}}
	r.Baseline, err = runJournal(cases, &engine.Engine{Memory: mem}, "baseline", *progress, journal)
	if err != nil {
		fmt.Fprintln(os.Stderr, "journal failure:", err)
		os.Exit(2)
	}
	if *endpoint != "" {
		model, err := engine.NewLocalModelWithProfile(*endpoint, *name, *parallel, *timeout, *profile)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		r.ComparisonEnabled = true
		if *modelFailures != 0 || *modelCooldown != 0 {
			if err := model.ConfigureFailureCooldown(uint32(*modelFailures), *modelCooldown); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(2)
			}
		}
		x, journalErr := runJournal(cases, &engine.Engine{Memory: mem, Model: model, ModelBudget: *budget}, "hybrid", *progress, journal)
		if journalErr != nil {
			fmt.Fprintln(os.Stderr, "journal failure:", journalErr)
			os.Exit(2)
		}
		r.Hybrid = &x
		r.Improvements, r.Regressions, r.Changed = compare(r.Baseline, x)
	}
	encoded, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		panic(err)
	}
	fmt.Println(string(encoded))
	if *output != "" {
		if err := os.WriteFile(*output, append(encoded, '\n'), 0600); err != nil {
			panic(err)
		}
	}
}
