// Inspector: connects to the running vcsim and dumps the topology-relevant
// properties the Site24x7 poller relies on (parent links, cluster.host arrays,
// vm.runtime.host, host hardware UUIDs). Read-only diagnostic.
package main

import (
	"context"
	
	"fmt"
	"net/url"
	"os"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

func main() {
	addr := os.Getenv("VC_URL")
	if addr == "" {
		addr = "https://administrator@vsphere.local:Site24x7!Demo@100.75.23.73:443/sdk"
	}
	ctx := context.Background()
	u, err := url.Parse(addr)
	if err != nil {
		panic(err)
	}
	c, err := govmomi.NewClient(ctx, u, true)
	if err != nil {
		panic(err)
	}
	defer c.Logout(ctx)

	m := view.NewManager(c.Client)

	dump := func(kind string, props []string, fn func(mo.Reference)) {
		v, err := m.CreateContainerView(ctx, c.ServiceContent.RootFolder, []string{kind}, true)
		if err != nil {
			fmt.Printf("ERR view %s: %v\n", kind, err)
			return
		}
		defer v.Destroy(ctx)
		var refs []types.ManagedObjectReference
		// generic retrieve
		pc := property.DefaultCollector(c.Client)
		_ = pc
		switch kind {
		case "Datacenter":
			var l []mo.Datacenter
			v.Retrieve(ctx, []string{kind}, props, &l)
			for i := range l {
				fn(&l[i])
			}
		case "ClusterComputeResource":
			var l []mo.ClusterComputeResource
			v.Retrieve(ctx, []string{kind}, props, &l)
			for i := range l {
				fn(&l[i])
			}
		case "HostSystem":
			var l []mo.HostSystem
			v.Retrieve(ctx, []string{kind}, props, &l)
			for i := range l {
				fn(&l[i])
			}
		case "VirtualMachine":
			var l []mo.VirtualMachine
			v.Retrieve(ctx, []string{kind}, props, &l)
			for i := range l {
				fn(&l[i])
			}
		}
		_ = refs
	}

	fmt.Println("================ DATACENTERS ================")
	dump("Datacenter", []string{"name", "parent", "hostFolder", "vmFolder"}, func(r mo.Reference) {
		d := r.(*mo.Datacenter)
		fmt.Printf("DC  %-12s mor=%s parent=%v hostFolder=%v vmFolder=%v\n",
			d.Name, d.Self.Value, refStr(d.Parent), d.HostFolder.Value, d.VmFolder.Value)
	})

	fmt.Println("\n================ CLUSTERS ================")
	dump("ClusterComputeResource", []string{"name", "parent", "host", "configuration", "summary"}, func(r mo.Reference) {
		cl := r.(*mo.ClusterComputeResource)
		fmt.Printf("CL  %-18s mor=%s parent=%v  host[]=%d hosts %v\n",
			cl.Name, cl.Self.Value, refStr(cl.Parent), len(cl.Host), morVals(cl.Host))
		if cl.Configuration.DrsConfig.DefaultVmBehavior == "" {
			fmt.Printf("      !! DrsConfig.DefaultVmBehavior EMPTY\n")
		} else {
			fmt.Printf("      DrsConfig: enabled=%v behavior=%s vmotionRate=%d\n",
				bp(cl.Configuration.DrsConfig.Enabled), cl.Configuration.DrsConfig.DefaultVmBehavior, cl.Configuration.DrsConfig.VmotionRate)
		}
	})

	fmt.Println("\n================ HOSTS ================")
	dump("HostSystem", []string{"name", "parent", "vm", "summary", "hardware", "runtime"}, func(r mo.Reference) {
		h := r.(*mo.HostSystem)
		uuid := ""
		if h.Summary.Hardware != nil {
			uuid = h.Summary.Hardware.Uuid
		}
		sysUuid := ""
		if h.Hardware != nil {
			sysUuid = h.Hardware.SystemInfo.Uuid
		}
		fmt.Printf("HOST %-14s mor=%s parent=%v(%s) vm[]=%d  summaryUuid=%s sysUuid=%s power=%s conn=%s\n",
			h.Name, h.Self.Value, refStr(h.Parent), parentType(h.Parent), len(h.Vm),
			uuid, sysUuid, h.Runtime.PowerState, h.Runtime.ConnectionState)
	})

	fmt.Println("\n================ VMs (first 8) ================")
	n := 0
	dump("VirtualMachine", []string{"name", "parent", "runtime", "summary", "resourcePool"}, func(r mo.Reference) {
		if n >= 8 {
			return
		}
		n++
		vm := r.(*mo.VirtualMachine)
		host := ""
		if vm.Runtime.Host != nil {
			host = vm.Runtime.Host.Value
		}
		rp := ""
		if vm.ResourcePool != nil {
			rp = vm.ResourcePool.Value
		}
		fmt.Printf("VM  %-22s mor=%s parent=%v runtime.host=%s resourcePool=%s cfgUuid=%s\n",
			vm.Name, vm.Self.Value, refStr(vm.Parent), host, rp, vm.Summary.Config.Uuid)
	})
}

func refStr(r *types.ManagedObjectReference) string {
	if r == nil {
		return "<nil>"
	}
	return r.Value
}
func parentType(r *types.ManagedObjectReference) string {
	if r == nil {
		return "nil"
	}
	return r.Type
}
func morVals(rs []types.ManagedObjectReference) []string {
	out := make([]string, 0, len(rs))
	for _, r := range rs {
		out = append(out, r.Value)
	}
	return out
}
func bp(b *bool) bool {
	if b == nil {
		return false
	}
	return *b
}
