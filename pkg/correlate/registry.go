// Load-state registry + baseline metric resolver (Phase 2).
//
// Reconcile() stores each entity's final LoadState here. QueryPerf then consults
// BaselineMetric() for the correlated counters so the LIVE perf charts match the
// QuickStats/UsageSummary snapshot produced by the rollup — instead of the stock
// simulator's independent cyclic sample arrays.
//
// Only the core CPU/memory counters are load-derived; everything else falls
// through to the stock data (returns nil here).
package correlate

import (
	"sync"

	"github.com/vmware/govmomi/vim25/types"
)

// vSphere counter IDs we derive from LoadState (see scenarios_impl.go comments).
const (
	counterCPUUsagePct = 2  // cpu.usage.average      — units: 1/100th percent
	counterCPUUsageMhz = 6  // cpu.usagemhz.average   — units: MHz
	counterMemUsagePct = 24 // mem.usage.average      — units: 1/100th percent
	counterMemConsumed = 25 // mem.consumed.average   — units: KB
	counterMemActive   = 26 // mem.active.average     — units: KB
	counterMemShared   = 36 // mem.shared.average     — units: KB
	counterMemBalloon  = 65 // mem.vmmemctl.average   — units: KB
	counterMemSwapped  = 35 // mem.swapped.average    — units: KB
)

var (
	stateMu sync.RWMutex
	states  = map[string]LoadState{} // keyed by morKey
)

// morKey matches the override registry's keying ("Type:Value") so we are immune
// to ServerGUID mismatches between in-process and over-the-wire references.
func morKey(ref types.ManagedObjectReference) string {
	return ref.Type + ":" + ref.Value
}

// SetState records an entity's load state for later metric derivation.
func SetState(ref types.ManagedObjectReference, l LoadState) {
	stateMu.Lock()
	defer stateMu.Unlock()
	states[morKey(ref)] = l
}

// GetState returns the recorded load state and whether one exists.
func GetState(ref types.ManagedObjectReference) (LoadState, bool) {
	stateMu.RLock()
	defer stateMu.RUnlock()
	l, ok := states[morKey(ref)]
	return l, ok
}

// Reset clears all recorded state (used before a fresh Reconcile).
func Reset() {
	stateMu.Lock()
	defer stateMu.Unlock()
	states = map[string]LoadState{}
}

// BaselineMetric returns the load-derived value for a correlated counter, or nil
// to let QueryPerf fall back to the stock sample data. The returned slice is a
// single steady value (QueryPerf adds its own noise across the tick window).
func BaselineMetric(ref types.ManagedObjectReference, counterID int32, instance string) []int64 {
	// Aggregate instance only ("" or "*"); per-instance (per-vCPU/NIC) metrics
	// keep stock behaviour.
	if instance != "" && instance != "*" {
		return nil
	}
	l, ok := GetState(ref)
	if !ok {
		return nil
	}

	switch counterID {
	case counterCPUUsagePct:
		return []int64{int64(l.CPUPct() * 100)} // 1/100th percent
	case counterCPUUsageMhz:
		return []int64{l.CPUUsageMhz}
	case counterMemUsagePct:
		return []int64{int64(l.MemPct() * 100)} // 1/100th percent
	case counterMemConsumed:
		return []int64{l.MemUsageMB * 1024} // MB -> KB
	case counterMemActive:
		return []int64{l.MemActiveMB * 1024}
	case counterMemShared:
		return []int64{l.MemSharedMB * 1024}
	case counterMemBalloon:
		return []int64{l.MemBalloonedMB * 1024}
	case counterMemSwapped:
		return []int64{l.MemSwappedMB * 1024}
	}
	return nil
}
