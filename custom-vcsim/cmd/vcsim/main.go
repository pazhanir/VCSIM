// Custom vcsim with large-scale inventory and failure scenario simulation.
//
// Ports:
//
//	:443  - vSphere SOAP API (standard vCenter port, HTTPS)
//	:8990 - Scenario management REST API
//
// The vSphere API on :443 is fully compatible with real vCenter.
// Site24x7 connects to :443 for monitoring.
// Operators use :8990 to trigger failure scenarios.
package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25/types"

	// Import simulator endpoint packages — each registers via init() + RegisterEndpoint()
	_ "github.com/vmware/govmomi/eam/simulator"
	_ "github.com/vmware/govmomi/lookup/simulator"
	_ "github.com/vmware/govmomi/pbm/simulator"
	_ "github.com/vmware/govmomi/ssoadmin/simulator"
	_ "github.com/vmware/govmomi/sts/simulator"
	_ "github.com/vmware/govmomi/vapi/simulator"

	"github.com/site24x7/vcsim-demo/pkg/api"
	"github.com/site24x7/vcsim-demo/pkg/inventory"
	"github.com/site24x7/vcsim-demo/pkg/overrides"
	"github.com/site24x7/vcsim-demo/pkg/scenarios"
)

func main() {
	vsphereAddr := flag.String("l", ":443", "vSphere API listen address")
	scenarioAddr := flag.String("scenario-addr", ":8990", "Scenario controller listen address")
	username := flag.String("username", "administrator@vsphere.local", "vSphere username")
	password := flag.String("password", "Site24x7!Demo", "vSphere password")
	skipInventory := flag.Bool("skip-inventory", false, "Skip custom inventory creation (use default vcsim inventory)")
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log.Println("==============================================")
	log.Println(" Custom VCsim - Site24x7 VMware Demo")
	log.Println("==============================================")
	log.Printf(" vSphere API:         https://0.0.0.0%s", *vsphereAddr)
	log.Printf(" Scenario Controller: http://0.0.0.0%s", *scenarioAddr)
	log.Printf(" Username:            %s", *username)
	log.Println("==============================================")

	// Create the VPX model (vCenter simulator)
	model := simulator.VPX()
	if !*skipInventory {
		// Start with minimal inventory — we'll build our own programmatically
		model.Datacenter = 1
		model.Cluster = 1
		model.ClusterHost = 2 // hosts inside cluster
		model.Host = 0        // standalone hosts
		model.Machine = 2
		model.Datastore = 1
		model.Portgroup = 1
		model.Autostart = false
	}

	// Wire host hardware customization hook.
	// This fires for every host during configure(), allowing us to set
	// vendor/model/CPU per host based on profiles registered by the
	// inventory builder.
	simulator.HostCustomizationFunc = func(hostname string, summary *types.HostHardwareSummary, hardware *types.HostHardwareInfo) {
		profile := inventory.LookupHostProfile(hostname)
		if profile != nil {
			profile.Apply(summary, hardware)
		}
	}
	log.Println("[hardware] Host customization hook installed for vendor/model diversity")

	err := model.Create()
	if err != nil {
		log.Fatalf("Failed to create VPX model: %v", err)
	}
	defer model.Remove()

	// Wire metric overrides into vcsim's PerformanceManager.
	// When a scenario sets metric overrides in the registry, this hook
	// ensures QueryPerf returns those values instead of default data.
	reg := overrides.Global()
	simulator.MetricOverrideFunc = func(entity types.ManagedObjectReference, counterID int32, instance string) []int64 {
		metrics := reg.GetMetrics(entity)
		for _, mo := range metrics {
			if mo.CounterID == counterID && mo.Instance == instance {
				return mo.Values
			}
		}
		return nil
	}
	log.Println("[metrics] Override hook installed for PerformanceManager.QueryPerf")

	// Configure the listen address with TLS (real vCenter uses HTTPS)
	model.Service.Listen = &url.URL{
		Scheme: "https",
		Host:   *vsphereAddr,
		User:   url.UserPassword(*username, *password),
	}
	model.Service.TLS = new(tls.Config)    // Enable HTTPS
	model.Service.RegisterEndpoints = true // Enable VAPI, PBM, Lookup, STS, SSO, EAM endpoint registration

	// Start the simulator server
	server := model.Service.NewServer()
	defer server.Close()

	log.Printf("[vcsim] vSphere API running at %s", server.URL.String())

	// Build the custom inventory if not skipped
	if !*skipInventory {
		log.Println("[inventory] Building large-scale inventory (~4000 objects)...")
		if err := buildCustomInventory(ctx, server.URL); err != nil {
			log.Printf("[inventory] WARNING: Custom inventory build failed: %v", err)
			log.Println("[inventory] Continuing with default vcsim inventory")
		} else {
			log.Println("[inventory] Inventory build complete!")
		}
	}

	// Connect a client for the scenario manager.
	// Use server.URL directly — it contains the actual address vcsim is
	// listening on (including credentials set from model.Service.Listen.User).
	// Constructing a separate 127.0.0.1 URL can cause mismatches when vcsim
	// advertises its own IP via defaultIP().
	scenarioClientURL := *server.URL // copy
	scenarioClientURL.Path = "/sdk"
	if scenarioClientURL.User == nil {
		scenarioClientURL.User = url.UserPassword(*username, *password)
	}
	log.Printf("[scenario] Connecting scenario manager to: %s", scenarioClientURL.Host)
	client, err := govmomi.NewClient(ctx, &scenarioClientURL, true)
	if err != nil {
		log.Fatalf("Failed to connect scenario manager to vcsim: %v", err)
	}

	// Create scenario manager
	mgr := scenarios.NewManager(client, reg)

	// Start scenario controller API on separate port
	apiServer := api.NewServer(mgr, *scenarioAddr)
	go func() {
		if err := apiServer.Start(); err != nil {
			log.Fatalf("[api] Scenario controller failed: %v", err)
		}
	}()

	log.Println("")
	log.Println("Ready! Waiting for connections...")
	log.Println("")
	log.Println("Quick Start:")
	log.Printf("  List scenarios:   curl http://localhost%s/api/scenarios", *scenarioAddr)
	log.Printf("  Activate:         curl -X POST http://localhost%s/api/scenario/activate \\", *scenarioAddr)
	log.Println("                      -H 'Content-Type: application/json' \\")
	log.Println("                      -d '{\"id\":\"cpu_host_saturation\",\"targets\":[\"/DC0/host/DC0_C0/DC0_C0_H0\"]}'")
	log.Printf("  Active scenarios: curl http://localhost%s/api/scenarios/active", *scenarioAddr)
	log.Printf("  Clear all:        curl -X POST http://localhost%s/api/scenario/clear-all", *scenarioAddr)
	log.Println("")

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	log.Println("Shutting down...")
}

func buildCustomInventory(ctx context.Context, serverURL *url.URL) error {
	cfg := inventory.DefaultConfig()
	builder, err := inventory.NewBuilder(serverURL, cfg)
	if err != nil {
		return fmt.Errorf("create builder: %w", err)
	}
	defer builder.Close(ctx)

	buildCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	return builder.Build(buildCtx)
}
