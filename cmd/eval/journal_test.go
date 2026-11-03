package main

import (
	"bufio"
	"encoding/json"
	"github.com/renbtg-cloud/speeditout/engine"
	"os"
	"testing"
)

func TestJournalPreservesIndividualResults(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "journal-*.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	mem, err := engine.NewMemory("")
	if err != nil {
		t.Fatal(err)
	}
	defer mem.Close()
	cases := []TestCase{{ID: "one", Text: "", ExpectedIntent: "unknown", ExpectedDecision: "clarify"}, {ID: "two", Text: "Escreva ao chefe que retomo segunda.", Context: map[string]string{"domain": "work"}, ExpectedIntent: "defer_work", ExpectedDecision: "propose_draft"}}
	s, err := runJournal(cases, &engine.Engine{Memory: mem}, "baseline", false, f)
	if err != nil || s.Total != 2 || s.Passed != 2 {
		t.Fatalf("summary %+v err=%v", s, err)
	}
	if _, err := f.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	scanner := bufio.NewScanner(f)
	n := 0
	for scanner.Scan() {
		var record struct {
			Phase       string      `json:"phase"`
			Index       int         `json:"index"`
			Total       int         `json:"total"`
			Observation Observation `json:"observation"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			t.Fatal(err)
		}
		n++
		if record.Phase != "baseline" || record.Index != n || record.Total != 2 || !record.Observation.Correct {
			t.Fatalf("bad journal %+v", record)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("journal has %d records", n)
	}
}
