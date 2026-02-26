// Hardware profiles for realistic vendor/model/CPU diversity across hosts.
//
// Each datacenter uses a primary vendor (reflecting real-world procurement),
// with some clusters using a secondary vendor for variety.
package inventory

import (
	"sync"

	"github.com/vmware/govmomi/vim25/types"
)

// hostProfileRegistry maps hostnames to their hardware profiles.
// It is populated during inventory build (before hosts are created) and
// looked up by the HostCustomizationFunc hook when vcsim configures each host.
var (
	hostProfileMu       sync.RWMutex
	hostProfileRegistry = make(map[string]*HardwareProfile)
)

// RegisterHostProfile associates a hostname with a hardware profile.
// Call this before the host is created so the customization hook can find it.
func RegisterHostProfile(hostname string, profile *HardwareProfile) {
	hostProfileMu.Lock()
	defer hostProfileMu.Unlock()
	hostProfileRegistry[hostname] = profile
}

// LookupHostProfile returns the hardware profile for a hostname, or nil.
func LookupHostProfile(hostname string) *HardwareProfile {
	hostProfileMu.RLock()
	defer hostProfileMu.RUnlock()
	return hostProfileRegistry[hostname]
}

// HardwareProfile defines the vendor, model, CPU, memory, and BIOS
// characteristics for a simulated ESXi host.
type HardwareProfile struct {
	// Server identity
	Vendor string // e.g. "Dell Inc."
	Model  string // e.g. "PowerEdge R750"

	// CPU
	CPUModel       string // e.g. "Intel(R) Xeon(R) Gold 6338 CPU @ 2.00GHz"
	CPUVendor      string // "intel" or "amd"
	CPUDescription string // Full description for CpuPackage
	CPUHz          int64  // Per-core frequency in Hz
	CPUMhz         int32  // Per-core frequency in MHz (for Summary)
	NumCPUPkgs     int16  // Physical sockets
	NumCPUCores    int16  // Total physical cores
	NumCPUThreads  int16  // Total logical threads

	// Memory
	MemoryBytes int64 // Total RAM in bytes

	// BIOS
	BIOSVendor  string
	BIOSVersion string

	// Network / Storage
	NumNics int32
	NumHBAs int32
}

// Apply sets the profile's values on the host's Summary.Hardware and Hardware structs.
func (p *HardwareProfile) Apply(summary *types.HostHardwareSummary, hardware *types.HostHardwareInfo) {
	// Summary.Hardware
	summary.Vendor = p.Vendor
	summary.Model = p.Model
	summary.CpuModel = p.CPUModel
	summary.CpuMhz = p.CPUMhz
	summary.NumCpuPkgs = p.NumCPUPkgs
	summary.NumCpuCores = p.NumCPUCores
	summary.NumCpuThreads = p.NumCPUThreads
	summary.MemorySize = p.MemoryBytes
	summary.NumNics = p.NumNics
	summary.NumHBAs = p.NumHBAs

	// Hardware.SystemInfo
	hardware.SystemInfo.Vendor = p.Vendor
	hardware.SystemInfo.Model = p.Model

	// Hardware.BiosInfo
	if hardware.BiosInfo != nil {
		hardware.BiosInfo.Vendor = p.BIOSVendor
		hardware.BiosInfo.BiosVersion = p.BIOSVersion
	}

	// Hardware.CpuPkg — update all CPU packages
	for i := range hardware.CpuPkg {
		hardware.CpuPkg[i].Vendor = p.CPUVendor
		hardware.CpuPkg[i].Hz = p.CPUHz
		hardware.CpuPkg[i].Description = p.CPUDescription
	}

	// Hardware.MemorySize and CpuInfo
	hardware.MemorySize = p.MemoryBytes
	hardware.CpuInfo.NumCpuPackages = p.NumCPUPkgs
	hardware.CpuInfo.NumCpuCores = p.NumCPUCores
	hardware.CpuInfo.NumCpuThreads = p.NumCPUThreads
	hardware.CpuInfo.Hz = p.CPUHz
}

// ──────────────────────────────────────────────────────────────────────────────
// Dell Profiles
// ──────────────────────────────────────────────────────────────────────────────

var DellR760_Xeon6430 = HardwareProfile{
	Vendor:         "Dell Inc.",
	Model:          "PowerEdge R760",
	CPUModel:       "Intel(R) Xeon(R) Gold 6430 CPU @ 2.10GHz",
	CPUVendor:      "intel",
	CPUDescription: "Intel(R) Xeon(R) Gold 6430 CPU @ 2.10GHz",
	CPUHz:          2100000000,
	CPUMhz:         2100,
	NumCPUPkgs:     2,
	NumCPUCores:    64,
	NumCPUThreads:  128,
	MemoryBytes:    549755813888, // 512 GB
	BIOSVendor:     "Dell Inc.",
	BIOSVersion:    "1.6.2",
	NumNics:        4,
	NumHBAs:        3,
}

var DellR750_Xeon6338 = HardwareProfile{
	Vendor:         "Dell Inc.",
	Model:          "PowerEdge R750",
	CPUModel:       "Intel(R) Xeon(R) Gold 6338 CPU @ 2.00GHz",
	CPUVendor:      "intel",
	CPUDescription: "Intel(R) Xeon(R) Gold 6338 CPU @ 2.00GHz",
	CPUHz:          2000000000,
	CPUMhz:         2000,
	NumCPUPkgs:     2,
	NumCPUCores:    64,
	NumCPUThreads:  128,
	MemoryBytes:    549755813888, // 512 GB
	BIOSVendor:     "Dell Inc.",
	BIOSVersion:    "1.8.1",
	NumNics:        4,
	NumHBAs:        3,
}

var DellR660_Xeon5418Y = HardwareProfile{
	Vendor:         "Dell Inc.",
	Model:          "PowerEdge R660",
	CPUModel:       "Intel(R) Xeon(R) Gold 5418Y CPU @ 2.00GHz",
	CPUVendor:      "intel",
	CPUDescription: "Intel(R) Xeon(R) Gold 5418Y CPU @ 2.00GHz",
	CPUHz:          2000000000,
	CPUMhz:         2000,
	NumCPUPkgs:     2,
	NumCPUCores:    48,
	NumCPUThreads:  96,
	MemoryBytes:    274877906944, // 256 GB
	BIOSVendor:     "Dell Inc.",
	BIOSVersion:    "2.1.3",
	NumNics:        2,
	NumHBAs:        2,
}

var DellR740xd_Xeon6248 = HardwareProfile{
	Vendor:         "Dell Inc.",
	Model:          "PowerEdge R740xd",
	CPUModel:       "Intel(R) Xeon(R) Gold 6248 CPU @ 2.50GHz",
	CPUVendor:      "intel",
	CPUDescription: "Intel(R) Xeon(R) Gold 6248 CPU @ 2.50GHz",
	CPUHz:          2500000000,
	CPUMhz:         2500,
	NumCPUPkgs:     2,
	NumCPUCores:    40,
	NumCPUThreads:  80,
	MemoryBytes:    412316860416, // 384 GB
	BIOSVendor:     "Dell Inc.",
	BIOSVersion:    "2.14.1",
	NumNics:        4,
	NumHBAs:        4,
}

// ──────────────────────────────────────────────────────────────────────────────
// HPE Profiles
// ──────────────────────────────────────────────────────────────────────────────

var HPE_DL380_Gen10Plus_Xeon6342 = HardwareProfile{
	Vendor:         "HPE",
	Model:          "ProLiant DL380 Gen10 Plus",
	CPUModel:       "Intel(R) Xeon(R) Gold 6342 CPU @ 2.80GHz",
	CPUVendor:      "intel",
	CPUDescription: "Intel(R) Xeon(R) Gold 6342 CPU @ 2.80GHz",
	CPUHz:          2800000000,
	CPUMhz:         2800,
	NumCPUPkgs:     2,
	NumCPUCores:    48,
	NumCPUThreads:  96,
	MemoryBytes:    549755813888, // 512 GB
	BIOSVendor:     "HPE",
	BIOSVersion:    "U30 v2.70 (04/18/2024)",
	NumNics:        4,
	NumHBAs:        3,
}

var HPE_DL360_Gen11_Xeon5415 = HardwareProfile{
	Vendor:         "HPE",
	Model:          "ProLiant DL360 Gen11",
	CPUModel:       "Intel(R) Xeon(R) Gold 5415+ CPU @ 2.90GHz",
	CPUVendor:      "intel",
	CPUDescription: "Intel(R) Xeon(R) Gold 5415+ CPU @ 2.90GHz",
	CPUHz:          2900000000,
	CPUMhz:         2900,
	NumCPUPkgs:     2,
	NumCPUCores:    16,
	NumCPUThreads:  32,
	MemoryBytes:    274877906944, // 256 GB
	BIOSVendor:     "HPE",
	BIOSVersion:    "U46 v1.56 (02/12/2025)",
	NumNics:        2,
	NumHBAs:        2,
}

var HPE_DL380_Gen11_Xeon6448Y = HardwareProfile{
	Vendor:         "HPE",
	Model:          "ProLiant DL380a Gen11",
	CPUModel:       "Intel(R) Xeon(R) Gold 6448Y CPU @ 2.10GHz",
	CPUVendor:      "intel",
	CPUDescription: "Intel(R) Xeon(R) Gold 6448Y CPU @ 2.10GHz",
	CPUHz:          2100000000,
	CPUMhz:         2100,
	NumCPUPkgs:     2,
	NumCPUCores:    64,
	NumCPUThreads:  128,
	MemoryBytes:    1099511627776, // 1 TB
	BIOSVendor:     "HPE",
	BIOSVersion:    "U46 v1.62 (06/10/2025)",
	NumNics:        4,
	NumHBAs:        4,
}

// ──────────────────────────────────────────────────────────────────────────────
// Cisco Profiles
// ──────────────────────────────────────────────────────────────────────────────

var Cisco_UCSC_C240_M7_Xeon8462Y = HardwareProfile{
	Vendor:         "Cisco Systems Inc",
	Model:          "UCSC-C240-M7S",
	CPUModel:       "Intel(R) Xeon(R) Platinum 8462Y+ CPU @ 2.80GHz",
	CPUVendor:      "intel",
	CPUDescription: "Intel(R) Xeon(R) Platinum 8462Y+ CPU @ 2.80GHz",
	CPUHz:          2800000000,
	CPUMhz:         2800,
	NumCPUPkgs:     2,
	NumCPUCores:    64,
	NumCPUThreads:  128,
	MemoryBytes:    549755813888, // 512 GB
	BIOSVendor:     "Cisco Systems Inc",
	BIOSVersion:    "C240M7.5.2.0.230050",
	NumNics:        4,
	NumHBAs:        3,
}

var Cisco_UCSX_210C_M7_Xeon6430 = HardwareProfile{
	Vendor:         "Cisco Systems Inc",
	Model:          "UCSX-210C-M7",
	CPUModel:       "Intel(R) Xeon(R) Gold 6430 CPU @ 2.10GHz",
	CPUVendor:      "intel",
	CPUDescription: "Intel(R) Xeon(R) Gold 6430 CPU @ 2.10GHz",
	CPUHz:          2100000000,
	CPUMhz:         2100,
	NumCPUPkgs:     2,
	NumCPUCores:    64,
	NumCPUThreads:  128,
	MemoryBytes:    549755813888, // 512 GB
	BIOSVendor:     "Cisco Systems Inc",
	BIOSVersion:    "UCSX-M7.5.2.0.230045",
	NumNics:        2,
	NumHBAs:        2,
}

// ──────────────────────────────────────────────────────────────────────────────
// Lenovo Profiles
// ──────────────────────────────────────────────────────────────────────────────

var Lenovo_SR650_V3_Xeon6438Y = HardwareProfile{
	Vendor:         "Lenovo",
	Model:          "ThinkSystem SR650 V3",
	CPUModel:       "Intel(R) Xeon(R) Gold 6438Y+ CPU @ 2.00GHz",
	CPUVendor:      "intel",
	CPUDescription: "Intel(R) Xeon(R) Gold 6438Y+ CPU @ 2.00GHz",
	CPUHz:          2000000000,
	CPUMhz:         2000,
	NumCPUPkgs:     2,
	NumCPUCores:    64,
	NumCPUThreads:  128,
	MemoryBytes:    549755813888, // 512 GB
	BIOSVendor:     "Lenovo",
	BIOSVersion:    "U8E126I-2.20",
	NumNics:        4,
	NumHBAs:        3,
}

var Lenovo_SR630_V3_Xeon5415 = HardwareProfile{
	Vendor:         "Lenovo",
	Model:          "ThinkSystem SR630 V3",
	CPUModel:       "Intel(R) Xeon(R) Gold 5415+ CPU @ 2.90GHz",
	CPUVendor:      "intel",
	CPUDescription: "Intel(R) Xeon(R) Gold 5415+ CPU @ 2.90GHz",
	CPUHz:          2900000000,
	CPUMhz:         2900,
	NumCPUPkgs:     2,
	NumCPUCores:    16,
	NumCPUThreads:  32,
	MemoryBytes:    137438953472, // 128 GB
	BIOSVendor:     "Lenovo",
	BIOSVersion:    "U8E122I-1.80",
	NumNics:        2,
	NumHBAs:        2,
}

// ──────────────────────────────────────────────────────────────────────────────
// Supermicro Profiles
// ──────────────────────────────────────────────────────────────────────────────

var Supermicro_SYS_621C_EPYC9554 = HardwareProfile{
	Vendor:         "Supermicro",
	Model:          "SYS-621C-TN12R",
	CPUModel:       "AMD EPYC 9554 64-Core Processor",
	CPUVendor:      "amd",
	CPUDescription: "AMD EPYC 9554 64-Core Processor",
	CPUHz:          3100000000,
	CPUMhz:         3100,
	NumCPUPkgs:     2,
	NumCPUCores:    128,
	NumCPUThreads:  256,
	MemoryBytes:    1099511627776, // 1 TB
	BIOSVendor:     "American Megatrends International, LLC.",
	BIOSVersion:    "1.5",
	NumNics:        4,
	NumHBAs:        4,
}

var Supermicro_SYS_121H_EPYC9354 = HardwareProfile{
	Vendor:         "Supermicro",
	Model:          "SYS-121H-TNR",
	CPUModel:       "AMD EPYC 9354 32-Core Processor",
	CPUVendor:      "amd",
	CPUDescription: "AMD EPYC 9354 32-Core Processor",
	CPUHz:          3250000000,
	CPUMhz:         3250,
	NumCPUPkgs:     2,
	NumCPUCores:    64,
	NumCPUThreads:  128,
	MemoryBytes:    549755813888, // 512 GB
	BIOSVendor:     "American Megatrends International, LLC.",
	BIOSVersion:    "1.3a",
	NumNics:        2,
	NumHBAs:        2,
}

// ──────────────────────────────────────────────────────────────────────────────
// Datacenter → Cluster → Profile Mapping
// ──────────────────────────────────────────────────────────────────────────────
//
// Real-world enterprises standardise on one or two vendors per site.
// We simulate that pattern here:
//
//   DC-US-East      → Dell (production), HPE (big data)
//   DC-US-West      → HPE (production), Cisco (edge/DR)
//   DC-EU-Frankfurt → Cisco (production), Lenovo (compliance)
//   DC-APAC-Singapore → Lenovo (production), Supermicro (dev)
//   DC-Dev-Lab      → Supermicro (dev/staging/sandbox)
//
// Within a cluster, hosts alternate between two profiles so the
// inventory isn't perfectly uniform (mirrors real procurement batches).

// ClusterProfiles maps cluster names to a pair of hardware profiles.
// Hosts with odd index get profile[0], even index get profile[1].
var ClusterProfiles = map[string][2]*HardwareProfile{
	// DC-US-East — Dell primary
	"Cluster-WebTier": {&DellR760_Xeon6430, &DellR750_Xeon6338},
	"Cluster-AppTier": {&DellR760_Xeon6430, &DellR660_Xeon5418Y},
	"Cluster-DBTier":  {&DellR740xd_Xeon6248, &DellR750_Xeon6338},
	"Cluster-BigData": {&HPE_DL380_Gen11_Xeon6448Y, &HPE_DL380_Gen10Plus_Xeon6342},

	// DC-US-West — HPE primary
	"Cluster-Prod-West": {&HPE_DL380_Gen10Plus_Xeon6342, &HPE_DL380_Gen11_Xeon6448Y},
	"Cluster-DR":        {&HPE_DL380_Gen10Plus_Xeon6342, &Cisco_UCSC_C240_M7_Xeon8462Y},
	"Cluster-Edge":      {&Cisco_UCSX_210C_M7_Xeon6430, &Cisco_UCSC_C240_M7_Xeon8462Y},

	// DC-EU-Frankfurt — Cisco primary
	"Cluster-EU-Prod":       {&Cisco_UCSC_C240_M7_Xeon8462Y, &Cisco_UCSX_210C_M7_Xeon6430},
	"Cluster-EU-Compliance": {&Lenovo_SR650_V3_Xeon6438Y, &Lenovo_SR630_V3_Xeon5415},

	// DC-APAC-Singapore — Lenovo primary
	"Cluster-APAC-Prod": {&Lenovo_SR650_V3_Xeon6438Y, &Lenovo_SR630_V3_Xeon5415},
	"Cluster-APAC-Dev":  {&Supermicro_SYS_121H_EPYC9354, &Supermicro_SYS_621C_EPYC9554},

	// DC-Dev-Lab — Supermicro / mixed
	"Cluster-Dev":     {&Supermicro_SYS_621C_EPYC9554, &Supermicro_SYS_121H_EPYC9354},
	"Cluster-Staging": {&DellR660_Xeon5418Y, &HPE_DL360_Gen11_Xeon5415},
	"Cluster-Sandbox": {&Supermicro_SYS_121H_EPYC9354, &Lenovo_SR630_V3_Xeon5415},
}

// ProfileForHost returns the hardware profile for a host based on its
// cluster name and position (1-based index) within the cluster.
func ProfileForHost(clusterName string, hostIndex int) *HardwareProfile {
	profiles, ok := ClusterProfiles[clusterName]
	if !ok {
		// Fallback — use Dell R750 as a sensible default
		return &DellR750_Xeon6338
	}
	if hostIndex%2 == 0 {
		return profiles[1]
	}
	return profiles[0]
}
