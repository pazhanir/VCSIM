// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: Apache-2.0

package esx

import (
	"time"

	"github.com/vmware/govmomi/vim25/types"
)

// HostHardwareInfo is the default template for the HostSystem hardware property.
// Updated for ESXi 8.0.2 — UEFI firmware, SGX info, persistent memory,
// memory tiering, and modern CPU/BIOS fields.
//
// The HostCustomizationFunc hook in host_system.go overrides vendor/model/CPU
// per-host; this template provides the structural baseline.
//
// Capture method:
//
//	govc object.collect -s -dump HostSystem:ha-host hardware
var HostHardwareInfo = &types.HostHardwareInfo{
	SystemInfo: types.HostSystemInfo{
		Vendor: "Dell Inc.",
		Model:  "PowerEdge R760",
		Uuid:   "e88d4d56-9f1e-3ea1-71fa-13a8e1a7fd70",
		OtherIdentifyingInfo: []types.HostSystemIdentificationInfo{
			{
				IdentifierValue: "ASSET-R760-001",
				IdentifierType: &types.ElementDescription{
					Description: types.Description{
						Label:   "Asset Tag",
						Summary: "Asset tag of the system",
					},
					Key: "AssetTag",
				},
			},
			{
				IdentifierValue: "Dell System",
				IdentifierType: &types.ElementDescription{
					Description: types.Description{
						Label:   "OEM specific string",
						Summary: "OEM specific string",
					},
					Key: "OemSpecificString",
				},
			},
			{
				IdentifierValue: "www.dell.com",
				IdentifierType: &types.ElementDescription{
					Description: types.Description{
						Label:   "OEM specific string",
						Summary: "OEM specific string",
					},
					Key: "OemSpecificString",
				},
			},
			{
				IdentifierValue: "Dell-56 4d 8d e8 1e 9f a1 3e-71 fa 13 a8 e1 a7 fd 70",
				IdentifierType: &types.ElementDescription{
					Description: types.Description{
						Label:   "Service tag",
						Summary: "Service tag of the system",
					},
					Key: "ServiceTag",
				},
			},
			{
				IdentifierValue: "0",
				IdentifierType: &types.ElementDescription{
					Description: types.Description{
						Label:   "Enclosure serial number tag",
						Summary: "Enclosure serial number tag of the system",
					},
					Key: "EnclosureSerialNumberTag",
				},
			},
		},
	},
	CpuPowerManagementInfo: &types.HostCpuPowerManagementInfo{
		CurrentPolicy:   "Balanced",
		HardwareSupport: "acpi-cppc",
	},
	CpuInfo: types.HostCpuInfo{
		NumCpuPackages: 2,
		NumCpuCores:    64,
		NumCpuThreads:  128,
		Hz:             2100000000,
	},
	CpuPkg: []types.HostCpuPackage{
		{
			Index:       0,
			Vendor:      "intel",
			Hz:          2100000000,
			BusHz:       100000000,
			Description: "Intel(R) Xeon(R) Gold 6430 CPU @ 2.10GHz",
			ThreadId:    []int16{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31},
			CpuFeature: []types.HostCpuIdInfo{
				{
					Level:  0,
					Vendor: "",
					Eax:    "0000:0000:0000:0000:0000:0000:0010:0000",
					Ebx:    "0111:0101:0110:1110:0110:0101:0100:0111",
					Ecx:    "0110:1100:0110:0101:0111:0100:0110:1110",
					Edx:    "0100:1001:0110:0101:0110:1110:0110:1001",
				},
				{
					Level:  1,
					Vendor: "",
					Eax:    "0000:0000:0000:1000:0000:0110:1111:0111",
					Ebx:    "0000:0000:0000:0001:0000:1000:0000:0000",
					Ecx:    "1111:1111:1111:1010:0011:0010:0010:1011",
					Edx:    "0000:1111:1010:1011:1111:1011:1111:1111",
				},
				{
					Level:  -2147483648,
					Vendor: "",
					Eax:    "1000:0000:0000:0000:0000:0000:0000:1000",
					Ebx:    "0000:0000:0000:0000:0000:0000:0000:0000",
					Ecx:    "0000:0000:0000:0000:0000:0000:0000:0000",
					Edx:    "0000:0000:0000:0000:0000:0000:0000:0000",
				},
				{
					Level:  -2147483647,
					Vendor: "",
					Eax:    "0000:0000:0000:0000:0000:0000:0000:0000",
					Ebx:    "0000:0000:0000:0000:0000:0000:0000:0000",
					Ecx:    "0000:0000:0000:0000:0000:0000:0000:0001",
					Edx:    "0010:1100:0001:0000:0000:1000:0000:0000",
				},
			},
		},
		{
			Index:       1,
			Vendor:      "intel",
			Hz:          2100000000,
			BusHz:       100000000,
			Description: "Intel(R) Xeon(R) Gold 6430 CPU @ 2.10GHz",
			ThreadId:    []int16{32, 33, 34, 35, 36, 37, 38, 39, 40, 41, 42, 43, 44, 45, 46, 47, 48, 49, 50, 51, 52, 53, 54, 55, 56, 57, 58, 59, 60, 61, 62, 63},
			CpuFeature: []types.HostCpuIdInfo{
				{
					Level:  0,
					Vendor: "",
					Eax:    "0000:0000:0000:0000:0000:0000:0010:0000",
					Ebx:    "0111:0101:0110:1110:0110:0101:0100:0111",
					Ecx:    "0110:1100:0110:0101:0111:0100:0110:1110",
					Edx:    "0100:1001:0110:0101:0110:1110:0110:1001",
				},
				{
					Level:  1,
					Vendor: "",
					Eax:    "0000:0000:0000:1000:0000:0110:1111:0111",
					Ebx:    "0000:0010:0000:0001:0000:1000:0000:0000",
					Ecx:    "1111:1111:1111:1010:0011:0010:0010:1011",
					Edx:    "0000:1111:1010:1011:1111:1011:1111:1111",
				},
				{
					Level:  -2147483648,
					Vendor: "",
					Eax:    "1000:0000:0000:0000:0000:0000:0000:1000",
					Ebx:    "0000:0000:0000:0000:0000:0000:0000:0000",
					Ecx:    "0000:0000:0000:0000:0000:0000:0000:0000",
					Edx:    "0000:0000:0000:0000:0000:0000:0000:0000",
				},
				{
					Level:  -2147483647,
					Vendor: "",
					Eax:    "0000:0000:0000:0000:0000:0000:0000:0000",
					Ebx:    "0000:0000:0000:0000:0000:0000:0000:0000",
					Ecx:    "0000:0000:0000:0000:0000:0000:0000:0001",
					Edx:    "0010:1100:0001:0000:0000:1000:0000:0000",
				},
			},
		},
	},
	MemorySize: 549755813888, // 512 GB
	NumaInfo: &types.HostNumaInfo{
		Type:     "NUMA",
		NumNodes: 2,
		NumaNode: []types.HostNumaNode{
			{
				TypeId:            0x0,
				CpuID:             []int16{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31},
				MemoryRangeBegin:  0,
				MemoryRangeLength: 274877906944,
			},
			{
				TypeId:            0x1,
				CpuID:             []int16{32, 33, 34, 35, 36, 37, 38, 39, 40, 41, 42, 43, 44, 45, 46, 47, 48, 49, 50, 51, 52, 53, 54, 55, 56, 57, 58, 59, 60, 61, 62, 63},
				MemoryRangeBegin:  274877906944,
				MemoryRangeLength: 274877906944,
			},
		},
	},
	SmcPresent: types.NewBool(false),
	PciDevice: []types.HostPciDevice{
		{
			Id:           "0000:00:00.0",
			ClassId:      1536,
			Bus:          0x0,
			Slot:         0x0,
			Function:     0x0,
			VendorId:     -32634,
			SubVendorId:  5549,
			VendorName:   "Intel Corporation",
			DeviceId:     29072,
			SubDeviceId:  6518,
			ParentBridge: "",
			DeviceName:   "Sapphire Rapids-SP LPC Controller/eSPI Controller",
		},
		{
			Id:           "0000:00:01.0",
			ClassId:      1540,
			Bus:          0x0,
			Slot:         0x1,
			Function:     0x0,
			VendorId:     -32634,
			SubVendorId:  0,
			VendorName:   "Intel Corporation",
			DeviceId:     29073,
			SubDeviceId:  0,
			ParentBridge: "",
			DeviceName:   "Sapphire Rapids-SP PCI Express Root Port",
		},
		{
			Id:           "0000:00:07.0",
			ClassId:      1537,
			Bus:          0x0,
			Slot:         0x7,
			Function:     0x0,
			VendorId:     -32634,
			SubVendorId:  5549,
			VendorName:   "Intel Corporation",
			DeviceId:     28944,
			SubDeviceId:  6518,
			ParentBridge: "",
			DeviceName:   "Sapphire Rapids-SP ISA Bridge",
		},
		{
			Id:           "0000:00:07.1",
			ClassId:      257,
			Bus:          0x0,
			Slot:         0x7,
			Function:     0x1,
			VendorId:     -32634,
			SubVendorId:  5549,
			VendorName:   "Intel Corporation",
			DeviceId:     28945,
			SubDeviceId:  6518,
			ParentBridge: "",
			DeviceName:   "Sapphire Rapids-SP SATA Controller [AHCI mode]",
		},
		{
			Id:           "0000:03:00.0",
			ClassId:      263,
			Bus:          0x3,
			Slot:         0x0,
			Function:     0x0,
			VendorId:     4096, // Broadcom / LSI
			SubVendorId:  4096,
			VendorName:   "Broadcom / LSI",
			DeviceId:     173,
			SubDeviceId:  16384,
			ParentBridge: "0000:00:01.0",
			DeviceName:   "MegaRAID Tri-Mode SAS3916",
		},
		{
			Id:           "0000:04:00.0",
			ClassId:      512,
			Bus:          0x4,
			Slot:         0x0,
			Function:     0x0,
			VendorId:     -32634, // Intel
			SubVendorId:  -32634,
			VendorName:   "Intel Corporation",
			DeviceId:     4735,
			SubDeviceId:  33,
			ParentBridge: "0000:00:01.0",
			DeviceName:   "Ethernet Controller E810-XXV for SFP",
		},
		{
			Id:           "0000:04:00.1",
			ClassId:      512,
			Bus:          0x4,
			Slot:         0x0,
			Function:     0x1,
			VendorId:     -32634,
			SubVendorId:  -32634,
			VendorName:   "Intel Corporation",
			DeviceId:     4735,
			SubDeviceId:  33,
			ParentBridge: "0000:00:01.0",
			DeviceName:   "Ethernet Controller E810-XXV for SFP",
		},
		{
			Id:           "0000:0b:00.0",
			ClassId:      512,
			Bus:          0xb,
			Slot:         0x0,
			Function:     0x0,
			VendorId:     -32634,
			SubVendorId:  -32634,
			VendorName:   "Intel Corporation",
			DeviceId:     4735,
			SubDeviceId:  33,
			ParentBridge: "0000:00:01.0",
			DeviceName:   "Ethernet Controller E810-XXV for SFP",
		},
		{
			Id:           "0000:0b:00.1",
			ClassId:      512,
			Bus:          0xb,
			Slot:         0x0,
			Function:     0x1,
			VendorId:     -32634,
			SubVendorId:  -32634,
			VendorName:   "Intel Corporation",
			DeviceId:     4735,
			SubDeviceId:  33,
			ParentBridge: "0000:00:01.0",
			DeviceName:   "Ethernet Controller E810-XXV for SFP",
		},
	},
	CpuFeature: []types.HostCpuIdInfo{
		{
			Level:  0,
			Vendor: "",
			Eax:    "0000:0000:0000:0000:0000:0000:0010:0000",
			Ebx:    "0111:0101:0110:1110:0110:0101:0100:0111",
			Ecx:    "0110:1100:0110:0101:0111:0100:0110:1110",
			Edx:    "0100:1001:0110:0101:0110:1110:0110:1001",
		},
		{
			Level:  1,
			Vendor: "",
			Eax:    "0000:0000:0000:1000:0000:0110:1111:0111",
			Ebx:    "0000:0000:0000:0001:0000:1000:0000:0000",
			Ecx:    "1111:1111:1111:1010:0011:0010:0010:1011",
			Edx:    "0000:1111:1010:1011:1111:1011:1111:1111",
		},
		{
			Level:  -2147483648,
			Vendor: "",
			Eax:    "1000:0000:0000:0000:0000:0000:0000:1000",
			Ebx:    "0000:0000:0000:0000:0000:0000:0000:0000",
			Ecx:    "0000:0000:0000:0000:0000:0000:0000:0000",
			Edx:    "0000:0000:0000:0000:0000:0000:0000:0000",
		},
		{
			Level:  -2147483647,
			Vendor: "",
			Eax:    "0000:0000:0000:0000:0000:0000:0000:0000",
			Ebx:    "0000:0000:0000:0000:0000:0000:0000:0000",
			Ecx:    "0000:0000:0000:0000:0000:0000:0000:0001",
			Edx:    "0010:1100:0001:0000:0000:1000:0000:0000",
		},
	},
	BiosInfo: &types.HostBIOSInfo{
		BiosVersion:          "1.6.2",
		ReleaseDate:          nil, // set in init() below
		Vendor:               "Dell Inc.",
		MajorRelease:         1,
		MinorRelease:         6,
		FirmwareMajorRelease: 1,
		FirmwareMinorRelease: 26,
		FirmwareType:         "UEFI",
	},
	ReliableMemoryInfo: &types.HostReliableMemoryInfo{},
	PersistentMemoryInfo: &types.HostPersistentMemoryInfo{
		CapacityInMB: 0, // No pmem by default; scenarios can inject
		VolumeUUID:   "",
	},
	SgxInfo: &types.HostSgxInfo{
		SgxState:       "notPresent",
		TotalEpcMemory: 0,
		FlcMode:        "unlocked",
		LePubKeyHash:   "",
	},
	MemoryTieringType: "noTiering",
}

func init() {
	date, _ := time.Parse("2006-01-02", "2024-03-15")
	HostHardwareInfo.BiosInfo.ReleaseDate = &date
}
