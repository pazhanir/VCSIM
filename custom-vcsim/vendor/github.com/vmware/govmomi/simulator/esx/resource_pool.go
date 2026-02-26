// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: Apache-2.0

package esx

import (
	"time"

	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// ResourcePool is the default template for ResourcePool properties.
// Capture method:
//
//	govc pool.info "*" -dump
var ResourcePool = mo.ResourcePool{
	ManagedEntity: mo.ManagedEntity{
		ExtensibleManagedObject: mo.ExtensibleManagedObject{
			Self:           types.ManagedObjectReference{Type: "ResourcePool", Value: "ha-root-pool"},
			Value:          nil,
			AvailableField: nil,
		},
		Parent:              &types.ManagedObjectReference{Type: "ComputeResource", Value: "ha-compute-res"},
		CustomValue:         nil,
		OverallStatus:       "green",
		ConfigStatus:        "green",
		ConfigIssue:         nil,
		EffectiveRole:       []int32{-1},
		Permission:          nil,
		Name:                "Resources",
		DisabledMethod:      []string{"CreateVApp", "CreateChildVM_Task"},
		RecentTask:          nil,
		DeclaredAlarmState:  nil,
		TriggeredAlarmState: nil,
		AlarmActionsEnabled: (*bool)(nil),
		Tag:                 nil,
	},
	Summary: &types.ResourcePoolSummary{
		DynamicData: types.DynamicData{},
		Name:        "Resources",
		Config: types.ResourceConfigSpec{
			DynamicData:   types.DynamicData{},
			Entity:        &types.ManagedObjectReference{Type: "ResourcePool", Value: "ha-root-pool"},
			ChangeVersion: "",
			LastModified:  (*time.Time)(nil),
			CpuAllocation: types.ResourceAllocationInfo{
				DynamicData:           types.DynamicData{},
				Reservation:           types.NewInt64(4121),
				ExpandableReservation: types.NewBool(false),
				Limit:                 types.NewInt64(4121),
				Shares: &types.SharesInfo{
					DynamicData: types.DynamicData{},
					Shares:      9000,
					Level:       "custom",
				},
				OverheadLimit: nil,
			},
			MemoryAllocation: types.ResourceAllocationInfo{
				DynamicData:           types.DynamicData{},
				Reservation:           types.NewInt64(961),
				ExpandableReservation: types.NewBool(false),
				Limit:                 types.NewInt64(961),
				Shares: &types.SharesInfo{
					DynamicData: types.DynamicData{},
					Shares:      9000,
					Level:       "custom",
				},
				OverheadLimit: nil,
			},
		},
		Runtime: types.ResourcePoolRuntimeInfo{
			DynamicData: types.DynamicData{},
			Memory: types.ResourcePoolResourceUsage{
				DynamicData:          types.DynamicData{},
				ReservationUsed:      0,
				ReservationUsedForVm: 0,
				UnreservedForPool:    1007681536,
				UnreservedForVm:      1007681536,
				OverallUsage:         0,
				MaxUsage:             1007681536,
			},
			Cpu: types.ResourcePoolResourceUsage{
				DynamicData:          types.DynamicData{},
				ReservationUsed:      0,
				ReservationUsedForVm: 0,
				UnreservedForPool:    4121,
				UnreservedForVm:      4121,
				OverallUsage:         0,
				MaxUsage:             4121,
			},
			OverallStatus: "green",
		},
		QuickStats: &types.ResourcePoolQuickStats{
			OverallCpuUsage:              2850, // MHz — ~69% of 4121 MHz limit
			OverallCpuDemand:             3100, // MHz — slightly above usage (some contention)
			GuestMemoryUsage:             620,  // MB — active guest memory
			HostMemoryUsage:              710,  // MB — consumed host memory (includes overhead)
			DistributedCpuEntitlement:    3200, // MHz — DRS entitlement
			DistributedMemoryEntitlement: 750,  // MB — DRS memory entitlement
			StaticCpuEntitlement:         4121, // MHz — worst-case CPU (matches limit)
			StaticMemoryEntitlement:      961,  // MB — worst-case memory (matches limit)
			PrivateMemory:                580,  // MB — non-shared host memory granted
			SharedMemory:                 130,  // MB — shared host memory granted
			SwappedMemory:                0,    // MB — no swap pressure
			BalloonedMemory:              0,    // MB — no balloon pressure
			OverheadMemory:               90,   // MB — VM overhead reservation
			ConsumedOverheadMemory:       64,   // MB — current overhead consumption
			CompressedMemory:             0,    // KB — no compression
		},
		ConfiguredMemoryMB: 0,
	},
	Runtime: types.ResourcePoolRuntimeInfo{
		DynamicData: types.DynamicData{},
		Memory: types.ResourcePoolResourceUsage{
			DynamicData:          types.DynamicData{},
			ReservationUsed:      0,
			ReservationUsedForVm: 0,
			UnreservedForPool:    1007681536,
			UnreservedForVm:      1007681536,
			OverallUsage:         0,
			MaxUsage:             1007681536,
		},
		Cpu: types.ResourcePoolResourceUsage{
			DynamicData:          types.DynamicData{},
			ReservationUsed:      0,
			ReservationUsedForVm: 0,
			UnreservedForPool:    4121,
			UnreservedForVm:      4121,
			OverallUsage:         0,
			MaxUsage:             4121,
		},
		OverallStatus: "green",
	},
	Owner:        types.ManagedObjectReference{Type: "ComputeResource", Value: "ha-compute-res"},
	ResourcePool: nil,
	Vm:           nil,
	Config: types.ResourceConfigSpec{
		DynamicData:   types.DynamicData{},
		Entity:        &types.ManagedObjectReference{Type: "ResourcePool", Value: "ha-root-pool"},
		ChangeVersion: "",
		LastModified:  (*time.Time)(nil),
		CpuAllocation: types.ResourceAllocationInfo{
			DynamicData:           types.DynamicData{},
			Reservation:           types.NewInt64(4121),
			ExpandableReservation: types.NewBool(false),
			Limit:                 types.NewInt64(4121),
			Shares: &types.SharesInfo{
				DynamicData: types.DynamicData{},
				Shares:      9000,
				Level:       "custom",
			},
			OverheadLimit: nil,
		},
		MemoryAllocation: types.ResourceAllocationInfo{
			DynamicData:           types.DynamicData{},
			Reservation:           types.NewInt64(961),
			ExpandableReservation: types.NewBool(false),
			Limit:                 types.NewInt64(961),
			Shares: &types.SharesInfo{
				DynamicData: types.DynamicData{},
				Shares:      9000,
				Level:       "custom",
			},
			OverheadLimit: nil,
		},
	},
	ChildConfiguration: nil,
}
