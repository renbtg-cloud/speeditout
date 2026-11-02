package main

import (
	"github.com/renbtg-cloud/speeditout/engine"
	"testing"
)

func TestEvaluationCountsNoNetworkAsNoModelCalls(t *testing.T) {
	mem, err := engine.NewMemory("")
	if err != nil {
		t.Fatal(err)
	}
	defer mem.Close()
	x := run([]TestCase{
		{ID: "one", Text: "Escreva ao chefe que retorno segunda.", Context: map[string]string{"domain": "work"}, ExpectedIntent: "defer_work", ExpectedDecision: "propose_draft"},
		{ID: "two", Text: "", ExpectedIntent: "unknown", ExpectedDecision: "clarify"},
	}, &engine.Engine{Memory: mem}, "baseline", false)
	if x.Total != 2 || x.Failed != 0 || x.Passed != 2 || x.EvidenceGateConsidered != 0 || x.EvidenceGateUnavailable != 0 {
		t.Fatalf("evaluation counters incorrect: %+v", x)
	}
}
