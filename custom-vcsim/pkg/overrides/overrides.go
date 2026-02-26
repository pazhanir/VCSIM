// Package overrides provides a thread-safe registry for runtime metric and
// property overrides. The scenario controller writes overrides, and the
// custom PerformanceManager / property interceptor reads them.
package overrides

import (
	"fmt"
	"sync"
	"time"

	"github.com/vmware/govmomi/vim25/types"
)

// morKey returns a canonical string key for a ManagedObjectReference,
// using only Type and Value. This avoids map-lookup mismatches caused by
// the ServerGUID field differing between the scenario manager (which gets
// MORs via the govmomi finder, potentially with ServerGUID populated) and
// the QueryPerf handler (which receives MORs from raw SOAP XML where
// ServerGUID is typically empty).
func morKey(ref types.ManagedObjectReference) string {
	return fmt.Sprintf("%s:%s", ref.Type, ref.Value)
}

// MetricOverride describes a set of metric values that should be returned
// instead of the default simulated data for a given entity + counter.
type MetricOverride struct {
	CounterID int32
	// Values to cycle through (one per sample interval).  The engine will
	// pick values[tick % len(values)] when building QueryPerf responses.
	Values []int64
	// Instance (e.g. "", "0", "vmnic0").  Empty string means aggregate.
	Instance string
	// ExpiresAt is optional.  Zero value means "until manually cleared".
	ExpiresAt time.Time
}

// PropertyOverride describes a property value that should be returned instead
// of the real simulator value for a given managed object.
type PropertyOverride struct {
	Property string
	Value    interface{}
	// ExpiresAt is optional.
	ExpiresAt time.Time
}

// Registry is the central thread-safe store for all overrides.
// Map keys use "Type:Value" strings (via morKey) to avoid mismatches
// caused by the ServerGUID field in ManagedObjectReference.
type Registry struct {
	mu         sync.RWMutex
	metrics    map[string][]MetricOverride
	properties map[string][]PropertyOverride
}

// Global singleton.
var global = &Registry{
	metrics:    make(map[string][]MetricOverride),
	properties: make(map[string][]PropertyOverride),
}

// Global returns the singleton registry.
func Global() *Registry { return global }

// ---------- Metric overrides ----------

// SetMetric adds or replaces a metric override for the given entity and counter.
func (r *Registry) SetMetric(ref types.ManagedObjectReference, o MetricOverride) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := morKey(ref)
	list := r.metrics[key]
	// Replace existing override for same counter+instance
	for i, existing := range list {
		if existing.CounterID == o.CounterID && existing.Instance == o.Instance {
			list[i] = o
			r.metrics[key] = list
			return
		}
	}
	r.metrics[key] = append(list, o)
}

// GetMetrics returns all active (non-expired) metric overrides for an entity.
func (r *Registry) GetMetrics(ref types.ManagedObjectReference) []MetricOverride {
	r.mu.RLock()
	defer r.mu.RUnlock()
	now := time.Now()
	var result []MetricOverride
	for _, o := range r.metrics[morKey(ref)] {
		if !o.ExpiresAt.IsZero() && now.After(o.ExpiresAt) {
			continue
		}
		result = append(result, o)
	}
	return result
}

// ClearMetrics removes all metric overrides for an entity.
func (r *Registry) ClearMetrics(ref types.ManagedObjectReference) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.metrics, morKey(ref))
}

// ClearAllMetrics removes all metric overrides.
func (r *Registry) ClearAllMetrics() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.metrics = make(map[string][]MetricOverride)
}

// ---------- Property overrides ----------

// SetProperty adds or replaces a property override.
func (r *Registry) SetProperty(ref types.ManagedObjectReference, o PropertyOverride) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := morKey(ref)
	list := r.properties[key]
	for i, existing := range list {
		if existing.Property == o.Property {
			list[i] = o
			r.properties[key] = list
			return
		}
	}
	r.properties[key] = append(list, o)
}

// GetProperty returns a property override if one exists and is active.
func (r *Registry) GetProperty(ref types.ManagedObjectReference, prop string) (interface{}, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	now := time.Now()
	for _, o := range r.properties[morKey(ref)] {
		if o.Property == prop {
			if !o.ExpiresAt.IsZero() && now.After(o.ExpiresAt) {
				return nil, false
			}
			return o.Value, true
		}
	}
	return nil, false
}

// GetProperties returns all active property overrides for an entity.
func (r *Registry) GetProperties(ref types.ManagedObjectReference) []PropertyOverride {
	r.mu.RLock()
	defer r.mu.RUnlock()
	now := time.Now()
	var result []PropertyOverride
	for _, o := range r.properties[morKey(ref)] {
		if !o.ExpiresAt.IsZero() && now.After(o.ExpiresAt) {
			continue
		}
		result = append(result, o)
	}
	return result
}

// ClearProperties removes all property overrides for an entity.
func (r *Registry) ClearProperties(ref types.ManagedObjectReference) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.properties, morKey(ref))
}

// ClearAll removes all overrides (metrics + properties) for all entities.
func (r *Registry) ClearAll() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.metrics = make(map[string][]MetricOverride)
	r.properties = make(map[string][]PropertyOverride)
}

// ---------- Introspection ----------

// ActiveMetricKeys returns the string keys of all entities with active metric overrides.
func (r *Registry) ActiveMetricKeys() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var keys []string
	for k := range r.metrics {
		keys = append(keys, k)
	}
	return keys
}

// ActivePropertyKeys returns the string keys of all entities with active property overrides.
func (r *Registry) ActivePropertyKeys() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var keys []string
	for k := range r.properties {
		keys = append(keys, k)
	}
	return keys
}
