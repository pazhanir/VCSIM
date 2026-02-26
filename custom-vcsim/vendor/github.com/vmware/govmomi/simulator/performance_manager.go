// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: Apache-2.0

package simulator

import (
	"hash/fnv"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/vmware/govmomi/simulator/esx"
	"github.com/vmware/govmomi/simulator/vpx"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"
)

// MetricOverrideFunc, when set, is called for each entity+counter during
// QueryPerf to allow external code to override the default metric data.
// If it returns a non-nil slice, those values replace the default sample
// data for that entity+counter combination.
// The function signature is: func(entity MOR, counterID int32, instance string) []int64
var MetricOverrideFunc func(types.ManagedObjectReference, int32, string) []int64

var realtimeProviderSummary = types.PerfProviderSummary{
	CurrentSupported: true,
	SummarySupported: true,
	RefreshRate:      20,
}

var historicProviderSummary = types.PerfProviderSummary{
	CurrentSupported: false,
	SummarySupported: true,
	RefreshRate:      -1,
}

type PerformanceManager struct {
	mo.PerformanceManager
	vmMetrics         []types.PerfMetricId
	hostMetrics       []types.PerfMetricId
	rpMetrics         []types.PerfMetricId
	clusterMetrics    []types.PerfMetricId
	datastoreMetrics  []types.PerfMetricId
	datacenterMetrics []types.PerfMetricId
	perfCounterIndex  map[int32]types.PerfCounterInfo
	metricData        map[string]map[int32][]int64
}

func (m *PerformanceManager) init(r *Registry) {
	if r.IsESX() {
		m.PerfCounter = esx.PerfCounter
		m.HistoricalInterval = esx.HistoricalInterval
		m.hostMetrics = esx.HostMetrics
		m.vmMetrics = esx.VmMetrics
		m.rpMetrics = esx.ResourcePoolMetrics
		m.metricData = esx.MetricData
	} else {
		m.PerfCounter = vpx.PerfCounter
		m.HistoricalInterval = vpx.HistoricalInterval
		m.hostMetrics = vpx.HostMetrics
		m.vmMetrics = vpx.VmMetrics
		m.rpMetrics = vpx.ResourcePoolMetrics
		m.clusterMetrics = vpx.ClusterMetrics
		m.datastoreMetrics = vpx.DatastoreMetrics
		m.datacenterMetrics = vpx.DatacenterMetrics
		m.metricData = vpx.MetricData
	}
	m.perfCounterIndex = make(map[int32]types.PerfCounterInfo, len(m.PerfCounter))
	for _, p := range m.PerfCounter {
		m.perfCounterIndex[p.Key] = p
	}
}

func (p *PerformanceManager) QueryPerfCounter(ctx *Context, req *types.QueryPerfCounter) soap.HasFault {
	body := new(methods.QueryPerfCounterBody)
	body.Res = new(types.QueryPerfCounterResponse)
	body.Res.Returnval = make([]types.PerfCounterInfo, len(req.CounterId))
	for i, id := range req.CounterId {
		body.Res.Returnval[i] = p.perfCounterIndex[id]
	}
	return body
}

func (p *PerformanceManager) QueryPerfProviderSummary(ctx *Context, req *types.QueryPerfProviderSummary) soap.HasFault {
	body := new(methods.QueryPerfProviderSummaryBody)
	body.Res = new(types.QueryPerfProviderSummaryResponse)

	// The entity must exist
	if ctx.Map.Get(req.Entity) == nil {
		body.Fault_ = Fault("", &types.InvalidArgument{
			InvalidProperty: "Entity",
		})
		return body
	}

	switch req.Entity.Type {
	case "VirtualMachine", "HostSystem", "ResourcePool":
		body.Res.Returnval = realtimeProviderSummary
	default:
		body.Res.Returnval = historicProviderSummary
	}
	body.Res.Returnval.Entity = req.Entity
	return body
}

func (p *PerformanceManager) buildAvailablePerfMetricsQueryResponse(ids []types.PerfMetricId, numCPU int, datastoreURL string) *types.QueryAvailablePerfMetricResponse {
	r := new(types.QueryAvailablePerfMetricResponse)
	r.Returnval = make([]types.PerfMetricId, 0, len(ids))
	for _, id := range ids {
		switch id.Instance {
		case "$cpu":
			for i := 0; i < numCPU; i++ {
				r.Returnval = append(r.Returnval, types.PerfMetricId{CounterId: id.CounterId, Instance: strconv.Itoa(i)})
			}
		case "$physDisk":
			if datastoreURL != "" {
				r.Returnval = append(r.Returnval, types.PerfMetricId{CounterId: id.CounterId, Instance: datastoreURL})
			}
		case "$file":
			r.Returnval = append(r.Returnval, types.PerfMetricId{CounterId: id.CounterId, Instance: "DISKFILE"})
			r.Returnval = append(r.Returnval, types.PerfMetricId{CounterId: id.CounterId, Instance: "DELTAFILE"})
			r.Returnval = append(r.Returnval, types.PerfMetricId{CounterId: id.CounterId, Instance: "SWAPFILE"})
			r.Returnval = append(r.Returnval, types.PerfMetricId{CounterId: id.CounterId, Instance: "OTHERFILE"})
		default:
			r.Returnval = append(r.Returnval, types.PerfMetricId{CounterId: id.CounterId, Instance: id.Instance})
		}
	}
	// Add a CounterId without a corresponding PerfCounterInfo entry. See issue #2835
	r.Returnval = append(r.Returnval, types.PerfMetricId{CounterId: 10042})
	return r
}

func (p *PerformanceManager) queryAvailablePerfMetric(ctx *Context, entity types.ManagedObjectReference, interval int32) *types.QueryAvailablePerfMetricResponse {
	switch entity.Type {
	case "VirtualMachine":
		ref := ctx.Map.Get(entity)
		if ref == nil {
			return new(types.QueryAvailablePerfMetricResponse)
		}
		vm := ref.(*VirtualMachine)
		ds := ""
		if len(vm.Datastore) != 0 {
			ds = vm.Datastore[0].Value
		}
		return p.buildAvailablePerfMetricsQueryResponse(p.vmMetrics, int(vm.Summary.Config.NumCpu), ds)
	case "HostSystem":
		ref := ctx.Map.Get(entity)
		if ref == nil {
			return new(types.QueryAvailablePerfMetricResponse)
		}
		host := ref.(*HostSystem)
		ds := ""
		if len(host.Datastore) != 0 {
			ds = host.Datastore[0].Value
		}
		numCPU := 0
		if host.Hardware != nil && host.Hardware.CpuInfo.NumCpuThreads > 0 {
			numCPU = int(host.Hardware.CpuInfo.NumCpuThreads)
		}
		return p.buildAvailablePerfMetricsQueryResponse(p.hostMetrics, numCPU, ds)
	case "ResourcePool":
		return p.buildAvailablePerfMetricsQueryResponse(p.rpMetrics, 0, "")
	case "ClusterComputeResource":
		if interval != 20 {
			return p.buildAvailablePerfMetricsQueryResponse(p.clusterMetrics, 0, "")
		}
	case "Datastore":
		if interval != 20 {
			return p.buildAvailablePerfMetricsQueryResponse(p.datastoreMetrics, 0, "")
		}
	case "Datacenter":
		if interval != 20 {
			return p.buildAvailablePerfMetricsQueryResponse(p.datacenterMetrics, 0, "")
		}
	}

	// Don't know how to handle this. Return empty response.
	return new(types.QueryAvailablePerfMetricResponse)
}

func (p *PerformanceManager) QueryAvailablePerfMetric(ctx *Context, req *types.QueryAvailablePerfMetric) soap.HasFault {
	body := new(methods.QueryAvailablePerfMetricBody)
	body.Res = p.queryAvailablePerfMetric(ctx, req.Entity, req.IntervalId)

	return body
}

func (p *PerformanceManager) QueryPerf(ctx *Context, req *types.QueryPerf) soap.HasFault {
	body := new(methods.QueryPerfBody)
	body.Res = new(types.QueryPerfResponse)
	body.Res.Returnval = make([]types.BasePerfEntityMetricBase, len(req.QuerySpec))

	for i, qs := range req.QuerySpec {
		// Get metric data for this entity type
		metricData, ok := p.metricData[qs.Entity.Type]
		if !ok {
			body.Fault_ = Fault("", &types.InvalidArgument{
				InvalidProperty: "Entity",
			})
			return body
		}

		// When instance="*" (wildcard), expand each requested metric into
		// the set of available instances for that counter on this entity type.
		// The poller checks instance=="" for aggregate metrics; echoing "*"
		// back causes the poller to skip all data.
		expandedMetrics := qs.MetricId
		hasWildcard := false
		for _, mid := range qs.MetricId {
			if mid.Instance == "*" {
				hasWildcard = true
				break
			}
		}
		if hasWildcard {
			// Get available metrics for this entity type
			avail := p.queryAvailablePerfMetric(ctx, qs.Entity, qs.IntervalId)
			// Build lookup: counterId -> []Instance
			availInstances := make(map[int32][]string)
			if avail != nil {
				for _, am := range avail.Returnval {
					availInstances[am.CounterId] = append(availInstances[am.CounterId], am.Instance)
				}
			}
			expandedMetrics = make([]types.PerfMetricId, 0, len(qs.MetricId)*2)
			for _, mid := range qs.MetricId {
				if mid.Instance == "*" {
					instances, found := availInstances[mid.CounterId]
					if found {
						for _, inst := range instances {
							expandedMetrics = append(expandedMetrics, types.PerfMetricId{
								CounterId: mid.CounterId,
								Instance:  inst,
							})
						}
					} else {
						// Counter not in available list; return aggregate anyway
						expandedMetrics = append(expandedMetrics, types.PerfMetricId{
							CounterId: mid.CounterId,
							Instance:  "",
						})
					}
				} else {
					expandedMetrics = append(expandedMetrics, mid)
				}
			}
		}

		var start, end time.Time
		if qs.EndTime == nil {
			end = time.Now()
		} else {
			end = *qs.EndTime
		}

		// Generate metric series. Divide into n buckets of interval seconds
		interval := qs.IntervalId
		if interval == -1 || interval == 0 {
			// Default based on entity type: clusters/datastores use historical (300s),
			// VMs/hosts/resource pools use realtime (20s)
			switch qs.Entity.Type {
			case "ClusterComputeResource", "Datastore", "Datacenter":
				interval = 300
			default:
				interval = 20
			}
		}

		if qs.StartTime == nil {
			// Default to a reasonable window based on interval:
			// realtime (20s): 1 hour back = 180 samples
			// historical (300s): 1 hour back = 12 samples
			start = end.Add(time.Duration(-3600) * time.Second)
		} else {
			start = *qs.StartTime
		}

		n := 1 + int32(end.Sub(start).Seconds())/interval
		if qs.MaxSample > 0 && n > qs.MaxSample {
			n = qs.MaxSample
		}
		// Safety cap: never generate more than 360 samples regardless of time window
		if n > 360 {
			n = 360
		}

		metrics := new(types.PerfEntityMetric)
		metrics.Entity = qs.Entity

		// Loop through each interval "tick"
		metrics.SampleInfo = make([]types.PerfSampleInfo, n)
		metrics.Value = make([]types.BasePerfMetricSeries, len(expandedMetrics))
		for tick := int32(0); tick < n; tick++ {
			metrics.SampleInfo[tick] = types.PerfSampleInfo{Timestamp: end.Add(time.Duration(-interval*tick) * time.Second), Interval: interval}
		}

		series := make([]*types.PerfMetricIntSeries, len(expandedMetrics))
		for j, mid := range expandedMetrics {
			// Create list of metrics for this tick
			series[j] = &types.PerfMetricIntSeries{Value: make([]int64, n)}
			series[j].Id = mid

			// Check for metric override first
			var overridePoints []int64
			if MetricOverrideFunc != nil {
				overridePoints = MetricOverrideFunc(qs.Entity, mid.CounterId, mid.Instance)
			}

			if len(overridePoints) > 0 {
				// Use override data
				for tick := int32(0); tick < n; tick++ {
					val := overridePoints[int(tick)%len(overridePoints)]
					// Add slight noise to override values too (±2%)
					scale := val / 50
					if scale > 0 {
						val += int64(rand.NormFloat64() * float64(scale))
						if val < 0 {
							val = 0
						}
					}
					series[j].Value[tick] = val
				}
			} else {
				// Use default sample data
				points := metricData[mid.CounterId]
				offset := int64(start.Unix()) / int64(interval)

				// Instance-aware scaling: when Instance is set (e.g. "vmnic0", "vmnic1"),
				// scale base data by a deterministic factor so different instances
				// return different values (50-150% of base).
				var instanceScale float64 = 1.0
				if mid.Instance != "" && mid.Instance != "*" {
					ih := fnv.New32a()
					ih.Write([]byte(mid.Instance))
					instanceScale = 0.5 + float64(ih.Sum32()%100)/100.0 // 0.50 - 1.49
				}

				for tick := int32(0); tick < n; tick++ {
					var pt int64

					// Use sample data if we have it. Otherwise, just send 0.
					if len(points) > 0 {
						pt = points[(offset+int64(tick))%int64(len(points))]
						// Apply instance scaling
						pt = int64(float64(pt) * instanceScale)
						scale := pt / 5
						if scale > 0 {
							// Add some gaussian noise to make the data look more "real"
							pt += int64(rand.NormFloat64() * float64(scale))
							if pt < 0 {
								pt = 0
							}
						}
					} else {
						pt = 0
					}
					series[j].Value[tick] = pt
				}
			}
			metrics.Value[j] = series[j]
		}

		if qs.Format == string(types.PerfFormatCsv) {
			metricsCsv := new(types.PerfEntityMetricCSV)
			metricsCsv.Entity = qs.Entity

			//PerfSampleInfo encoded in the following CSV format: [interval1], [date1], [interval2], [date2], and so on.
			metricsCsv.SampleInfoCSV = sampleInfoCSV(metrics)
			metricsCsv.Value = make([]types.PerfMetricSeriesCSV, len(expandedMetrics))

			for j, mid := range expandedMetrics {
				seriesCsv := &types.PerfMetricSeriesCSV{Value: ""}
				seriesCsv.Id = mid
				seriesCsv.Value = valueCSV(series[j])
				metricsCsv.Value[j] = *seriesCsv
			}

			body.Res.Returnval[i] = metricsCsv
		} else {
			body.Res.Returnval[i] = metrics
		}
	}
	return body
}

// sampleInfoCSV converts the SampleInfo field to a CSV string
func sampleInfoCSV(m *types.PerfEntityMetric) string {
	values := make([]string, len(m.SampleInfo)*2)

	i := 0
	for _, s := range m.SampleInfo {
		values[i] = strconv.Itoa(int(s.Interval))
		i++
		values[i] = s.Timestamp.Format(time.RFC3339)
		i++
	}

	return strings.Join(values, ",")
}

// valueCSV converts the Value field to a CSV string
func valueCSV(s *types.PerfMetricIntSeries) string {
	values := make([]string, len(s.Value))

	for i := range s.Value {
		values[i] = strconv.FormatInt(s.Value[i], 10)
	}

	return strings.Join(values, ",")
}
