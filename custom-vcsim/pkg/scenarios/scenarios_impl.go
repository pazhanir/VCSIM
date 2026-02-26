package scenarios

import (
	"context"
	"fmt"
	"time"

	"github.com/vmware/govmomi/vim25/types"

	"github.com/site24x7/vcsim-demo/pkg/overrides"
)

// ==================== CPU Scenarios ====================

// Scenario 1: Host CPU Saturation
// - Host CPU at 96-99%
// - CASCADE: All VMs on host get elevated CPU ready time
func (m *Manager) activateCPUHostSaturation(ctx context.Context, req ActivateRequest, expiry time.Time) error {
	cpuPercent := getParamInt(req.Params, "cpu_percent", 97)

	for _, target := range req.Targets {
		ref, err := m.resolveRef(ctx, target)
		if err != nil {
			return err
		}

		// Set high CPU on host
		m.setHighCPUMetrics(ref, cpuPercent, expiry)

		// CASCADE: Find all VMs on this host and set high CPU ready
		vms, err := m.findVMsOnHost(ctx, target)
		if err != nil {
			// Non-fatal — host may have no VMs
			continue
		}
		for _, vm := range vms {
			vmRef := vm.Reference()
			m.setHighCPUReady(vmRef, 8000+int(vmRef.Value[len(vmRef.Value)-1])*500, expiry)
			// Also slightly elevate VM CPU usage
			m.setHighCPUMetrics(vmRef, 70+int(vmRef.Value[len(vmRef.Value)-1])%20, expiry)
		}
	}
	return nil
}

// Scenario 2: Cluster-Wide CPU Contention
// - All hosts in cluster at 82-95% CPU
// - CASCADE: All VMs show elevated ready time, resource pools near limits
func (m *Manager) activateCPUClusterContention(ctx context.Context, req ActivateRequest, expiry time.Time) error {
	for _, target := range req.Targets {
		hosts, err := m.findHostsInCluster(ctx, target)
		if err != nil {
			return fmt.Errorf("find hosts in cluster %s: %w", target, err)
		}

		for i, host := range hosts {
			hostRef := host.Reference()
			cpuPercent := 82 + (i*3)%18 // 82-99% varied
			m.setHighCPUMetrics(hostRef, cpuPercent, expiry)

			// CASCADE: VMs on each host
			vms, _ := m.findVMsOnHost(ctx, host.InventoryPath)
			for _, vm := range vms {
				vmRef := vm.Reference()
				m.setHighCPUReady(vmRef, 5000+i*1000, expiry)
			}
		}
	}
	return nil
}

// Scenario 3: VM CPU Co-Stop
func (m *Manager) activateCPUVMCoStop(ctx context.Context, req ActivateRequest, expiry time.Time) error {
	for _, target := range req.Targets {
		ref, err := m.resolveRef(ctx, target)
		if err != nil {
			return err
		}
		// Co-stop metric
		m.registry.SetMetric(ref, overrides.MetricOverride{
			CounterID: 13, // cpu.costop.summation
			Values:    generateHighValues(12000, 25),
			ExpiresAt: expiry,
		})
		// Ready time also elevated
		m.setHighCPUReady(ref, 10000, expiry)
		// CPU usage erratic (swinging wildly)
		m.registry.SetMetric(ref, overrides.MetricOverride{
			CounterID: 2, // cpu.usage.average
			Values:    []int64{9500, 2000, 9800, 1500, 9200, 3000, 9900, 1000, 8500, 4000, 9700, 2500},
			ExpiresAt: expiry,
		})
	}
	return nil
}

// Scenario 4: Resource Pool CPU Limit Hit
func (m *Manager) activateCPURPLimit(ctx context.Context, req ActivateRequest, expiry time.Time) error {
	for _, target := range req.Targets {
		ref, err := m.resolveRef(ctx, target)
		if err != nil {
			return err
		}
		// RP CPU at limit
		m.registry.SetMetric(ref, overrides.MetricOverride{
			CounterID: 6,                           // cpu.usagemhz.average
			Values:    generateHighValues(8000, 3), // MHz — near a typical limit
			ExpiresAt: expiry,
		})
	}
	return nil
}

// ==================== Memory Scenarios ====================

// Scenario 5: Host Memory Exhaustion with Balloon + Swap Chain
// CASCADE: VMkernel reclaim chain affects all VMs on host
func (m *Manager) activateMemHostExhaustion(ctx context.Context, req ActivateRequest, expiry time.Time) error {
	memPercent := getParamInt(req.Params, "mem_percent", 98)

	for _, target := range req.Targets {
		ref, err := m.resolveRef(ctx, target)
		if err != nil {
			return err
		}

		// Host memory exhaustion with full reclaim chain
		m.setHighMemMetrics(ref, memPercent, 4096, 2000, expiry)

		// Compression rate
		m.registry.SetMetric(ref, overrides.MetricOverride{
			CounterID: 80, // mem.compressionRate.average
			Values:    generateHighValues(500, 20),
			ExpiresAt: expiry,
		})

		// CASCADE: All VMs on host get balloon and swap
		vms, err := m.findVMsOnHost(ctx, target)
		if err != nil {
			continue
		}
		for i, vm := range vms {
			vmRef := vm.Reference()
			balloonMB := 512 + (i%4)*256 // Varied balloon per VM
			m.registry.SetMetric(vmRef, overrides.MetricOverride{
				CounterID: 65, // mem.vmmemctl.average (balloon)
				Values:    generateHighValues(int64(balloonMB*1024), 10),
				ExpiresAt: expiry,
			})
			m.registry.SetMetric(vmRef, overrides.MetricOverride{
				CounterID: 35, // mem.swapped.average
				Values:    generateHighValues(int64(256*1024), 15),
				ExpiresAt: expiry,
			})
		}
	}
	return nil
}

// Scenario 6: VM Memory Leak
func (m *Manager) activateMemVMLeak(ctx context.Context, req ActivateRequest, expiry time.Time) error {
	for _, target := range req.Targets {
		ref, err := m.resolveRef(ctx, target)
		if err != nil {
			return err
		}
		// Consumed memory growing over time
		m.registry.SetMetric(ref, overrides.MetricOverride{
			CounterID: 25,                                       // mem.consumed.average
			Values:    generateIncrementingValues(500*1024, 12), // Growing by 500MB per interval
			ExpiresAt: expiry,
		})
		// Active memory tracking consumed
		m.registry.SetMetric(ref, overrides.MetricOverride{
			CounterID: 26, // mem.active.average
			Values:    generateIncrementingValues(480*1024, 12),
			ExpiresAt: expiry,
		})
		// Usage climbing
		m.registry.SetMetric(ref, overrides.MetricOverride{
			CounterID: 24, // mem.usage.average
			Values:    []int64{5000, 5500, 6000, 6500, 7000, 7500, 8000, 8500, 9000, 9200, 9500, 9800},
			ExpiresAt: expiry,
		})
	}
	return nil
}

// Scenario 7: TPS Failure
func (m *Manager) activateMemTPSFailure(ctx context.Context, req ActivateRequest, expiry time.Time) error {
	for _, target := range req.Targets {
		ref, err := m.resolveRef(ctx, target)
		if err != nil {
			return err
		}
		// Low shared memory (TPS not working)
		m.registry.SetMetric(ref, overrides.MetricOverride{
			CounterID: 36,                          // mem.shared.average
			Values:    generateHighValues(1024, 5), // Very low — 1MB shared
			ExpiresAt: expiry,
		})
		// Higher than expected usage
		m.setHighMemMetrics(ref, 88, 0, 0, expiry)
	}
	return nil
}

// Scenario 8: NUMA Imbalance
func (m *Manager) activateMemNUMAImbalance(ctx context.Context, req ActivateRequest, expiry time.Time) error {
	for _, target := range req.Targets {
		ref, err := m.resolveRef(ctx, target)
		if err != nil {
			return err
		}
		// NUMA remote access — elevated
		m.registry.SetMetric(ref, overrides.MetricOverride{
			CounterID: 37, // mem.llSwapInRate.average (local-remote swaps)
			Values:    generateHighValues(800, 20),
			ExpiresAt: expiry,
		})
		// Moderate CPU ready from NUMA scheduling
		m.setHighCPUReady(ref, 3000, expiry)
	}
	return nil
}

// ==================== Disk / Storage Scenarios ====================

// Scenario 9: Datastore Full + VM Suspension
// CASCADE: VMs on datastore get suspended
func (m *Manager) activateDiskDSFull(ctx context.Context, req ActivateRequest, expiry time.Time) error {
	freePercent := getParamInt(req.Params, "free_percent", 2)

	for _, target := range req.Targets {
		ref, err := m.resolveRef(ctx, target)
		if err != nil {
			return err
		}

		// Set datastore free space very low
		totalBytes := int64(4096) * 1024 * 1024 * 1024 // 4TB
		freeBytes := totalBytes * int64(freePercent) / 100
		m.registry.SetProperty(ref, overrides.PropertyOverride{
			Property:  "summary.freeSpace",
			Value:     freeBytes,
			ExpiresAt: expiry,
		})
		m.registry.SetProperty(ref, overrides.PropertyOverride{
			Property:  "summary.capacity",
			Value:     totalBytes,
			ExpiresAt: expiry,
		})

		// CASCADE: Suspend some VMs on this datastore
		// (In real implementation, we'd find VMs by datastore association)
		// For now, we set the property to indicate the datastore is nearly full
	}
	return nil
}

// Scenario 10: Storage Latency Spike
// CASCADE: All hosts and VMs using the datastore get elevated latency
func (m *Manager) activateDiskLatencySpike(ctx context.Context, req ActivateRequest, expiry time.Time) error {
	readLatency := getParamInt(req.Params, "read_latency_ms", 40)
	writeLatency := getParamInt(req.Params, "write_latency_ms", 60)

	for _, target := range req.Targets {
		ref, err := m.resolveRef(ctx, target)
		if err != nil {
			return err
		}
		m.setHighDiskLatency(ref, readLatency, writeLatency, expiry)
	}
	return nil
}

// Scenario 11: Datastore Inaccessible
// CASCADE: VMs become orphaned
func (m *Manager) activateDiskDSInaccessible(ctx context.Context, req ActivateRequest, expiry time.Time) error {
	for _, target := range req.Targets {
		ref, err := m.resolveRef(ctx, target)
		if err != nil {
			return err
		}
		m.registry.SetProperty(ref, overrides.PropertyOverride{
			Property:  "summary.accessible",
			Value:     false,
			ExpiresAt: expiry,
		})
	}
	return nil
}

// Scenario 12: Disk I/O Saturation
func (m *Manager) activateDiskIOSaturation(ctx context.Context, req ActivateRequest, expiry time.Time) error {
	for _, target := range req.Targets {
		ref, err := m.resolveRef(ctx, target)
		if err != nil {
			return err
		}
		m.registry.SetMetric(ref, overrides.MetricOverride{
			CounterID: 172, // disk.numberReadAveraged.average
			Values:    generateHighValues(5000, 15),
			ExpiresAt: expiry,
		})
		m.registry.SetMetric(ref, overrides.MetricOverride{
			CounterID: 173, // disk.numberWriteAveraged.average
			Values:    generateHighValues(3000, 15),
			ExpiresAt: expiry,
		})
		// Latency rises with IO saturation
		m.setHighDiskLatency(ref, 15, 25, expiry)
	}
	return nil
}

// Scenario 13: All Paths Down
func (m *Manager) activateDiskAPD(ctx context.Context, req ActivateRequest, expiry time.Time) error {
	for _, target := range req.Targets {
		ref, err := m.resolveRef(ctx, target)
		if err != nil {
			return err
		}
		m.registry.SetProperty(ref, overrides.PropertyOverride{
			Property:  "summary.accessible",
			Value:     false,
			ExpiresAt: expiry,
		})
		// Zero IOPS
		m.registry.SetMetric(ref, overrides.MetricOverride{
			CounterID: 172, Values: []int64{0}, ExpiresAt: expiry,
		})
		m.registry.SetMetric(ref, overrides.MetricOverride{
			CounterID: 173, Values: []int64{0}, ExpiresAt: expiry,
		})
	}
	return nil
}

// ==================== Network Scenarios ====================

// Scenario 14: Management Network Partition
// CASCADE: Host disconnected, VMs orphaned
func (m *Manager) activateNetMgmtPartition(ctx context.Context, req ActivateRequest, expiry time.Time) error {
	for _, target := range req.Targets {
		host, err := m.finder.HostSystem(ctx, target)
		if err != nil {
			return err
		}
		// Disconnect the host
		task, err := host.Disconnect(ctx)
		if err != nil {
			return err
		}
		if _, err := task.WaitForResult(ctx, nil); err != nil {
			return err
		}
	}
	return nil
}

// Scenario 15: VM Network Isolation
func (m *Manager) activateNetVMIsolation(ctx context.Context, req ActivateRequest, expiry time.Time) error {
	for _, target := range req.Targets {
		ref, err := m.resolveRef(ctx, target)
		if err != nil {
			return err
		}
		// Network usage drops to 0
		m.registry.SetMetric(ref, overrides.MetricOverride{
			CounterID: 195, // net.usage.average
			Values:    []int64{0},
			ExpiresAt: expiry,
		})
		// Dropped packets spike
		m.setHighNetDrops(ref, 500, expiry)
	}
	return nil
}

// Scenario 16: Network Saturation
// CASCADE: All VMs on host experience packet drops
func (m *Manager) activateNetSaturation(ctx context.Context, req ActivateRequest, expiry time.Time) error {
	for _, target := range req.Targets {
		ref, err := m.resolveRef(ctx, target)
		if err != nil {
			return err
		}
		// Host NIC near capacity (9500 KBps of 10GbE)
		m.registry.SetMetric(ref, overrides.MetricOverride{
			CounterID: 195, // net.usage.average
			Values:    generateHighValues(9500, 3),
			ExpiresAt: expiry,
		})
		m.setHighNetDrops(ref, 200, expiry)

		// CASCADE: All VMs on this host
		vms, err := m.findVMsOnHost(ctx, target)
		if err != nil {
			continue
		}
		for _, vm := range vms {
			vmRef := vm.Reference()
			m.setHighNetDrops(vmRef, 50, expiry)
		}
	}
	return nil
}

// Scenario 17: DVSwitch Uplink Failover
func (m *Manager) activateNetUplinkFailover(ctx context.Context, req ActivateRequest, expiry time.Time) error {
	for _, target := range req.Targets {
		ref, err := m.resolveRef(ctx, target)
		if err != nil {
			return err
		}
		// Brief spike in dropped packets
		m.setHighNetDrops(ref, 1000, expiry)
		// Bandwidth temporarily drops
		m.registry.SetMetric(ref, overrides.MetricOverride{
			CounterID: 195, // net.usage.average
			Values:    []int64{0, 0, 500, 2000, 4000, 4500, 4800, 4900, 5000, 5000, 5000, 5000},
			ExpiresAt: expiry,
		})
	}
	return nil
}

// ==================== Host Scenarios ====================

// Scenario 18: Host PSOD
// CASCADE: All VMs crash, HA restarts on other hosts
func (m *Manager) activateHostPSOD(ctx context.Context, req ActivateRequest, expiry time.Time) error {
	for _, target := range req.Targets {
		host, err := m.finder.HostSystem(ctx, target)
		if err != nil {
			return err
		}

		// Power off all VMs on this host first (simulating crash)
		vms, _ := m.findVMsOnHost(ctx, target)
		for _, vm := range vms {
			task, err := vm.PowerOff(ctx)
			if err == nil {
				_, _ = task.WaitForResult(ctx, nil)
			}
		}

		// Disconnect the host (simulating PSOD)
		task, err := host.Disconnect(ctx)
		if err != nil {
			return err
		}
		if _, err := task.WaitForResult(ctx, nil); err != nil {
			return err
		}

		// Set host runtime.connectionState to notResponding
		hostRef := host.Reference()
		m.registry.SetProperty(hostRef, overrides.PropertyOverride{
			Property:  "runtime.connectionState",
			Value:     types.HostSystemConnectionStateNotResponding,
			ExpiresAt: expiry,
		})

		// Zero out all host metrics
		m.registry.SetMetric(hostRef, overrides.MetricOverride{
			CounterID: 2, Values: []int64{0}, ExpiresAt: expiry, // cpu.usage
		})
		m.registry.SetMetric(hostRef, overrides.MetricOverride{
			CounterID: 24, Values: []int64{0}, ExpiresAt: expiry, // mem.usage
		})
	}
	return nil
}

// Scenario 19: Host Maintenance Mode
// CASCADE: VMs evacuated, destination hosts load increases
func (m *Manager) activateHostMaintenance(ctx context.Context, req ActivateRequest, expiry time.Time) error {
	for _, target := range req.Targets {
		host, err := m.finder.HostSystem(ctx, target)
		if err != nil {
			return err
		}
		task, err := host.EnterMaintenanceMode(ctx, 0, true, nil)
		if err != nil {
			return err
		}
		if _, err := task.WaitForResult(ctx, nil); err != nil {
			return err
		}
	}
	return nil
}

// Scenario 20: Host Boot Storm
// CASCADE: Multiple hosts go down, remaining hosts overloaded
func (m *Manager) activateHostBootStorm(ctx context.Context, req ActivateRequest, expiry time.Time) error {
	hostCount := getParamInt(req.Params, "host_count", 3)

	for _, target := range req.Targets {
		hosts, err := m.findHostsInCluster(ctx, target)
		if err != nil {
			return err
		}

		// Disconnect first N hosts
		disconnected := 0
		for _, host := range hosts {
			if disconnected >= hostCount {
				break
			}
			task, err := host.Disconnect(ctx)
			if err != nil {
				continue
			}
			_, _ = task.WaitForResult(ctx, nil)
			disconnected++
		}

		// CASCADE: Remaining hosts get overloaded
		for i := hostCount; i < len(hosts); i++ {
			hostRef := hosts[i].Reference()
			overloadPercent := 85 + (i%3)*5 // 85-95%
			m.setHighCPUMetrics(hostRef, overloadPercent, expiry)
			m.setHighMemMetrics(hostRef, overloadPercent-5, 2048, 1000, expiry)
		}
	}
	return nil
}

// Scenario 21: vMotion Network Failure
func (m *Manager) activateHostVMotionFail(ctx context.Context, req ActivateRequest, expiry time.Time) error {
	for _, target := range req.Targets {
		ref, err := m.resolveRef(ctx, target)
		if err != nil {
			return err
		}
		// Set property indicating vMotion is not available
		m.registry.SetProperty(ref, overrides.PropertyOverride{
			Property:  "summary.config.vmotionEnabled",
			Value:     false,
			ExpiresAt: expiry,
		})
	}
	return nil
}

// Scenario 22: Host Time Drift
func (m *Manager) activateHostTimeDrift(ctx context.Context, req ActivateRequest, expiry time.Time) error {
	for _, target := range req.Targets {
		ref, err := m.resolveRef(ctx, target)
		if err != nil {
			return err
		}
		m.registry.SetProperty(ref, overrides.PropertyOverride{
			Property:  "config.dateTimeInfo.ntpConfig.running",
			Value:     false,
			ExpiresAt: expiry,
		})
	}
	return nil
}

// ==================== VM Scenarios ====================

// Scenario 23: VM Unexpected Crash
func (m *Manager) activateVMCrash(ctx context.Context, req ActivateRequest, expiry time.Time) error {
	for _, target := range req.Targets {
		vm, err := m.finder.VirtualMachine(ctx, target)
		if err != nil {
			return err
		}
		vmRef := vm.Reference()

		// First set heartbeat to gray (tools not responding)
		m.registry.SetProperty(vmRef, overrides.PropertyOverride{
			Property:  "guestHeartbeatStatus",
			Value:     types.ManagedEntityStatusGray,
			ExpiresAt: expiry,
		})

		// Then power off (crash)
		task, err := vm.PowerOff(ctx)
		if err != nil {
			return err
		}
		_, _ = task.WaitForResult(ctx, nil)
	}
	return nil
}

// Scenario 24: VM Snapshot Chain Growing
func (m *Manager) activateVMSnapshotGrowth(ctx context.Context, req ActivateRequest, expiry time.Time) error {
	snapshotCount := getParamInt(req.Params, "snapshot_count", 5)

	for _, target := range req.Targets {
		vm, err := m.finder.VirtualMachine(ctx, target)
		if err != nil {
			return err
		}

		// Create multiple snapshots
		for i := 1; i <= snapshotCount; i++ {
			name := fmt.Sprintf("snapshot-%d", i)
			desc := fmt.Sprintf("Automated snapshot %d - FORGOT TO DELETE", i)
			task, err := vm.CreateSnapshot(ctx, name, desc, false, false)
			if err != nil {
				continue
			}
			_, _ = task.WaitForResult(ctx, nil)
		}
	}
	return nil
}

// Scenario 25: VMware Tools Not Running
func (m *Manager) activateVMToolsDown(ctx context.Context, req ActivateRequest, expiry time.Time) error {
	for _, target := range req.Targets {
		ref, err := m.resolveRef(ctx, target)
		if err != nil {
			return err
		}
		m.registry.SetProperty(ref, overrides.PropertyOverride{
			Property:  "guest.toolsRunningStatus",
			Value:     "guestToolsNotRunning",
			ExpiresAt: expiry,
		})
		m.registry.SetProperty(ref, overrides.PropertyOverride{
			Property:  "guest.toolsStatus",
			Value:     types.VirtualMachineToolsStatusToolsNotRunning,
			ExpiresAt: expiry,
		})
		m.registry.SetProperty(ref, overrides.PropertyOverride{
			Property:  "guestHeartbeatStatus",
			Value:     types.ManagedEntityStatusGray,
			ExpiresAt: expiry,
		})
	}
	return nil
}

// Scenario 26: VM Invalid/Orphaned State
func (m *Manager) activateVMInvalidState(ctx context.Context, req ActivateRequest, expiry time.Time) error {
	for _, target := range req.Targets {
		ref, err := m.resolveRef(ctx, target)
		if err != nil {
			return err
		}
		m.registry.SetProperty(ref, overrides.PropertyOverride{
			Property:  "runtime.connectionState",
			Value:     types.VirtualMachineConnectionStateOrphaned,
			ExpiresAt: expiry,
		})
	}
	return nil
}

// ==================== Cascading Scenarios ====================

// Scenario 27: SAN Array Failure (Perfect Storm)
// 5 datastores offline → ~60 VMs orphaned → host storage errors → alarms cascade
func (m *Manager) activateCascadeSANFailure(ctx context.Context, req ActivateRequest, expiry time.Time) error {
	// Targets should be datastore paths
	for _, target := range req.Targets {
		ref, err := m.resolveRef(ctx, target)
		if err != nil {
			return err
		}

		// 1. Datastore inaccessible
		m.registry.SetProperty(ref, overrides.PropertyOverride{
			Property: "summary.accessible", Value: false, ExpiresAt: expiry,
		})

		// 2. Zero IOPS and max latency on datastore
		m.registry.SetMetric(ref, overrides.MetricOverride{
			CounterID: 172, Values: []int64{0}, ExpiresAt: expiry,
		})
		m.registry.SetMetric(ref, overrides.MetricOverride{
			CounterID: 170, Values: generateHighValues(500, 30), ExpiresAt: expiry,
		})
	}
	return nil
}

// Scenario 28: Cascading HA Failure
// Host 1 fails → HA overloads survivors → Host 2 fails → capacity exhaustion
func (m *Manager) activateCascadeHAExhaustion(ctx context.Context, req ActivateRequest, expiry time.Time) error {
	for _, target := range req.Targets {
		hosts, err := m.findHostsInCluster(ctx, target)
		if err != nil {
			return err
		}
		if len(hosts) < 3 {
			return fmt.Errorf("need at least 3 hosts in cluster for HA cascade")
		}

		// Step 1: First host fails
		host1 := hosts[0]
		vms1, _ := m.findVMsOnHost(ctx, host1.InventoryPath)
		for _, vm := range vms1 {
			task, _ := vm.PowerOff(ctx)
			if task != nil {
				_, _ = task.WaitForResult(ctx, nil)
			}
		}
		task1, _ := host1.Disconnect(ctx)
		if task1 != nil {
			_, _ = task1.WaitForResult(ctx, nil)
		}

		// Step 2: Survivors overloaded
		for i := 1; i < len(hosts); i++ {
			hostRef := hosts[i].Reference()
			m.setHighCPUMetrics(hostRef, 88+i*2, expiry)
			m.setHighMemMetrics(hostRef, 85+i*2, 2048, 500, expiry)
		}

		// Step 3: Second host fails (overload)
		if len(hosts) > 1 {
			host2 := hosts[1]
			vms2, _ := m.findVMsOnHost(ctx, host2.InventoryPath)
			for _, vm := range vms2 {
				task, _ := vm.PowerOff(ctx)
				if task != nil {
					_, _ = task.WaitForResult(ctx, nil)
				}
			}
			task2, _ := host2.Disconnect(ctx)
			if task2 != nil {
				_, _ = task2.WaitForResult(ctx, nil)
			}
			hostRef2 := host2.Reference()
			m.registry.SetProperty(hostRef2, overrides.PropertyOverride{
				Property: "runtime.connectionState", Value: types.HostSystemConnectionStateNotResponding,
				ExpiresAt: expiry,
			})
		}

		// Step 4: Remaining hosts at critical levels
		for i := 2; i < len(hosts); i++ {
			hostRef := hosts[i].Reference()
			m.setHighCPUMetrics(hostRef, 96+i%4, expiry)
			m.setHighMemMetrics(hostRef, 97, 4096, 3000, expiry)
		}
	}
	return nil
}

// Scenario 29: Network Partition (Split Brain)
func (m *Manager) activateCascadeNetworkSplit(ctx context.Context, req ActivateRequest, expiry time.Time) error {
	for _, target := range req.Targets {
		hosts, err := m.findHostsInCluster(ctx, target)
		if err != nil {
			return err
		}

		// Disconnect half the hosts
		halfCount := len(hosts) / 2
		for i := 0; i < halfCount; i++ {
			task, err := hosts[i].Disconnect(ctx)
			if err != nil {
				continue
			}
			_, _ = task.WaitForResult(ctx, nil)
		}

		// Remaining hosts show network issues
		for i := halfCount; i < len(hosts); i++ {
			hostRef := hosts[i].Reference()
			m.setHighNetDrops(hostRef, 300, expiry)
		}
	}
	return nil
}

// Scenario 30: Rack Power Failure
// Multi-host + storage + network all down simultaneously
func (m *Manager) activateCascadeRackPower(ctx context.Context, req ActivateRequest, expiry time.Time) error {
	hostCount := getParamInt(req.Params, "host_count", 4)

	for _, target := range req.Targets {
		hosts, err := m.findHostsInCluster(ctx, target)
		if err != nil {
			return err
		}

		// Power off hosts in the "rack"
		affected := 0
		for _, host := range hosts {
			if affected >= hostCount {
				break
			}
			// Power off VMs first
			vms, _ := m.findVMsOnHost(ctx, host.InventoryPath)
			for _, vm := range vms {
				task, _ := vm.PowerOff(ctx)
				if task != nil {
					_, _ = task.WaitForResult(ctx, nil)
				}
			}
			// Disconnect host
			task, err := host.Disconnect(ctx)
			if err != nil {
				continue
			}
			_, _ = task.WaitForResult(ctx, nil)
			affected++

			// Set as not responding
			m.registry.SetProperty(host.Reference(), overrides.PropertyOverride{
				Property: "runtime.connectionState", Value: types.HostSystemConnectionStateNotResponding,
				ExpiresAt: expiry,
			})
		}

		// Remaining hosts absorb load
		for i := hostCount; i < len(hosts); i++ {
			hostRef := hosts[i].Reference()
			m.setHighCPUMetrics(hostRef, 90+i%5, expiry)
			m.setHighMemMetrics(hostRef, 88+i%5, 2048, 1500, expiry)
		}
	}
	return nil
}

// ==================== Deactivation Logic ====================

func (m *Manager) deactivateClearOverrides(ctx context.Context, req ActivateRequest) error {
	for _, target := range req.Targets {
		ref, err := m.resolveRef(ctx, target)
		if err != nil {
			continue
		}
		m.registry.ClearMetrics(ref)
		m.registry.ClearProperties(ref)
	}
	return nil
}

func (m *Manager) deactivateHostDisconnect(ctx context.Context, req ActivateRequest) error {
	for _, target := range req.Targets {
		host, err := m.finder.HostSystem(ctx, target)
		if err != nil {
			continue
		}
		ref := host.Reference()
		m.registry.ClearMetrics(ref)
		m.registry.ClearProperties(ref)

		// Reconnect the host
		task, err := host.Reconnect(ctx, nil, nil)
		if err != nil {
			continue
		}
		_, _ = task.WaitForResult(ctx, nil)
	}
	return nil
}

func (m *Manager) deactivateHostMaintenance(ctx context.Context, req ActivateRequest) error {
	for _, target := range req.Targets {
		host, err := m.finder.HostSystem(ctx, target)
		if err != nil {
			continue
		}
		task, err := host.ExitMaintenanceMode(ctx, 0)
		if err != nil {
			continue
		}
		_, _ = task.WaitForResult(ctx, nil)
	}
	return nil
}

func (m *Manager) deactivateVMCrash(ctx context.Context, req ActivateRequest) error {
	for _, target := range req.Targets {
		vm, err := m.finder.VirtualMachine(ctx, target)
		if err != nil {
			continue
		}
		ref := vm.Reference()
		m.registry.ClearMetrics(ref)
		m.registry.ClearProperties(ref)

		// Power the VM back on
		task, err := vm.PowerOn(ctx)
		if err != nil {
			continue
		}
		_, _ = task.WaitForResult(ctx, nil)
	}
	return nil
}
