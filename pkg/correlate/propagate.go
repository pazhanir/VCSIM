// Chaos propagation (Phase 3).
//
// When a scenario drives an entity's load up, the change must ripple through the
// inventory the way real vSphere behaves:
//
//   - a VM's load rolls UP into its host, and the host's into its cluster;
//   - a busy VM only hurts NEIGHBOURS once the shared host runs out of headroom
//     (CPU contention => ready time; memory pressure => balloon/swap), because
//     VMs are coupled only through the physical host they share.
//
// This file provides an in-process engine the scenario manager calls. It mutates
// the LoadState registry (so QuickStats + QueryPerf baselines move together) and
// reports contention victims so the caller can apply the symptom metrics
// (cpu.ready, mem.vmmemctl, mem.swapped) those neighbours would show.
package correlate

import (
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// Victim describes a neighbour VM affected by host contention and by how much.
type Victim struct {
	VM          types.ManagedObjectReference
	Name        string
	CPUReadyPct float64 // 0-100: share of time waiting for a physical core
	BalloonMB   int64   // memory reclaimed from this VM via balloon
	SwappedMB   int64   // memory swapped from this VM
	HostCPUOver float64 // host CPU over-subscription ratio (>1 means contended)
	HostMemOver float64 // host memory over-subscription ratio
}

// Engine wires the in-process registry to the propagation logic.
type Engine struct {
	m *simulator.Registry
}

// NewEngine returns a propagation engine over the simulator registry.
func NewEngine(m *simulator.Registry) *Engine { return &Engine{m: m} }

// hostOf returns the host MO a VM runs on, or nil.
func (e *Engine) hostOf(vm *mo.VirtualMachine) *simulator.HostSystem {
	if vm.Runtime.Host == nil {
		return nil
	}
	h, _ := e.m.Get(*vm.Runtime.Host).(*simulator.HostSystem)
	return h
}

// vmsOnHost returns the powered-on VMs whose runtime host is the given ref.
func (e *Engine) vmsOnHost(host types.ManagedObjectReference) []*simulator.VirtualMachine {
	var out []*simulator.VirtualMachine
	for _, ent := range e.m.All("VirtualMachine") {
		vm, ok := ent.(*simulator.VirtualMachine)
		if !ok || vm.Runtime.Host == nil || *vm.Runtime.Host != host {
			continue
		}
		if vm.Runtime.PowerState == types.VirtualMachinePowerStatePoweredOn {
			out = append(out, vm)
		}
	}
	return out
}

// SetVMLoad sets a VM's CPU/mem utilization (as a fraction 0-1 of its own
// allotment), updates its QuickStats and recorded state, then rolls the change
// up to the host and cluster and computes any neighbour contention.
//
// Passing cpuFrac or memFrac < 0 leaves that dimension at its current value.
func (e *Engine) SetVMLoad(vmRef types.ManagedObjectReference, cpuFrac, memFrac float64) []Victim {
	vm, ok := e.m.Get(vmRef).(*simulator.VirtualMachine)
	if !ok {
		return nil
	}
	cur, _ := GetState(vmRef)
	capMhz := int64(vm.Config.Hardware.NumCPU) * int64(hostCoreMhz(e, vm))
	capMem := int64(vm.Config.Hardware.MemoryMB)

	ls := cur
	ls.CPUCapacityMhz = capMhz
	ls.MemCapacityMB = capMem
	if cpuFrac >= 0 {
		ls.CPUUsageMhz = int64(cpuFrac * float64(capMhz))
	}
	if memFrac >= 0 {
		ls.MemUsageMB = int64(memFrac * float64(capMem))
		ls.MemActiveMB = ls.MemUsageMB * 60 / 100
		ls.MemSharedMB = ls.MemUsageMB * 15 / 100
	}
	ApplyToVMQuickStats(&vm.Summary.QuickStats, ls)
	SetState(vmRef, ls)

	host := e.hostOf(&vm.VirtualMachine)
	if host == nil {
		return nil
	}
	return e.recomputeHost(host)
}

// SetHostLoad forces a host's CPU/mem utilization fraction directly (used by
// host-level scenarios), updates QuickStats + state, rolls up to the cluster,
// and returns contention victims among the host's VMs.
func (e *Engine) SetHostLoad(hostRef types.ManagedObjectReference, cpuFrac, memFrac float64) []Victim {
	host, ok := e.m.Get(hostRef).(*simulator.HostSystem)
	if !ok {
		return nil
	}
	capCPU := int64(host.Summary.Hardware.CpuMhz) * int64(host.Summary.Hardware.NumCpuCores)
	capMem := host.Summary.Hardware.MemorySize / (1024 * 1024)

	// Start from the host's current state so a dimension passed as <0 ("leave
	// unchanged") is preserved instead of zeroed/negated.
	cur, _ := GetState(hostRef)
	hostCPU := cur.CPUUsageMhz
	hostMem := cur.MemUsageMB
	if cpuFrac >= 0 {
		hostCPU = int64(cpuFrac * float64(capCPU))
	}
	if memFrac >= 0 {
		hostMem = int64(memFrac * float64(capMem))
	}
	host.Summary.QuickStats.OverallCpuUsage = int32(hostCPU)
	host.Summary.QuickStats.OverallMemoryUsage = int32(hostMem)

	hostLS := LoadState{CPUCapacityMhz: capCPU, MemCapacityMB: capMem, CPUUsageMhz: hostCPU, MemUsageMB: hostMem}
	SetState(hostRef, hostLS)
	e.rollupCluster(host.Parent)

	return e.contention(hostRef, capCPU, capMem, hostCPU, hostMem)
}

// recomputeHost re-sums a host's VMs into its QuickStats/state, rolls up to the
// cluster, and returns contention victims.
func (e *Engine) recomputeHost(host *simulator.HostSystem) []Victim {
	capCPU := int64(host.Summary.Hardware.CpuMhz) * int64(host.Summary.Hardware.NumCpuCores)
	capMem := host.Summary.Hardware.MemorySize / (1024 * 1024)

	var sumCPU, sumMem int64
	for _, vm := range e.vmsOnHost(host.Self) {
		ls, _ := GetState(vm.Self)
		sumCPU += ls.CPUUsageMhz
		sumMem += ls.MemUsageMB
	}
	// add hypervisor overhead, cap at capacity for the *reported* figure
	ovhCPU := capCPU * 6 / 100
	hostCPU := sumCPU + ovhCPU
	hostMem := sumMem + 4096
	repCPU, repMem := hostCPU, hostMem
	if repCPU > capCPU {
		repCPU = capCPU
	}
	if repMem > capMem {
		repMem = capMem
	}
	host.Summary.QuickStats.OverallCpuUsage = int32(repCPU)
	host.Summary.QuickStats.OverallMemoryUsage = int32(repMem)
	SetState(host.Self, LoadState{CPUCapacityMhz: capCPU, MemCapacityMB: capMem, CPUUsageMhz: repCPU, MemUsageMB: repMem})
	e.rollupCluster(host.Parent)

	// Contention is judged on DEMAND (uncapped), so an over-subscribed host is
	// detected even though the reported figure is clamped to capacity.
	return e.contention(host.Self, capCPU, capMem, hostCPU, hostMem)
}

// contention computes victim effects for VMs on a host when demand exceeds
// capacity. Returns nil (no neighbour impact) when the host has headroom.
func (e *Engine) contention(host types.ManagedObjectReference, capCPU, capMem, demandCPU, demandMem int64) []Victim {
	cpuOver := ratio(demandCPU, capCPU)
	memOver := ratio(demandMem, capMem)

	// CPU scheduling contention begins before full saturation: ready time climbs
	// noticeably once a host passes ~85% CPU. Memory reclaim (balloon/swap) only
	// starts when the host is genuinely out of RAM (>100%).
	const cpuContentionOnset = 0.85
	if cpuOver < cpuContentionOnset && memOver <= 1.0 {
		return nil // host has headroom => neighbours unaffected
	}

	vms := e.vmsOnHost(host)
	var victims []Victim
	for _, vm := range vms {
		v := Victim{VM: vm.Self, Name: vm.Name, HostCPUOver: cpuOver, HostMemOver: memOver}
		// CPU contention => ready time grows as the host approaches/exceeds
		// capacity. Scaled so ~85% -> small, 100% -> moderate, oversubscribed
		// (>100%) -> severe.
		if cpuOver >= cpuContentionOnset {
			ready := (cpuOver - cpuContentionOnset) / cpuContentionOnset * 100
			if ready > 80 {
				ready = 80
			}
			if ready < 1 {
				ready = 1
			}
			v.CPUReadyPct = ready
		}
		// Memory pressure => reclaim from each VM (balloon first, then swap).
		if memOver > 1.0 {
			ls, _ := GetState(vm.Self)
			reclaim := int64(float64(ls.MemUsageMB) * (memOver - 1.0) / memOver)
			balloon := reclaim
			if balloon > ls.MemUsageMB/2 {
				balloon = ls.MemUsageMB / 2 // balloon caps ~50%, rest swaps
			}
			v.BalloonMB = balloon
			v.SwappedMB = reclaim - balloon
			ls.MemBalloonedMB = v.BalloonMB
			ls.MemSwappedMB = v.SwappedMB
			SetState(vm.Self, ls)
			ApplyToVMQuickStats(&vm.Summary.QuickStats, ls)
		}
		victims = append(victims, v)
	}
	return victims
}

// rollupCluster re-sums all hosts under a compute resource into its UsageSummary.
func (e *Engine) rollupCluster(parent *types.ManagedObjectReference) {
	if parent == nil {
		return
	}
	cc, ok := e.m.Get(*parent).(*simulator.ClusterComputeResource)
	if !ok {
		return
	}
	var capCPU, capMem, useCPU, useMem int64
	for _, hr := range cc.Host {
		h, ok := e.m.Get(hr).(*simulator.HostSystem)
		if !ok {
			continue
		}
		capCPU += int64(h.Summary.Hardware.CpuMhz) * int64(h.Summary.Hardware.NumCpuCores)
		capMem += h.Summary.Hardware.MemorySize / (1024 * 1024)
		useCPU += int64(h.Summary.QuickStats.OverallCpuUsage)
		useMem += int64(h.Summary.QuickStats.OverallMemoryUsage)
	}
	cs, ok := cc.Summary.(*types.ClusterComputeResourceSummary)
	if !ok || cs == nil {
		return
	}
	if cs.UsageSummary == nil {
		cs.UsageSummary = &types.ClusterUsageSummary{}
	}
	cs.UsageSummary.TotalCpuCapacityMhz = int32(capCPU)
	cs.UsageSummary.TotalMemCapacityMB = int32(capMem)
	cs.UsageSummary.CpuDemandMhz = int32(useCPU)
	cs.UsageSummary.MemDemandMB = int32(useMem)
	SetState(cc.Self, LoadState{CPUCapacityMhz: capCPU, MemCapacityMB: capMem, CPUUsageMhz: useCPU, MemUsageMB: useMem})
}

func hostCoreMhz(e *Engine, vm *simulator.VirtualMachine) int32 {
	h := e.hostOf(&vm.VirtualMachine)
	if h == nil {
		return 2000
	}
	return h.Summary.Hardware.CpuMhz
}

func ratio(a, b int64) float64 {
	if b == 0 {
		return 0
	}
	return float64(a) / float64(b)
}
