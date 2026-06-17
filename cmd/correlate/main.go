// correlate-check: connects to a running simulator and verifies that reported
// usage is consistent across the inventory hierarchy:
//
//	host.QuickStats.OverallCpuUsage    ~= sum(VM.QuickStats.OverallCpuUsage) on that host
//	host.QuickStats.OverallMemoryUsage ~= sum(VM.QuickStats.HostMemoryUsage)  on that host
//	cluster.UsageSummary.CpuDemandMhz  ~= sum(host.QuickStats.OverallCpuUsage) in that cluster
//
// Exits non-zero if any invariant is violated beyond tolerance.
package main

import (
	"context"
	"fmt"
	"net/url"
	"os"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

func main() {
	addr := os.Getenv("VC_URL")
	if addr == "" {
		addr = "https://administrator@vsphere.local:Site24x7!Demo@100.75.23.73:443/sdk"
	}
	ctx := context.Background()
	u, _ := url.Parse(addr)
	c, err := govmomi.NewClient(ctx, u, true)
	if err != nil {
		panic(err)
	}
	defer c.Logout(ctx)
	m := view.NewManager(c.Client)

	getHosts := func() []mo.HostSystem {
		v, _ := m.CreateContainerView(ctx, c.ServiceContent.RootFolder, []string{"HostSystem"}, true)
		defer v.Destroy(ctx)
		var l []mo.HostSystem
		v.Retrieve(ctx, []string{"HostSystem"}, []string{"name", "parent", "summary"}, &l)
		return l
	}
	getVMs := func() []mo.VirtualMachine {
		v, _ := m.CreateContainerView(ctx, c.ServiceContent.RootFolder, []string{"VirtualMachine"}, true)
		defer v.Destroy(ctx)
		var l []mo.VirtualMachine
		v.Retrieve(ctx, []string{"VirtualMachine"}, []string{"name", "runtime", "summary"}, &l)
		return l
	}
	getClusters := func() []mo.ClusterComputeResource {
		v, _ := m.CreateContainerView(ctx, c.ServiceContent.RootFolder, []string{"ClusterComputeResource"}, true)
		defer v.Destroy(ctx)
		var l []mo.ClusterComputeResource
		v.Retrieve(ctx, []string{"ClusterComputeResource"}, []string{"name", "summary"}, &l)
		return l
	}

	hosts := getHosts()
	vms := getVMs()
	clusters := getClusters()

	// Sum VM usage per host.
	vmCPUByHost := map[types.ManagedObjectReference]int64{}
	vmMemByHost := map[types.ManagedObjectReference]int64{}
	vmCountByHost := map[types.ManagedObjectReference]int{}
	for _, vm := range vms {
		if vm.Runtime.Host == nil {
			continue
		}
		vmCPUByHost[*vm.Runtime.Host] += int64(vm.Summary.QuickStats.OverallCpuUsage)
		vmMemByHost[*vm.Runtime.Host] += int64(vm.Summary.QuickStats.HostMemoryUsage)
		vmCountByHost[*vm.Runtime.Host]++
	}

	fmt.Println("======== HOST  usage  vs  Σ(VM)  ========")
	hostCPUByCluster := map[types.ManagedObjectReference]int64{}
	fails := 0
	for _, h := range hosts {
		hCPU := int64(h.Summary.QuickStats.OverallCpuUsage)
		hMem := int64(h.Summary.QuickStats.OverallMemoryUsage)
		sumCPU := vmCPUByHost[h.Self]
		sumMem := vmMemByHost[h.Self]
		// Host usage must be >= sum of its VMs (hypervisor overhead added on
		// top) and must not exceed physical capacity.
		capCPU := int64(h.Summary.Hardware.CpuMhz) * int64Cores(h)
		status := "OK"
		if hCPU < sumCPU || hMem < sumMem {
			status = "FAIL (host < ΣVM)"
			fails++
		} else if hCPU > capCPU {
			status = "FAIL (host > capacity)"
			fails++
		}
		fmt.Printf("%-14s vms=%-3d cpu host=%-7d ΣVM=%-7d | mem host=%-7d ΣVM=%-7d  %s\n",
			h.Name, vmCountByHost[h.Self], hCPU, sumCPU, hMem, sumMem, status)
		if h.Parent != nil {
			hostCPUByCluster[*h.Parent] += hCPU
		}
	}

	fmt.Println("\n======== CLUSTER demand  vs  Σ(host) ========")
	for _, cl := range clusters {
		cs, ok := cl.Summary.(*types.ClusterComputeResourceSummary)
		if !ok || cs == nil || cs.UsageSummary == nil {
			fmt.Printf("%-18s no UsageSummary\n", cl.Name)
			continue
		}
		demand := int64(cs.UsageSummary.CpuDemandMhz)
		sumHost := hostCPUByCluster[cl.Self]
		status := "OK"
		if abs(demand-sumHost) > sumHost/20+1 { // 5% tolerance
			status = "FAIL"
			fails++
		}
		fmt.Printf("%-18s cpuDemand=%-9d Σhost=%-9d cap=%-9d  %s\n",
			cl.Name, demand, sumHost, cs.UsageSummary.TotalCpuCapacityMhz, status)
	}

	// Phase 2: verify QueryPerf cpu.usagemhz (counter 6, aggregate) matches the
	// host QuickStats it was derived from.
	fmt.Println("\n======== PERF cpu.usagemhz  vs  host QuickStats ========")
	perfMgr := *c.ServiceContent.PerfManager
	for _, h := range hosts {
		req := &types.QueryPerf{
			This: perfMgr,
			QuerySpec: []types.PerfQuerySpec{{
				Entity:     h.Reference(),
				MaxSample:  1,
				IntervalId: 20,
				MetricId:   []types.PerfMetricId{{CounterId: 6, Instance: ""}},
			}},
		}
		resp, err := methods.QueryPerf(ctx, c.Client, req)
		if err != nil || resp == nil || len(resp.Returnval) == 0 {
			continue
		}
		var perf int64 = -1
		if em, ok := resp.Returnval[0].(*types.PerfEntityMetric); ok {
			for _, s := range em.Value {
				if ser, ok := s.(*types.PerfMetricIntSeries); ok && len(ser.Value) > 0 {
					perf = ser.Value[0]
				}
			}
		}
		qs := int64(h.Summary.QuickStats.OverallCpuUsage)
		status := "OK"
		// perf has ±noise; allow 15% + small floor.
		if qs > 0 && abs(perf-qs) > qs*15/100+50 {
			status = "FAIL"
			fails++
		}
		fmt.Printf("%-14s QuickStats=%-7d perf.usagemhz=%-7d  %s\n", h.Name, qs, perf, status)
	}

	fmt.Printf("\n%d invariant violation(s)\n", fails)
	if fails > 0 {
		os.Exit(1)
	}
	fmt.Println("ALL CORRELATION INVARIANTS HOLD ✓")
}

func int64Cores(h mo.HostSystem) int64 { return int64(h.Summary.Hardware.NumCpuCores) }

func abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}
