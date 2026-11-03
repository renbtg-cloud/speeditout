package engine

import (
	"context"
	"errors"
	"sync/atomic"
	"time"
)

// ModelCircuitStats is local only. No user text, model prompt, or secrets leave
// the process through these counters.
type ModelCircuitStats struct {
	Invocations uint64 `json:"invocations"`
	Failures    uint64 `json:"failures"`
	Skipped     uint64 `json:"skipped"`
	Open        bool   `json:"open"`
}

type modelCircuit struct {
	threshold   uint32
	cooldown    time.Duration
	consecutive atomic.Uint32
	openUntil   atomic.Int64
	invocations atomic.Uint64
	failures    atomic.Uint64
	skipped     atomic.Uint64
}

func (m *LocalModel) ConfigureFailureCooldown(after uint32, cooldown time.Duration) error {
	if m == nil {
		return errors.New("model not configured")
	}
	if after == 0 && cooldown == 0 {
		m.circuit = nil
		return nil
	}
	if after < 2 || after > 100 || cooldown < 10*time.Millisecond || cooldown > time.Hour {
		return errors.New("invalid circuit breaker limits")
	}
	m.circuit = &modelCircuit{threshold: after, cooldown: cooldown}
	return nil
}

func (m *LocalModel) ModelStats() ModelCircuitStats {
	if m == nil || m.circuit == nil {
		return ModelCircuitStats{}
	}
	b := m.circuit
	return ModelCircuitStats{
		Invocations: b.invocations.Load(), Failures: b.failures.Load(), Skipped: b.skipped.Load(),
		Open: b.openUntil.Load() > time.Now().UnixNano(),
	}
}

func (m *LocalModel) circuitEnter() error {
	if m == nil || m.circuit == nil {
		return nil
	}
	b := m.circuit
	if b.openUntil.Load() > time.Now().UnixNano() {
		b.skipped.Add(1)
		return errors.New("local model circuit temporarily open")
	}
	b.invocations.Add(1)
	return nil
}

func (m *LocalModel) circuitDone(err error) {
	if m == nil || m.circuit == nil {
		return
	}
	b := m.circuit
	if err == nil {
		b.consecutive.Store(0)
		return
	}
	if errors.Is(err, context.Canceled) {
		return
	}
	b.failures.Add(1)
	if b.consecutive.Add(1) >= b.threshold {
		b.openUntil.Store(time.Now().Add(b.cooldown).UnixNano())
		b.consecutive.Store(0)
	}
}
