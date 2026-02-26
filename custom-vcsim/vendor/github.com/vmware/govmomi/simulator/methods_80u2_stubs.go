// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: Apache-2.0

package simulator

// Method stubs for vSphere 8.0.0.1, 8.0.1.0, and 8.0.2.0 new API methods.
// These return sensible defaults to prevent "method not implemented" errors
// from clients expecting vSphere 8.0.2 API completeness.

import (
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"
)

// StorageQueryManager wraps the mo.StorageQueryManager type for method dispatch.
type StorageQueryManager struct {
	mo.StorageQueryManager
}

// ---- 8.0.2.0 Methods ----

// SetServiceAccount sets the service account for an extension (no-op stub).
func (m *ExtensionManager) SetServiceAccount(ctx *Context, req *types.SetServiceAccount) soap.HasFault {
	return &methods.SetServiceAccountBody{
		Res: &types.SetServiceAccountResponse{},
	}
}

// QueryFileLockInfo returns file lock information for a datastore path (stub returns empty result).
func (m *FileManager) QueryFileLockInfo(ctx *Context, req *types.QueryFileLockInfo) soap.HasFault {
	return &methods.QueryFileLockInfoBody{
		Res: &types.QueryFileLockInfoResponse{
			Returnval: types.FileLockInfoResult{},
		},
	}
}

// ---- 8.0.1.0 Methods ----

// GetCryptoKeyStatus returns the status of crypto keys on a host (stub returns empty list).
func (m *CryptoManagerKmip) GetCryptoKeyStatus(ctx *Context, req *types.GetCryptoKeyStatus) soap.HasFault {
	return &methods.GetCryptoKeyStatusBody{
		Res: &types.GetCryptoKeyStatusResponse{},
	}
}

// SetKeyCustomAttributes sets custom attributes on a crypto key (no-op stub).
func (m *CryptoManagerKmip) SetKeyCustomAttributes(ctx *Context, req *types.SetKeyCustomAttributes) soap.HasFault {
	return &methods.SetKeyCustomAttributesBody{
		Res: &types.SetKeyCustomAttributesResponse{},
	}
}

// IncreaseDirectorySize increases the size of a namespace directory (no-op stub).
func (m *DatastoreNamespaceManager) IncreaseDirectorySize(ctx *Context, req *types.IncreaseDirectorySize) soap.HasFault {
	return &methods.IncreaseDirectorySizeBody{
		Res: &types.IncreaseDirectorySizeResponse{},
	}
}

// QueryDirectoryInfo returns information about a namespace directory (stub returns defaults).
func (m *DatastoreNamespaceManager) QueryDirectoryInfo(ctx *Context, req *types.QueryDirectoryInfo) soap.HasFault {
	return &methods.QueryDirectoryInfoBody{
		Res: &types.QueryDirectoryInfoResponse{
			Returnval: types.DatastoreNamespaceManagerDirectoryInfo{},
		},
	}
}

// RetrieveCertificateInfoList returns certificate info for a host (stub returns empty list).
func (m *HostCertificateManager) RetrieveCertificateInfoList(ctx *Context, req *types.RetrieveCertificateInfoList) soap.HasFault {
	return &methods.RetrieveCertificateInfoListBody{
		Res: &types.RetrieveCertificateInfoListResponse{},
	}
}

// ---- 8.0.0.1 Methods ----

// QueryCompatibleVmnicsFromHosts returns compatible vmnics from specified hosts (stub returns empty list).
func (m *DistributedVirtualSwitchManager) QueryCompatibleVmnicsFromHosts(ctx *Context, req *types.QueryCompatibleVmnicsFromHosts) soap.HasFault {
	return &methods.QueryCompatibleVmnicsFromHostsBody{
		Res: &types.QueryCompatibleVmnicsFromHostsResponse{},
	}
}

// QuerySupportedNetworkOffloadSpec returns supported network offload specs (stub returns empty list).
func (m *DistributedVirtualSwitchManager) QuerySupportedNetworkOffloadSpec(ctx *Context, req *types.QuerySupportedNetworkOffloadSpec) soap.HasFault {
	return &methods.QuerySupportedNetworkOffloadSpecBody{
		Res: &types.QuerySupportedNetworkOffloadSpecResponse{},
	}
}

// QueryMaxQueueDepth returns the max queue depth for a datastore (stub returns 64).
func (m *HostDatastoreSystem) QueryMaxQueueDepth(ctx *Context, req *types.QueryMaxQueueDepth) soap.HasFault {
	return &methods.QueryMaxQueueDepthBody{
		Res: &types.QueryMaxQueueDepthResponse{
			Returnval: 64,
		},
	}
}

// SetMaxQueueDepth sets the max queue depth for a datastore (no-op stub).
func (m *HostDatastoreSystem) SetMaxQueueDepth(ctx *Context, req *types.SetMaxQueueDepth) soap.HasFault {
	return &methods.SetMaxQueueDepthBody{
		Res: &types.SetMaxQueueDepthResponse{},
	}
}

// QueryHostsWithAttachedLun returns hosts with an attached LUN (stub returns empty list).
func (m *StorageQueryManager) QueryHostsWithAttachedLun(ctx *Context, req *types.QueryHostsWithAttachedLun) soap.HasFault {
	return &methods.QueryHostsWithAttachedLunBody{
		Res: &types.QueryHostsWithAttachedLunResponse{},
	}
}
