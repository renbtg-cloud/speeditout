// Command regress scores frozen, developer-authored corpora against the local
// deterministic core. It never contacts the optional model or changes memory.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/renbtg-cloud/speeditout/engine"
)

type example struct {
	ID       string            `json:"id"`
	Text     string            `json:"text"`
	Context  map[string]string `json:"context"`
	Intent   string            `json:"expected_intent"`
	Decision string            `json:"expected_decision"`
}
type tally struct {
	Failed int `json:"failed"`
	Passed int `json:"passed"`
	Total  int `json:"total"`
}
type priorCase struct {
	Intent   string `json:"intent"`
	Decision string `json:"decision"`
	Correct  bool   `json:"correct"`
}
type summary struct {
	Set              string   `json:"set"`
	Failed           int      `json:"failed"`
	Passed           int      `json:"passed"`
	Total            int      `json:"total"`
	DeltaFailures    int      `json:"delta_failures_vs_reference"`
	NewRegressions   int      `json:"new_case_regressions"`
	NewImprovements  int      `json:"new_case_improvements"`
	ChangedDecisions int      `json:"changed_case_decisions"`
	Errors           []string `json:"errors,omitempty"`
}

func execute() error {
	basePath := flag.String("baseline", "results/v013-go-regressions.json", "frozen previous Go-only counts; read only")
	manifestPath := flag.String("manifest", "results/v013-percase-snapshot.json", "frozen per-case v013 outputs, empty disables per-case checking")
	dir := flag.String("cases-dir", "cases", "case corpus directory")
	out := flag.String("out", "", "optional JSON report path")
	flag.Parse()
	b, err := os.ReadFile(*basePath)
	if err != nil {
		return err
	}
	previous := map[string]tally{}
	if err = json.Unmarshal(b, &previous); err != nil {
		return err
	}
	snapshots := map[string]map[string]priorCase{}
	if *manifestPath != "" {
		frozen, err := os.ReadFile(*manifestPath)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(frozen, &snapshots); err != nil {
			return err
		}
	}
	if len(previous) == 0 {
		return errors.New("empty comparison baseline")
	}
	names := make([]string, 0, len(previous))
	for name := range previous {
		names = append(names, name)
	}
	sort.Strings(names)
	mem, err := engine.NewMemory("")
	if err != nil {
		return err
	}
	defer mem.Close()
	e := engine.Engine{Memory: mem}
	report := make([]summary, 0, len(names))
	regressed := 0
	for _, name := range names {
		if strings.ContainsAny(name, "/\\") || name == "." || name == ".." {
			return fmt.Errorf("unsafe case name %q", name)
		}
		raw, err := os.ReadFile(filepath.Join(*dir, name+".json"))
		if err != nil {
			return err
		}
		var cases []example
		if err := json.Unmarshal(raw, &cases); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		if len(cases) == 0 || len(cases) != previous[name].Total {
			return fmt.Errorf("%s: missing, empty or changed case count", name)
		}
		a := summary{Set: name, Total: len(cases)}
		if *manifestPath != "" && len(snapshots[name]) != len(cases) {
			return fmt.Errorf("%s: missing/mismatched frozen per-case snapshot", name)
		}
		seen := map[string]bool{}
		for _, c := range cases {
			if c.ID == "" || seen[c.ID] || c.Text == "" || c.Intent == "" || c.Decision == "" {
				return fmt.Errorf("%s: malformed or duplicate test id", name)
			}
			seen[c.ID] = true
			got := e.Interpret(engine.Request{Text: c.Text, Context: c.Context, Workspace: "synthetic", UserID: "evaluation"})
			correct := got.Intent == c.Intent && got.Decision == c.Decision
			if correct {
				a.Passed++
			} else {
				a.Failed++
				a.Errors = append(a.Errors, c.ID)
			}
			if *manifestPath != "" {
				before, ok := snapshots[name][c.ID]
				if !ok {
					return fmt.Errorf("%s: missing case id %s in snapshot", name, c.ID)
				}
				if before.Correct && !correct {
					a.NewRegressions++
				}
				if !before.Correct && correct {
					a.NewImprovements++
				}
				if before.Intent != got.Intent || before.Decision != got.Decision {
					a.ChangedDecisions++
				}
			}
		}
		a.DeltaFailures = a.Failed - previous[name].Failed
		if a.DeltaFailures > 0 || a.NewRegressions > 0 {
			regressed++
		}
		report = append(report, a)
		fmt.Printf("%s: %d/%d failed; %d/%d passed; delta %+d; per-case regressions=%d, improvements=%d\n", name, a.Failed, a.Total, a.Passed, a.Total, a.DeltaFailures, a.NewRegressions, a.NewImprovements)
	}
	if *out != "" {
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(*out, append(data, '\n'), 0600); err != nil {
			return err
		}
	}
	if regressed > 0 {
		return fmt.Errorf("%d/%d corpora have aggregate or per-case regressions", regressed, len(report))
	}
	fmt.Printf("No aggregate or per-case regressions across %d/%d tracked corpora (against frozen v0.13 outputs).\n", len(report), len(report))
	return nil
}
func main() {
	if err := execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
