// Package scenarios defines all failure simulation scenarios and their
// activation/deactivation logic.
package scenarios

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/site24x7/vcsim-demo/pkg/overrides"
)

// Category groups scenarios.
type Category string

const (
	CategoryCPU     Category = "cpu"
	CategoryMemory  Category = "memory"
	CategoryDisk    Category = "disk"
	CategoryNetwork Category = "network"
	CategoryHost    Category = "host"
	CategoryVM      Category = "vm"
	CategoryCascade Category = "cascade"
)

// ScenarioDef defines a scenario template.
type ScenarioDef struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Category    Category          `json:"category"`
	Description string            `json:"description"`
	Params      map[string]string `json:"params"` // key -> description of param
}

// ActiveScenario tracks a running scenario.
type ActiveScenario struct {
	ID        string                 `json:"id"`
	Targets   []string               `json:"targets"`
	Params    map[string]interface{} `json:"params"`
	StartedAt time.Time              `json:"started_at"`
}

// ActivateRequest is the request body for activating a scenario.
type ActivateRequest struct {
	ID       string                 `json:"id"`
	Targets  []string               `json:"targets"`
	Params   map[string]interface{} `json:"params,omitempty"`
	Duration int                    `json:"duration,omitempty"` // seconds, 0 = permanent
}

// Manager manages scenario lifecycle.
type Manager struct {
	mu       sync.RWMutex
	client   *govmomi.Client
	finder   *find.Finder
	registry *overrides.Registry
	active   map[string]*ActiveScenario
	defs     map[string]ScenarioDef
}

// NewManager creates a scenario manager connected to the vcsim instance.
func NewManager(client *govmomi.Client, reg *overrides.Registry) *Manager {
	m := &Manager{
		client:   client,
		finder:   find.NewFinder(client.Client, true),
		registry: reg,
		active:   make(map[string]*ActiveScenario),
		defs:     make(map[string]ScenarioDef),
	}
	m.registerAllScenarios()
	return m
}

// ListDefinitions returns all available scenario definitions.
func (m *Manager) ListDefinitions() []ScenarioDef {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []ScenarioDef
	for _, d := range m.defs {
		result = append(result, d)
	}
	return result
}

// ListActive returns all currently active scenarios.
func (m *Manager) ListActive() []*ActiveScenario {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*ActiveScenario
	for _, a := range m.active {
		result = append(result, a)
	}
	return result
}

// Activate activates a scenario.
func (m *Manager) Activate(ctx context.Context, req ActivateRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	def, ok := m.defs[req.ID]
	if !ok {
		return fmt.Errorf("unknown scenario: %s", req.ID)
	}

	var expiry time.Time
	if req.Duration > 0 {
		expiry = time.Now().Add(time.Duration(req.Duration) * time.Second)
	}

	log.Printf("[scenario] Activating: %s (%s) on %v", def.Name, def.ID, req.Targets)

	if err := m.activateScenario(ctx, req, expiry); err != nil {
		return fmt.Errorf("activate %s: %w", req.ID, err)
	}

	key := fmt.Sprintf("%s:%v", req.ID, req.Targets)
	m.active[key] = &ActiveScenario{
		ID:        req.ID,
		Targets:   req.Targets,
		Params:    req.Params,
		StartedAt: time.Now(),
	}

	return nil
}

// Deactivate deactivates a scenario.
func (m *Manager) Deactivate(ctx context.Context, req ActivateRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := fmt.Sprintf("%s:%v", req.ID, req.Targets)
	if _, ok := m.active[key]; !ok {
		return fmt.Errorf("scenario not active: %s on %v", req.ID, req.Targets)
	}

	log.Printf("[scenario] Deactivating: %s on %v", req.ID, req.Targets)

	if err := m.deactivateScenario(ctx, req); err != nil {
		return fmt.Errorf("deactivate %s: %w", req.ID, err)
	}

	delete(m.active, key)
	return nil
}

// ClearAll deactivates all active scenarios.
func (m *Manager) ClearAll(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	log.Printf("[scenario] Clearing all %d active scenarios", len(m.active))
	m.registry.ClearAll()
	m.active = make(map[string]*ActiveScenario)
	return nil
}

// registerAllScenarios populates the scenario definitions.
func (m *Manager) registerAllScenarios() {
	scenarios := []ScenarioDef{
		// ===== CPU (4) =====
		{ID: "cpu_host_saturation", Name: "Host CPU Saturation", Category: CategoryCPU,
			Description: "Single host CPU at 96-99%. All VMs on host get elevated CPU ready time.",
			Params:      map[string]string{"cpu_percent": "Target CPU usage % (default: 97)"}},
		{ID: "cpu_cluster_contention", Name: "Cluster-Wide CPU Contention", Category: CategoryCPU,
			Description: "All hosts in a cluster running at 80-95% CPU. VMs show ready time spikes.",
			Params:      map[string]string{}},
		{ID: "cpu_vm_costop", Name: "VM CPU Co-Stop (SMP)", Category: CategoryCPU,
			Description: "Multi-vCPU VM with severe co-scheduling delays.",
			Params:      map[string]string{}},
		{ID: "cpu_rp_limit", Name: "Resource Pool CPU Limit Hit", Category: CategoryCPU,
			Description: "VMs throttled by resource pool CPU limit even though host has capacity.",
			Params:      map[string]string{}},

		// ===== Memory (4) =====
		{ID: "mem_host_exhaustion", Name: "Host Memory Exhaustion (Balloon+Swap)", Category: CategoryMemory,
			Description: "Host memory at 98%. Balloon, compression, and swap active. All VMs affected.",
			Params:      map[string]string{"mem_percent": "Target memory usage % (default: 98)"}},
		{ID: "mem_vm_leak", Name: "VM Memory Leak", Category: CategoryMemory,
			Description: "Single VM consumed memory growing unbounded over time.",
			Params:      map[string]string{}},
		{ID: "mem_tps_failure", Name: "Transparent Page Sharing Failure", Category: CategoryMemory,
			Description: "TPS not deduplicating, host memory usage higher than expected.",
			Params:      map[string]string{}},
		{ID: "mem_numa_imbalance", Name: "NUMA Imbalance", Category: CategoryMemory,
			Description: "Memory allocated non-locally causing latency.",
			Params:      map[string]string{}},

		// ===== Disk/Storage (5) =====
		{ID: "disk_ds_full", Name: "Datastore Full + VM Suspension", Category: CategoryDisk,
			Description: "Datastore >95% full. VMs suspended to prevent corruption.",
			Params:      map[string]string{"free_percent": "Remaining free % (default: 2)"}},
		{ID: "disk_latency_spike", Name: "Storage Latency Spike", Category: CategoryDisk,
			Description: "SAN latency 30-100ms. All VMs on datastore experience I/O delays.",
			Params:      map[string]string{"read_latency_ms": "Read latency (default: 40)", "write_latency_ms": "Write latency (default: 60)"}},
		{ID: "disk_ds_inaccessible", Name: "Datastore Inaccessible", Category: CategoryDisk,
			Description: "Datastore completely offline. VMs become orphaned.",
			Params:      map[string]string{}},
		{ID: "disk_io_saturation", Name: "Disk I/O Saturation", Category: CategoryDisk,
			Description: "Storage IOPS hitting controller limits.",
			Params:      map[string]string{}},
		{ID: "disk_apd", Name: "All Paths Down (APD)", Category: CategoryDisk,
			Description: "Host loses all storage paths to a datastore.",
			Params:      map[string]string{}},

		// ===== Network (4) =====
		{ID: "net_mgmt_partition", Name: "Management Network Partition", Category: CategoryNetwork,
			Description: "Host loses management network. Appears disconnected, VMs orphaned.",
			Params:      map[string]string{}},
		{ID: "net_vm_isolation", Name: "VM Network Isolation", Category: CategoryNetwork,
			Description: "VMs on a portgroup lose connectivity. NICs disconnected.",
			Params:      map[string]string{}},
		{ID: "net_saturation", Name: "Network Saturation", Category: CategoryNetwork,
			Description: "Host NIC at capacity. All VMs experience packet drops.",
			Params:      map[string]string{}},
		{ID: "net_uplink_failover", Name: "DVSwitch Uplink Failover", Category: CategoryNetwork,
			Description: "Primary uplink fails, traffic fails over with brief disruption.",
			Params:      map[string]string{}},

		// ===== Host (5) =====
		{ID: "host_psod", Name: "Host PSOD (Hardware Failure)", Category: CategoryHost,
			Description: "Host crashes. All VMs powered off. HA restarts on other hosts.",
			Params:      map[string]string{}},
		{ID: "host_maintenance", Name: "Host Maintenance Mode", Category: CategoryHost,
			Description: "Host enters maintenance. VMs evacuated, destination hosts load increases.",
			Params:      map[string]string{}},
		{ID: "host_boot_storm", Name: "Host Boot Storm", Category: CategoryHost,
			Description: "3-4 hosts rebooting simultaneously. Cluster capacity drops dramatically.",
			Params:      map[string]string{"host_count": "Number of hosts to reboot (default: 3)"}},
		{ID: "host_vmotion_fail", Name: "vMotion Network Failure", Category: CategoryHost,
			Description: "vMotion VMkernel down. Cannot migrate VMs off host.",
			Params:      map[string]string{}},
		{ID: "host_time_drift", Name: "Host Time Drift", Category: CategoryHost,
			Description: "Host clock out of sync. NTP not running.",
			Params:      map[string]string{}},

		// ===== VM (4) =====
		{ID: "vm_crash", Name: "VM Unexpected Crash", Category: CategoryVM,
			Description: "VM suddenly powers off. Guest heartbeat stops before crash.",
			Params:      map[string]string{}},
		{ID: "vm_snapshot_growth", Name: "VM Snapshot Chain Growing", Category: CategoryVM,
			Description: "Forgotten snapshot chain consuming datastore space.",
			Params:      map[string]string{"snapshot_count": "Number of snapshots (default: 5)"}},
		{ID: "vm_tools_down", Name: "VMware Tools Not Running", Category: CategoryVM,
			Description: "Tools service stopped. No guest visibility.",
			Params:      map[string]string{}},
		{ID: "vm_invalid_state", Name: "VM Invalid/Orphaned State", Category: CategoryVM,
			Description: "VM shows as inaccessible in vCenter.",
			Params:      map[string]string{}},

		// ===== Cascading/Complex (4) =====
		{ID: "cascade_san_failure", Name: "SAN Array Failure (Perfect Storm)", Category: CategoryCascade,
			Description: "5 datastores go offline. ~60 VMs orphaned. Hosts show storage errors. Alarms cascade.",
			Params:      map[string]string{}},
		{ID: "cascade_ha_exhaustion", Name: "Cascading HA Failure", Category: CategoryCascade,
			Description: "Host fails → HA overloads surviving hosts → second host fails → capacity exhaustion.",
			Params:      map[string]string{}},
		{ID: "cascade_network_split", Name: "Network Partition (Split Brain)", Category: CategoryCascade,
			Description: "Half the cluster partitioned from vCenter. VMs orphaned but still running.",
			Params:      map[string]string{}},
		{ID: "cascade_rack_power", Name: "Rack Power Failure", Category: CategoryCascade,
			Description: "Entire rack loses power. Hosts, storage, network all down simultaneously.",
			Params:      map[string]string{"host_count": "Hosts in rack (default: 4)"}},
	}

	for _, s := range scenarios {
		m.defs[s.ID] = s
	}
}

// ---------- Scenario activation logic ----------

func (m *Manager) activateScenario(ctx context.Context, req ActivateRequest, expiry time.Time) error {
	switch req.ID {
	// CPU
	case "cpu_host_saturation":
		return m.activateCPUHostSaturation(ctx, req, expiry)
	case "cpu_cluster_contention":
		return m.activateCPUClusterContention(ctx, req, expiry)
	case "cpu_vm_costop":
		return m.activateCPUVMCoStop(ctx, req, expiry)
	case "cpu_rp_limit":
		return m.activateCPURPLimit(ctx, req, expiry)

	// Memory
	case "mem_host_exhaustion":
		return m.activateMemHostExhaustion(ctx, req, expiry)
	case "mem_vm_leak":
		return m.activateMemVMLeak(ctx, req, expiry)
	case "mem_tps_failure":
		return m.activateMemTPSFailure(ctx, req, expiry)
	case "mem_numa_imbalance":
		return m.activateMemNUMAImbalance(ctx, req, expiry)

	// Disk
	case "disk_ds_full":
		return m.activateDiskDSFull(ctx, req, expiry)
	case "disk_latency_spike":
		return m.activateDiskLatencySpike(ctx, req, expiry)
	case "disk_ds_inaccessible":
		return m.activateDiskDSInaccessible(ctx, req, expiry)
	case "disk_io_saturation":
		return m.activateDiskIOSaturation(ctx, req, expiry)
	case "disk_apd":
		return m.activateDiskAPD(ctx, req, expiry)

	// Network
	case "net_mgmt_partition":
		return m.activateNetMgmtPartition(ctx, req, expiry)
	case "net_vm_isolation":
		return m.activateNetVMIsolation(ctx, req, expiry)
	case "net_saturation":
		return m.activateNetSaturation(ctx, req, expiry)
	case "net_uplink_failover":
		return m.activateNetUplinkFailover(ctx, req, expiry)

	// Host
	case "host_psod":
		return m.activateHostPSOD(ctx, req, expiry)
	case "host_maintenance":
		return m.activateHostMaintenance(ctx, req, expiry)
	case "host_boot_storm":
		return m.activateHostBootStorm(ctx, req, expiry)
	case "host_vmotion_fail":
		return m.activateHostVMotionFail(ctx, req, expiry)
	case "host_time_drift":
		return m.activateHostTimeDrift(ctx, req, expiry)

	// VM
	case "vm_crash":
		return m.activateVMCrash(ctx, req, expiry)
	case "vm_snapshot_growth":
		return m.activateVMSnapshotGrowth(ctx, req, expiry)
	case "vm_tools_down":
		return m.activateVMToolsDown(ctx, req, expiry)
	case "vm_invalid_state":
		return m.activateVMInvalidState(ctx, req, expiry)

	// Cascade
	case "cascade_san_failure":
		return m.activateCascadeSANFailure(ctx, req, expiry)
	case "cascade_ha_exhaustion":
		return m.activateCascadeHAExhaustion(ctx, req, expiry)
	case "cascade_network_split":
		return m.activateCascadeNetworkSplit(ctx, req, expiry)
	case "cascade_rack_power":
		return m.activateCascadeRackPower(ctx, req, expiry)

	default:
		return fmt.Errorf("no activation logic for: %s", req.ID)
	}
}

func (m *Manager) deactivateScenario(ctx context.Context, req ActivateRequest) error {
	// For most scenarios, clearing all overrides on the targets is sufficient.
	// For state-change scenarios (host disconnect, VM power off), we need to
	// reverse the action.
	switch req.ID {
	case "host_psod", "net_mgmt_partition":
		return m.deactivateHostDisconnect(ctx, req)
	case "host_maintenance":
		return m.deactivateHostMaintenance(ctx, req)
	case "vm_crash":
		return m.deactivateVMCrash(ctx, req)
	default:
		return m.deactivateClearOverrides(ctx, req)
	}
}

// ---------- Helper functions ----------

func (m *Manager) resolveRef(ctx context.Context, path string) (types.ManagedObjectReference, error) {
	elements, err := m.finder.ManagedObjectList(ctx, path)
	if err != nil {
		return types.ManagedObjectReference{}, fmt.Errorf("resolve %s: %w", path, err)
	}
	if len(elements) == 0 {
		return types.ManagedObjectReference{}, fmt.Errorf("not found: %s", path)
	}
	ref := elements[0].Object.Reference()
	return ref, nil
}

func (m *Manager) findVMsOnHost(ctx context.Context, hostPath string) ([]*object.VirtualMachine, error) {
	host, err := m.finder.HostSystem(ctx, hostPath)
	if err != nil {
		return nil, err
	}
	return m.finder.VirtualMachineList(ctx, fmt.Sprintf("*/%s/*", host.Name()))
}

func (m *Manager) findHostsInCluster(ctx context.Context, clusterPath string) ([]*object.HostSystem, error) {
	return m.finder.HostSystemList(ctx, clusterPath+"/*")
}

func getParamInt(params map[string]interface{}, key string, defaultVal int) int {
	if v, ok := params[key]; ok {
		switch val := v.(type) {
		case float64:
			return int(val)
		case int:
			return val
		}
	}
	return defaultVal
}

// setHighCPUMetrics sets CPU saturation metrics on a host or VM.
func (m *Manager) setHighCPUMetrics(ref types.ManagedObjectReference, cpuPercent int, expiry time.Time) {
	// cpu.usage.average counter ID = 2 (standard vSphere counter)
	m.registry.SetMetric(ref, overrides.MetricOverride{
		CounterID: 2,                                            // cpu.usage.average
		Values:    generateHighValues(int64(cpuPercent*100), 5), // centipercent
		ExpiresAt: expiry,
	})
}

// setHighCPUReady sets elevated CPU ready time on a VM.
func (m *Manager) setHighCPUReady(ref types.ManagedObjectReference, readyMs int, expiry time.Time) {
	m.registry.SetMetric(ref, overrides.MetricOverride{
		CounterID: 12, // cpu.ready.summation
		Values:    generateHighValues(int64(readyMs), 20),
		ExpiresAt: expiry,
	})
}

// setHighMemMetrics sets memory pressure metrics on a host.
func (m *Manager) setHighMemMetrics(ref types.ManagedObjectReference, memPercent int, balloonMB int, swapRate int, expiry time.Time) {
	m.registry.SetMetric(ref, overrides.MetricOverride{
		CounterID: 24, // mem.usage.average
		Values:    generateHighValues(int64(memPercent*100), 3),
		ExpiresAt: expiry,
	})
	if balloonMB > 0 {
		m.registry.SetMetric(ref, overrides.MetricOverride{
			CounterID: 90,                                            // mem.vmmemctl.average (balloon)
			Values:    generateHighValues(int64(balloonMB*1024), 10), // KB
			ExpiresAt: expiry,
		})
	}
	if swapRate > 0 {
		m.registry.SetMetric(ref, overrides.MetricOverride{
			CounterID: 85, // mem.swapinRate.average
			Values:    generateHighValues(int64(swapRate), 30),
			ExpiresAt: expiry,
		})
		m.registry.SetMetric(ref, overrides.MetricOverride{
			CounterID: 86, // mem.swapoutRate.average
			Values:    generateHighValues(int64(swapRate/2), 30),
			ExpiresAt: expiry,
		})
	}
}

// setHighDiskLatency sets elevated disk latency metrics.
func (m *Manager) setHighDiskLatency(ref types.ManagedObjectReference, readLatency, writeLatency int, expiry time.Time) {
	m.registry.SetMetric(ref, overrides.MetricOverride{
		CounterID: 449, // disk.totalReadLatency.average
		Values:    generateHighValues(int64(readLatency), 15),
		ExpiresAt: expiry,
	})
	m.registry.SetMetric(ref, overrides.MetricOverride{
		CounterID: 453, // disk.totalWriteLatency.average
		Values:    generateHighValues(int64(writeLatency), 15),
		ExpiresAt: expiry,
	})
	m.registry.SetMetric(ref, overrides.MetricOverride{
		CounterID: 133, // disk.maxTotalLatency.latest
		Values:    generateHighValues(int64(writeLatency*2), 20),
		ExpiresAt: expiry,
	})
}

// setHighNetDrops sets elevated network drop metrics.
func (m *Manager) setHighNetDrops(ref types.ManagedObjectReference, dropsPerSec int, expiry time.Time) {
	m.registry.SetMetric(ref, overrides.MetricOverride{
		CounterID: 460, // net.droppedRx.summation
		Values:    generateIncrementingValues(int64(dropsPerSec), 10),
		ExpiresAt: expiry,
	})
	m.registry.SetMetric(ref, overrides.MetricOverride{
		CounterID: 461, // net.droppedTx.summation
		Values:    generateIncrementingValues(int64(dropsPerSec/2), 10),
		ExpiresAt: expiry,
	})
}

// generateHighValues creates a slice of values that hover around a target with jitter.
func generateHighValues(target int64, jitterPercent int) []int64 {
	values := make([]int64, 12)
	for i := range values {
		jitter := target * int64(jitterPercent) / 100
		// Simple deterministic "jitter" pattern
		offset := int64([]int{0, 1, -1, 2, -2, 0, 1, -1, 3, -2, 0, 2}[i%12])
		v := target + offset*jitter/3
		if v < 0 {
			v = 0
		}
		values[i] = v
	}
	return values
}

// generateIncrementingValues creates values that increment over time (for summation counters).
func generateIncrementingValues(ratePerInterval int64, count int) []int64 {
	values := make([]int64, count)
	for i := range values {
		values[i] = ratePerInterval * int64(i+1)
	}
	return values
}
