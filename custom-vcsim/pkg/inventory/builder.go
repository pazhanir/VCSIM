// Package inventory builds a large-scale vSphere inventory programmatically
// using the govmomi simulator and govc-style API calls.
// Target: 5 DCs, 14 clusters, 91 hosts, ~871 VMs, 48+ shared datastores, 7 DVSwitches.
package inventory

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"
)

// Config describes the desired inventory layout.
type Config struct {
	Datacenters []DatacenterConfig
}

// DatacenterConfig describes a single datacenter.
type DatacenterConfig struct {
	Name       string
	Clusters   []ClusterConfig
	Datastores []DatastoreConfig
	DVSwitches []DVSwitchConfig
}

// ClusterConfig describes a cluster with hosts, resource pools, and VMs.
type ClusterConfig struct {
	Name          string
	HostCount     int
	HostPrefix    string
	ResourcePools []ResourcePoolConfig
}

// ResourcePoolConfig describes a resource pool and its VMs.
type ResourcePoolConfig struct {
	Name     string
	VMCount  int
	VMPrefix string
}

// DatastoreConfig describes a datastore.
type DatastoreConfig struct {
	Name       string
	Type       string // "SAN", "NFS", "local"
	CapacityGB int64
}

// DVSwitchConfig describes a distributed virtual switch.
type DVSwitchConfig struct {
	Name       string
	Portgroups []string
}

// DefaultConfig returns the large-scale datacenter configuration.
func DefaultConfig() Config {
	return Config{
		Datacenters: []DatacenterConfig{
			dcUSEast(),
			dcUSWest(),
			dcEU(),
			dcAPAC(),
			dcDevLab(),
		},
	}
}

// SimpleConfig generates a flat-scaling inventory layout from numeric parameters.
// Each datacenter gets identical structure: N clusters, M hosts per cluster,
// V VMs per cluster (distributed across 3 resource pools), D datastores, S DVSwitches.
func SimpleConfig(dcs, clustersPerDC, hostsPerCluster, vmsPerCluster, dsPerDC, dvsPerDC int) Config {
	// Enforce minimums
	if dcs < 1 {
		dcs = 1
	}
	if clustersPerDC < 1 {
		clustersPerDC = 1
	}
	if hostsPerCluster < 1 {
		hostsPerCluster = 1
	}
	if vmsPerCluster < 0 {
		vmsPerCluster = 0
	}
	if dsPerDC < 1 {
		dsPerDC = 1
	}
	if dvsPerDC < 0 {
		dvsPerDC = 0
	}

	cfg := Config{}

	for d := 1; d <= dcs; d++ {
		dcName := fmt.Sprintf("DC-%d", d)

		// Build clusters
		clusters := make([]ClusterConfig, 0, clustersPerDC)
		for c := 1; c <= clustersPerDC; c++ {
			hostPrefix := fmt.Sprintf("ESXi-%d-%d-", d, c) // e.g., ESXi-1-2-01

			// Distribute VMs across 3 resource pools: Web, App, DB
			rpNames := []struct{ name, prefix string }{
				{"RP-Web", "vm-web"},
				{"RP-App", "vm-app"},
				{"RP-DB", "vm-db"},
			}
			rps := make([]ResourcePoolConfig, 0, len(rpNames))
			remaining := vmsPerCluster
			for ri, rp := range rpNames {
				var count int
				if ri == len(rpNames)-1 {
					count = remaining // last pool gets remainder
				} else {
					count = vmsPerCluster / len(rpNames)
					remaining -= count
				}
				if count > 0 {
					rps = append(rps, ResourcePoolConfig{
						Name:     rp.name,
						VMCount:  count,
						VMPrefix: fmt.Sprintf("%s-d%dc%d", rp.prefix, d, c),
					})
				}
			}

			clusters = append(clusters, ClusterConfig{
				Name:          fmt.Sprintf("Cluster-%d", c),
				HostCount:     hostsPerCluster,
				HostPrefix:    hostPrefix,
				ResourcePools: rps,
			})
		}

		// Build datastores
		datastores := make([]DatastoreConfig, 0, dsPerDC)
		for ds := 1; ds <= dsPerDC; ds++ {
			datastores = append(datastores, DatastoreConfig{
				Name:       fmt.Sprintf("SAN-%d-%02d", d, ds),
				Type:       "SAN",
				CapacityGB: 4096,
			})
		}

		// Build DVSwitches
		dvSwitches := make([]DVSwitchConfig, 0, dvsPerDC)
		for dv := 1; dv <= dvsPerDC; dv++ {
			dvSwitches = append(dvSwitches, DVSwitchConfig{
				Name: fmt.Sprintf("DVS-%d-%d", d, dv),
				Portgroups: []string{
					fmt.Sprintf("DVPG-Prod-%d-%d", d, dv),
					fmt.Sprintf("DVPG-Mgmt-%d-%d", d, dv),
					fmt.Sprintf("DVPG-vMotion-%d-%d", d, dv),
				},
			})
		}

		cfg.Datacenters = append(cfg.Datacenters, DatacenterConfig{
			Name:       dcName,
			Clusters:   clusters,
			Datastores: datastores,
			DVSwitches: dvSwitches,
		})
	}

	return cfg
}

func dcUSEast() DatacenterConfig {
	return DatacenterConfig{
		Name: "DC-US-East",
		Clusters: []ClusterConfig{
			{
				Name: "Cluster-WebTier", HostCount: 8, HostPrefix: "ESXi-WEB-E",
				ResourcePools: []ResourcePoolConfig{
					{Name: "RP-FrontEnd", VMCount: 48, VMPrefix: "web-fe"},
					{Name: "RP-CDN-Cache", VMCount: 48, VMPrefix: "cdn-cache"},
				},
			},
			{
				Name: "Cluster-AppTier", HostCount: 10, HostPrefix: "ESXi-APP-E",
				ResourcePools: []ResourcePoolConfig{
					{Name: "RP-Microservices", VMCount: 60, VMPrefix: "msvc"},
					{Name: "RP-API-Gateway", VMCount: 20, VMPrefix: "apigw"},
					{Name: "RP-MessageQueue", VMCount: 20, VMPrefix: "mq"},
				},
			},
			{
				Name: "Cluster-DBTier", HostCount: 6, HostPrefix: "ESXi-DB-E",
				ResourcePools: []ResourcePoolConfig{
					{Name: "RP-MySQL", VMCount: 16, VMPrefix: "mysql"},
					{Name: "RP-PostgreSQL", VMCount: 16, VMPrefix: "pgsql"},
					{Name: "RP-Redis", VMCount: 16, VMPrefix: "redis"},
				},
			},
			{
				Name: "Cluster-BigData", HostCount: 8, HostPrefix: "ESXi-BD-E",
				ResourcePools: []ResourcePoolConfig{
					{Name: "RP-Hadoop", VMCount: 40, VMPrefix: "hadoop"},
					{Name: "RP-Spark", VMCount: 40, VMPrefix: "spark"},
				},
			},
		},
		Datastores: []DatastoreConfig{
			{Name: "SAN-Prod-E01", Type: "SAN", CapacityGB: 4096},
			{Name: "SAN-Prod-E02", Type: "SAN", CapacityGB: 4096},
			{Name: "SAN-Prod-E03", Type: "SAN", CapacityGB: 4096},
			{Name: "SAN-Prod-E04", Type: "SAN", CapacityGB: 4096},
			{Name: "SAN-Prod-E05", Type: "SAN", CapacityGB: 4096},
			{Name: "SAN-Prod-E06", Type: "SAN", CapacityGB: 4096},
			{Name: "SAN-Prod-E07", Type: "SAN", CapacityGB: 4096},
			{Name: "SAN-Prod-E08", Type: "SAN", CapacityGB: 4096},
			{Name: "SAN-Prod-E09", Type: "SAN", CapacityGB: 4096},
			{Name: "SAN-Prod-E10", Type: "SAN", CapacityGB: 4096},
			{Name: "NFS-Shared-E01", Type: "NFS", CapacityGB: 2048},
			{Name: "NFS-Shared-E02", Type: "NFS", CapacityGB: 2048},
			{Name: "NFS-Shared-E03", Type: "NFS", CapacityGB: 2048},
		},
		DVSwitches: []DVSwitchConfig{
			{
				Name: "DVS-Production-East",
				Portgroups: []string{
					"DVPG-Prod-VLAN100",
					"DVPG-Mgmt-VLAN10",
					"DVPG-vMotion-VLAN20",
					"DVPG-Storage-VLAN30",
					"DVPG-DB-VLAN110",
					"DVPG-BigData-VLAN120",
					"DVPG-Backup-VLAN40",
				},
			},
		},
	}
}

func dcUSWest() DatacenterConfig {
	return DatacenterConfig{
		Name: "DC-US-West",
		Clusters: []ClusterConfig{
			{
				Name: "Cluster-Prod-West", HostCount: 8, HostPrefix: "ESXi-PROD-W",
				ResourcePools: []ResourcePoolConfig{
					{Name: "RP-WebTier", VMCount: 30, VMPrefix: "web-w"},
					{Name: "RP-AppTier", VMCount: 30, VMPrefix: "app-w"},
					{Name: "RP-DBTier", VMCount: 20, VMPrefix: "db-w"},
				},
			},
			{
				Name: "Cluster-DR", HostCount: 6, HostPrefix: "ESXi-DR-W",
				ResourcePools: []ResourcePoolConfig{
					{Name: "RP-DR-Active", VMCount: 36, VMPrefix: "dr-active"},
					{Name: "RP-DR-Standby", VMCount: 36, VMPrefix: "dr-standby"},
				},
			},
			{
				Name: "Cluster-Edge", HostCount: 4, HostPrefix: "ESXi-EDGE-W",
				ResourcePools: []ResourcePoolConfig{
					{Name: "RP-EdgeServices", VMCount: 32, VMPrefix: "edge"},
				},
			},
		},
		Datastores: []DatastoreConfig{
			{Name: "SAN-Prod-W01", Type: "SAN", CapacityGB: 4096},
			{Name: "SAN-Prod-W02", Type: "SAN", CapacityGB: 4096},
			{Name: "SAN-Prod-W03", Type: "SAN", CapacityGB: 4096},
			{Name: "SAN-Prod-W04", Type: "SAN", CapacityGB: 4096},
			{Name: "SAN-Prod-W05", Type: "SAN", CapacityGB: 4096},
			{Name: "SAN-Prod-W06", Type: "SAN", CapacityGB: 4096},
			{Name: "SAN-DR-W01", Type: "SAN", CapacityGB: 8192},
			{Name: "SAN-DR-W02", Type: "SAN", CapacityGB: 8192},
			{Name: "SAN-DR-W03", Type: "SAN", CapacityGB: 8192},
			{Name: "SAN-DR-W04", Type: "SAN", CapacityGB: 8192},
			{Name: "NFS-DR-W01", Type: "NFS", CapacityGB: 4096},
		},
		DVSwitches: []DVSwitchConfig{
			{
				Name: "DVS-Production-West",
				Portgroups: []string{
					"DVPG-Prod-W-VLAN100",
					"DVPG-Mgmt-W-VLAN10",
					"DVPG-vMotion-W-VLAN20",
				},
			},
			{
				Name: "DVS-DR",
				Portgroups: []string{
					"DVPG-DR-VLAN300",
					"DVPG-DR-Replication-VLAN301",
				},
			},
		},
	}
}

func dcEU() DatacenterConfig {
	return DatacenterConfig{
		Name: "DC-EU-Frankfurt",
		Clusters: []ClusterConfig{
			{
				Name: "Cluster-EU-Prod", HostCount: 6, HostPrefix: "ESXi-EU",
				ResourcePools: []ResourcePoolConfig{
					{Name: "RP-EU-Web", VMCount: 20, VMPrefix: "eu-web"},
					{Name: "RP-EU-App", VMCount: 25, VMPrefix: "eu-app"},
					{Name: "RP-EU-DB", VMCount: 15, VMPrefix: "eu-db"},
				},
			},
			{
				Name: "Cluster-EU-Compliance", HostCount: 4, HostPrefix: "ESXi-EU-C",
				ResourcePools: []ResourcePoolConfig{
					{Name: "RP-GDPR-Workloads", VMCount: 32, VMPrefix: "gdpr"},
				},
			},
		},
		Datastores: []DatastoreConfig{
			{Name: "SAN-EU-01", Type: "SAN", CapacityGB: 4096},
			{Name: "SAN-EU-02", Type: "SAN", CapacityGB: 4096},
			{Name: "SAN-EU-03", Type: "SAN", CapacityGB: 4096},
			{Name: "SAN-EU-04", Type: "SAN", CapacityGB: 4096},
		},
		DVSwitches: []DVSwitchConfig{
			{
				Name: "DVS-EU",
				Portgroups: []string{
					"DVPG-EU-Prod-VLAN100",
					"DVPG-EU-Mgmt-VLAN10",
				},
			},
		},
	}
}

func dcAPAC() DatacenterConfig {
	return DatacenterConfig{
		Name: "DC-APAC-Singapore",
		Clusters: []ClusterConfig{
			{
				Name: "Cluster-APAC-Prod", HostCount: 6, HostPrefix: "ESXi-APAC",
				ResourcePools: []ResourcePoolConfig{
					{Name: "RP-APAC-Web", VMCount: 16, VMPrefix: "apac-web"},
					{Name: "RP-APAC-App", VMCount: 20, VMPrefix: "apac-app"},
					{Name: "RP-APAC-DB", VMCount: 12, VMPrefix: "apac-db"},
				},
			},
			{
				Name: "Cluster-APAC-Dev", HostCount: 4, HostPrefix: "ESXi-APAC-D",
				ResourcePools: []ResourcePoolConfig{
					{Name: "RP-Dev", VMCount: 20, VMPrefix: "apac-dev"},
					{Name: "RP-QA", VMCount: 20, VMPrefix: "apac-qa"},
				},
			},
		},
		Datastores: []DatastoreConfig{
			{Name: "SAN-APAC-01", Type: "SAN", CapacityGB: 4096},
			{Name: "SAN-APAC-02", Type: "SAN", CapacityGB: 4096},
			{Name: "SAN-APAC-03", Type: "SAN", CapacityGB: 4096},
		},
		DVSwitches: []DVSwitchConfig{
			{
				Name: "DVS-APAC",
				Portgroups: []string{
					"DVPG-APAC-Prod-VLAN100",
					"DVPG-APAC-Mgmt-VLAN10",
				},
			},
		},
	}
}

func dcDevLab() DatacenterConfig {
	return DatacenterConfig{
		Name: "DC-Dev-Lab",
		Clusters: []ClusterConfig{
			{
				Name: "Cluster-Dev", HostCount: 6, HostPrefix: "ESXi-DEV",
				ResourcePools: []ResourcePoolConfig{
					{Name: "RP-Dev-Team-Alpha", VMCount: 30, VMPrefix: "dev-alpha"},
					{Name: "RP-Dev-Team-Beta", VMCount: 30, VMPrefix: "dev-beta"},
					{Name: "RP-CI-CD", VMCount: 30, VMPrefix: "cicd"},
				},
			},
			{
				Name: "Cluster-Staging", HostCount: 4, HostPrefix: "ESXi-STG",
				ResourcePools: []ResourcePoolConfig{
					{Name: "RP-Staging-Int", VMCount: 24, VMPrefix: "stg-int"},
					{Name: "RP-Staging-Perf", VMCount: 24, VMPrefix: "stg-perf"},
				},
			},
			{
				Name: "Cluster-Sandbox", HostCount: 4, HostPrefix: "ESXi-SBX",
				ResourcePools: []ResourcePoolConfig{
					{Name: "RP-Sandbox", VMCount: 40, VMPrefix: "sbx"},
				},
			},
		},
		Datastores: []DatastoreConfig{
			{Name: "SAN-Dev-01", Type: "SAN", CapacityGB: 2048},
			{Name: "SAN-Dev-02", Type: "SAN", CapacityGB: 2048},
			{Name: "SAN-Dev-03", Type: "SAN", CapacityGB: 2048},
			{Name: "SAN-Dev-04", Type: "SAN", CapacityGB: 2048},
			{Name: "NFS-Dev-01", Type: "NFS", CapacityGB: 1024},
		},
		DVSwitches: []DVSwitchConfig{
			{
				Name: "DVS-DevLab",
				Portgroups: []string{
					"DVPG-Dev-VLAN200",
					"DVPG-QA-VLAN201",
					"DVPG-Staging-VLAN202",
				},
			},
		},
	}
}

// Builder creates the full inventory on a running vcsim instance.
type Builder struct {
	client *govmomi.Client
	finder *find.Finder
	config Config
}

// NewBuilder creates a new inventory builder.
func NewBuilder(vcURL *url.URL, cfg Config) (*Builder, error) {
	ctx := context.Background()
	client, err := govmomi.NewClient(ctx, vcURL, true)
	if err != nil {
		return nil, fmt.Errorf("connect to vcsim: %w", err)
	}
	return &Builder{
		client: client,
		finder: find.NewFinder(client.Client, true),
		config: cfg,
	}, nil
}

// Build creates all inventory objects. Call this after vcsim is running
// with -dc 0 (empty inventory).
func (b *Builder) Build(ctx context.Context) error {
	for _, dcCfg := range b.config.Datacenters {
		if err := b.buildDatacenter(ctx, dcCfg); err != nil {
			return fmt.Errorf("build datacenter %s: %w", dcCfg.Name, err)
		}
	}
	return nil
}

func (b *Builder) buildDatacenter(ctx context.Context, cfg DatacenterConfig) error {
	log.Printf("[inventory] Creating datacenter: %s", cfg.Name)

	// Get root folder
	rootFolder := object.NewRootFolder(b.client.Client)
	dc, err := rootFolder.CreateDatacenter(ctx, cfg.Name)
	if err != nil {
		return fmt.Errorf("create datacenter: %w", err)
	}
	dc.SetInventoryPath("/" + cfg.Name)

	b.finder.SetDatacenter(dc)

	// Create clusters, hosts, resource pools, and VMs
	for _, clusterCfg := range cfg.Clusters {
		if err := b.buildCluster(ctx, dc, clusterCfg); err != nil {
			return fmt.Errorf("build cluster %s: %w", clusterCfg.Name, err)
		}
	}

	log.Printf("[inventory] Datacenter %s complete", cfg.Name)
	return nil
}

func (b *Builder) buildCluster(ctx context.Context, dc *object.Datacenter, cfg ClusterConfig) error {
	log.Printf("[inventory]   Creating cluster: %s (%d hosts)", cfg.Name, cfg.HostCount)

	hostFolder, err := dc.Folders(ctx)
	if err != nil {
		return err
	}

	clusterSpec := types.ClusterConfigSpecEx{
		DasConfig: &types.ClusterDasConfigInfo{
			Enabled:                 types.NewBool(true),
			AdmissionControlEnabled: types.NewBool(true),
			FailoverLevel:           1,
			DefaultVmSettings: &types.ClusterDasVmSettings{
				RestartPriority:        string(types.ClusterDasVmSettingsRestartPriorityMedium),
				RestartPriorityTimeout: 600,
				IsolationResponse:      string(types.ClusterDasVmSettingsIsolationResponsePowerOff),
				VmToolsMonitoringSettings: &types.ClusterVmToolsMonitoringSettings{
					Enabled:          types.NewBool(true),
					FailureInterval:  30,
					MinUpTime:        120,
					MaxFailures:      3,
					MaxFailureWindow: 3600,
				},
			},
		},
		DrsConfig: &types.ClusterDrsConfigInfo{
			Enabled:                   types.NewBool(true),
			DefaultVmBehavior:         types.DrsBehaviorFullyAutomated,
			VmotionRate:               3,
			EnableVmBehaviorOverrides: types.NewBool(true),
		},
	}

	cluster, err := hostFolder.HostFolder.CreateCluster(ctx, cfg.Name, clusterSpec)
	if err != nil {
		return fmt.Errorf("create cluster: %w", err)
	}

	// Add hosts to cluster, collecting host objects for datastore creation
	var hostRefs []*object.HostSystem
	for i := 1; i <= cfg.HostCount; i++ {
		hostName := fmt.Sprintf("%s%02d", cfg.HostPrefix, i)

		// Register the hardware profile for this host BEFORE creation,
		// so the HostCustomizationFunc hook can look it up.
		profile := ProfileForHost(cfg.Name, i)
		RegisterHostProfile(hostName, profile)

		hostRef, err := b.addHostToCluster(ctx, cluster, hostName)
		if err != nil {
			return fmt.Errorf("add host %s: %w", hostName, err)
		}
		hostRefs = append(hostRefs, hostRef)
	}

	// Create a shared local datastore on all hosts in this cluster.
	// The datastore name is unique per cluster to avoid conflicts.
	dsName := fmt.Sprintf("DS-%s", cfg.Name)
	if err := b.createSharedDatastore(ctx, dsName, hostRefs); err != nil {
		return fmt.Errorf("create datastore %s: %w", dsName, err)
	}

	// Get the cluster's root resource pool
	pool, err := cluster.ResourcePool(ctx)
	if err != nil {
		return fmt.Errorf("get cluster resource pool: %w", err)
	}

	// Create resource pools and VMs
	for _, rpCfg := range cfg.ResourcePools {
		if err := b.buildResourcePool(ctx, pool, rpCfg, dc, dsName); err != nil {
			return fmt.Errorf("build resource pool %s: %w", rpCfg.Name, err)
		}
	}

	return nil
}

func (b *Builder) addHostToCluster(ctx context.Context, cluster *object.ClusterComputeResource, hostName string) (*object.HostSystem, error) {
	spec := types.HostConnectSpec{
		HostName: hostName,
		UserName: "root",
		Password: "password",
		Force:    true,
	}

	task, err := cluster.AddHost(ctx, spec, true, nil, nil)
	if err != nil {
		return nil, err
	}

	result, err := task.WaitForResult(ctx, nil)
	if err != nil {
		return nil, err
	}

	hostRef := result.Result.(types.ManagedObjectReference)
	return object.NewHostSystem(b.client.Client, hostRef), nil
}

// createSharedDatastore creates a local datastore with the given name on every
// host in the list. The simulator's CreateLocalDatastore requires a real local
// directory path, so we create a temp directory for the datastore backing.
func (b *Builder) createSharedDatastore(ctx context.Context, name string, hosts []*object.HostSystem) error {
	dir, err := os.MkdirTemp("", "vcsim-ds-"+name+"-")
	if err != nil {
		return fmt.Errorf("create temp dir for datastore %s: %w", name, err)
	}
	// Each host needs its own subdirectory to avoid conflicts
	for i, host := range hosts {
		hostDir := filepath.Join(dir, fmt.Sprintf("host%d", i))
		if err := os.MkdirAll(hostDir, 0700); err != nil {
			return fmt.Errorf("create host dir: %w", err)
		}

		dss, err := host.ConfigManager().DatastoreSystem(ctx)
		if err != nil {
			return fmt.Errorf("get datastore system for host: %w", err)
		}

		_, err = dss.CreateLocalDatastore(ctx, name, hostDir)
		if err != nil {
			return fmt.Errorf("create local datastore on host: %w", err)
		}
	}
	return nil
}

func (b *Builder) buildResourcePool(ctx context.Context, parent *object.ResourcePool, cfg ResourcePoolConfig, dc *object.Datacenter, dsName string) error {
	log.Printf("[inventory]     Creating resource pool: %s (%d VMs)", cfg.Name, cfg.VMCount)

	spec := types.ResourceConfigSpec{
		CpuAllocation: types.ResourceAllocationInfo{
			Reservation:           types.NewInt64(0),
			ExpandableReservation: types.NewBool(true),
			Limit:                 types.NewInt64(-1),
			Shares: &types.SharesInfo{
				Level: types.SharesLevelNormal,
			},
		},
		MemoryAllocation: types.ResourceAllocationInfo{
			Reservation:           types.NewInt64(0),
			ExpandableReservation: types.NewBool(true),
			Limit:                 types.NewInt64(-1),
			Shares: &types.SharesInfo{
				Level: types.SharesLevelNormal,
			},
		},
	}

	rp, err := parent.Create(ctx, cfg.Name, spec)
	if err != nil {
		return fmt.Errorf("create resource pool: %w", err)
	}

	// Create VMs in this resource pool
	for i := 1; i <= cfg.VMCount; i++ {
		vmName := fmt.Sprintf("%s-%03d", cfg.VMPrefix, i)
		if err := b.createVM(ctx, rp, vmName, dc, dsName); err != nil {
			log.Printf("[inventory]       Warning: failed to create VM %s: %v", vmName, err)
		}
	}

	return nil
}

func (b *Builder) createVM(ctx context.Context, pool *object.ResourcePool, name string, dc *object.Datacenter, dsName string) error {
	// Use the specific datacenter passed in — NOT the finder's current DC,
	// which may point to a different datacenter.
	dcFolders, err := dc.Folders(ctx)
	if err != nil {
		return fmt.Errorf("get DC folders: %w", err)
	}

	dsPath := fmt.Sprintf("[%s]", dsName)

	annotation := fmt.Sprintf("Managed VM: %s | DC: %s | Datastore: %s", name, dc.Name(), dsName)

	spec := types.VirtualMachineConfigSpec{
		Name:       name,
		GuestId:    string(types.VirtualMachineGuestOsIdentifierOtherLinux64Guest),
		NumCPUs:    2,
		MemoryMB:   4096,
		Annotation: annotation,
		Files: &types.VirtualMachineFileInfo{
			VmPathName: dsPath,
		},
	}

	task, err := dcFolders.VmFolder.CreateVM(ctx, spec, pool, nil)
	if err != nil {
		return err
	}

	result, err := task.WaitForResult(ctx, nil)
	if err != nil {
		return err
	}

	// Power on the VM
	vm := object.NewVirtualMachine(b.client.Client, result.Result.(types.ManagedObjectReference))
	powerTask, err := vm.PowerOn(ctx)
	if err != nil {
		return nil // Non-fatal, VM created but not powered on
	}
	_, _ = powerTask.WaitForResult(ctx, nil)

	return nil
}

// Close releases the builder's client connection.
func (b *Builder) Close(ctx context.Context) error {
	return b.client.Logout(ctx)
}
