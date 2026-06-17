// Inventory rollup: reconcile reported usage bottom-up across the hierarchy.
//
//	VM load  ->  Host QuickStats  ->  Cluster UsageSummary
//
// Real vSphere couples VMs only through the shared physical host: a busy VM
// hurts neighbours ONLY when the host runs out of headroom (CPU contention =>
// ready time; memory pressure => balloon/swap). Reconcile() computes per-host
// headroom now so Phase 3 can apply those victim effects; Phase 1 uses it to
// produce a coherent static snapshot where cluster == sum(hosts) == sum(VMs).
package correlate

import (
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// hostAgg accumulates the load of all VMs placed on a host.
type hostAgg struct {
	host       *mo.HostSystem
	capCPUMhz  int64
	capMemMB   int64
	vmCPUMhz   int64 // summed VM CPU usage
	vmMemMB    int64 // summed VM consumed memory
	vmActiveMB int64
	vmCount    int
}

// Reconcile walks the in-process registry and rewrites reported usage so the
// inventory is internally consistent:
//
//   - each powered-on VM gets a deterministic baseline load (QuickStats set)
//   - each host's QuickStats.Overall{Cpu,Memory}Usage = sum of its VMs + overhead
//   - each cluster's UsageSummary cpu/mem demand = sum of its hosts
//
// It returns the per-host headroom snapshot (capacity minus demand) for use by
// later contention/propagation phases.
func Reconcile(m *simulator.Registry) map[types.ManagedObjectReference]LoadState {
	Reset() // drop any prior snapshot before recomputing

	hosts := map[types.ManagedObjectReference]*hostAgg{}

	// Index hosts and seed their capacity from hardware.
	for _, e := range m.All("HostSystem") {
		h, ok := e.(*simulator.HostSystem)
		if !ok {
			continue
		}
		capCPU := int64(h.Summary.Hardware.CpuMhz) * int64(h.Summary.Hardware.NumCpuCores)
		capMem := h.Summary.Hardware.MemorySize / (1024 * 1024)
		hosts[h.Self] = &hostAgg{host: &h.HostSystem, capCPUMhz: capCPU, capMemMB: capMem}
	}

	// Assign each powered-on VM a baseline load and accumulate onto its host.
	for _, e := range m.All("VirtualMachine") {
		vm, ok := e.(*simulator.VirtualMachine)
		if !ok {
			continue
		}
		if vm.Runtime.PowerState != types.VirtualMachinePowerStatePoweredOn {
			continue
		}
		if vm.Runtime.Host == nil {
			continue
		}
		agg := hosts[*vm.Runtime.Host]
		if agg == nil {
			continue
		}

		coreMhz := agg.host.Summary.Hardware.CpuMhz
		numCPU := vm.Config.Hardware.NumCPU
		memMB := vm.Config.Hardware.MemoryMB

		ls := BaselineVMLoad(vm.Name, numCPU, coreMhz, memMB)
		ApplyToVMQuickStats(&vm.Summary.QuickStats, ls)
		SetState(vm.Self, ls) // record for QueryPerf baseline derivation

		agg.vmCPUMhz += ls.CPUUsageMhz
		agg.vmMemMB += ls.MemUsageMB
		agg.vmActiveMB += ls.MemActiveMB
		agg.vmCount++
	}

	// Roll VM sums into host QuickStats (+ hypervisor overhead), and record
	// per-host headroom for downstream contention modelling.
	headroom := map[types.ManagedObjectReference]LoadState{}
	clusterAgg := map[types.ManagedObjectReference]*LoadState{}

	for ref, agg := range hosts {
		// Hypervisor overhead: ~6% CPU, ~4 GB memory for VMkernel.
		ovhCPU := agg.capCPUMhz * 6 / 100
		ovhMem := int64(4096)

		hostCPU := agg.vmCPUMhz + ovhCPU
		hostMem := agg.vmMemMB + ovhMem
		if hostCPU > agg.capCPUMhz {
			hostCPU = agg.capCPUMhz
		}
		if hostMem > agg.capMemMB {
			hostMem = agg.capMemMB
		}

		agg.host.Summary.QuickStats.OverallCpuUsage = int32(hostCPU)
		agg.host.Summary.QuickStats.OverallMemoryUsage = int32(hostMem)

		hostLS := LoadState{
			CPUCapacityMhz: agg.capCPUMhz,
			MemCapacityMB:  agg.capMemMB,
			CPUUsageMhz:    hostCPU,
			MemUsageMB:     hostMem,
			MemActiveMB:    agg.vmActiveMB,
		}
		headroom[ref] = hostLS
		SetState(ref, hostLS) // record for QueryPerf baseline derivation

		// Accumulate toward the parent cluster (ComputeResource).
		if agg.host.Parent != nil {
			ca := clusterAgg[*agg.host.Parent]
			if ca == nil {
				ca = &LoadState{}
				clusterAgg[*agg.host.Parent] = ca
			}
			ca.CPUCapacityMhz += agg.capCPUMhz
			ca.MemCapacityMB += agg.capMemMB
			ca.CPUUsageMhz += hostCPU
			ca.MemUsageMB += hostMem
		}
	}

	// Roll host sums into each cluster's UsageSummary.
	for _, e := range m.All("ClusterComputeResource") {
		cc, ok := e.(*simulator.ClusterComputeResource)
		if !ok {
			continue
		}
		ca := clusterAgg[cc.Self]
		if ca == nil {
			continue
		}
		cs, ok := cc.Summary.(*types.ClusterComputeResourceSummary)
		if !ok || cs == nil {
			continue
		}
		if cs.UsageSummary == nil {
			cs.UsageSummary = &types.ClusterUsageSummary{}
		}
		cs.UsageSummary.TotalCpuCapacityMhz = int32(ca.CPUCapacityMhz)
		cs.UsageSummary.TotalMemCapacityMB = int32(ca.MemCapacityMB)
		cs.UsageSummary.CpuDemandMhz = int32(ca.CPUUsageMhz)
		cs.UsageSummary.MemDemandMB = int32(ca.MemUsageMB)

		SetState(cc.Self, *ca) // record for QueryPerf baseline derivation
	}

	return headroom
}
