package main

import (
	"fmt"
	"log"
	"os"

	storewrapper "OpenCNC_config-service/common/store-wrapper"
	"OpenCNC_config-service/common/structures/topology"
	"OpenCNC_config-service/config_service/pkg/engine"
	"OpenCNC_config-service/config_service/pkg/plugins"
	"OpenCNC_config-service/config_service/pkg/protocolbackends"
)

// TestApplyConfigByIdWithRunningConfig retrieves the configuration by ID,
// fetches the live running config from each target node to initialize snapshots,
// and applies the configuration via the mapping engine.
func TestApplyConfigByIdWithRunningConfig(configID string) {
	testLogger := log.New(log.Writer(), "[TEST-APPLY-CONFIG-BY-ID] ", log.LstdFlags)
	testLogger.Printf("Starting test for applyConfigById with config ID: %s", configID)

	if configID == "" {
		testLogger.Fatalf("Config ID is empty!")
	}

	// 1. Fetch Topology and Configuration from KV Store
	topo, err := storewrapper.GetTopology()
	if err != nil {
		testLogger.Fatalf("Failed to retrieve topology from store: %v", err)
	}
	testLogger.Printf("Retrieved topology with %d nodes", len(topo.GetNodes()))

	cfg, err := storewrapper.GetConfiguration(configID)
	if err != nil {
		testLogger.Fatalf("Failed to retrieve configuration %s from store: %v", configID, err)
	}
	testLogger.Printf("Retrieved configuration %s", cfg.GetConfigId())

	// 2. Initialize Engine & NETCONF Backend with Plugins
	mappingEngine := engine.NewMappingEngine(testLogger)
	netconfPlugins := plugins.ForProtocol(topology.ManagementProtocol_NETCONF, testLogger)
	netconfBackend := protocolbackends.NewNetconfBackend("netconf", testLogger, netconfPlugins...)

	secret := os.Getenv("NETCONF_PASSWORD")
	if secret == "" {
		secret = "sys-admin"
	}

	// 3. For each node, fetch the live running config to populate snapshots
	for _, node := range topo.GetNodes() {
		if node == nil || node.ManagementInfo == nil {
			continue
		}
		if node.ManagementInfo.Protocol == topology.ManagementProtocol_NETCONF {
			testLogger.Printf("Target Node: %s | IP: %s | User: %s | Port: %d",
				node.Name,
				node.ManagementInfo.IpAddress,
				node.ManagementInfo.UserName,
				node.ManagementInfo.ManagementPort,
			)
			testLogger.Printf("Fetching live running configuration for node %s (%s:830)...", node.Name, node.ManagementInfo.IpAddress)
			if err := netconfBackend.FetchAndInitSnapshot(node, secret); err != nil {
				testLogger.Fatalf("Failed to fetch initial running config for node %s (%s:830): %v", node.Name, node.ManagementInfo.IpAddress, err)
			}
		}
	}

	// 4. Register Backend
	mappingEngine.RegisterBackend(netconfBackend)

	// 5. Apply Configuration
	testLogger.Printf("Applying configuration ID %s across nodes...", configID)
	if err := mappingEngine.ApplyConfiguration(topo, cfg, secret); err != nil {
		testLogger.Fatalf("ApplyConfiguration failed: %v", err)
	}

	testLogger.Printf("Successfully applied configuration ID %s to all target nodes!", configID)
}

func printUsageHelp() {
	fmt.Println("Usage: go run . -configId <CONFIGURATION_ID>")
}
