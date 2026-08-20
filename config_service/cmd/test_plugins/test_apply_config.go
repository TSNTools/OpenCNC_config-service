package main

import (
	"fmt"
	"log"
	"os"

	storewrapper "OpenCNC_config_service/common/store-wrapper"
	"OpenCNC_config_service/common/structures/topology"
	"OpenCNC_config_service/config_service/pkg/engine"
	"OpenCNC_config_service/config_service/pkg/managementSessions"
	"OpenCNC_config_service/config_service/pkg/plugins"
	"OpenCNC_config_service/config_service/pkg/protocolbackends"
)

// TestApplyConfigByIdWithRunningConfig retrieves the configuration by ID,
// fetches the live running config from each target node to initialize snapshots,
// and applies the configuration via the mapping engine.
func TestApplyConfigByIdWithRunningConfig(configID string) {
	log.Printf("[TEST-APPLY-CONFIG-BY-ID] Starting test for applyConfigById with config ID: %s", configID)

	if configID == "" {
		log.Fatalf("Config ID is empty!")
	}

	// 1. Fetch Topology and Configuration from KV Store
	topo, err := storewrapper.GetTopology()
	if err != nil {
		log.Fatalf("Failed to retrieve topology from store: %v", err)
	}
	log.Printf("Retrieved topology with %d nodes", len(topo.GetNodes()))

	cfg, err := storewrapper.GetConfiguration(configID)
	if err != nil {
		log.Fatalf("Failed to retrieve configuration %s from store: %v", configID, err)
	}
	log.Printf("Retrieved configuration %s", cfg.GetConfigId())

	// 2. Initialize Engine & NETCONF Backend with Plugins
	mappingEngine := engine.NewMappingEngine(nil)
	netconfPlugins := plugins.ForProtocol(topology.ManagementProtocol_NETCONF, nil)
	netconfBackend := protocolbackends.NewNetconfBackend("netconf", nil, netconfPlugins...)

	// 3. For each node, fetch the live running config to populate snapshots
	for _, node := range topo.GetNodes() {
		if node == nil || node.ManagementInfo == nil {
			continue
		}
		if node.ManagementInfo.Protocol == topology.ManagementProtocol_NETCONF {
			secret := os.Getenv("NETCONF_PASSWORD")
			if secret == "" {
				secret = node.ManagementInfo.UserName
			}

			log.Printf("Target Node: %s | IP: %s | User: %s | Port: %d",
				node.Name,
				node.ManagementInfo.IpAddress,
				node.ManagementInfo.UserName,
				node.ManagementInfo.ManagementPort,
			)
			log.Printf("Fetching live running configuration for node %s (%s:830)...", node.Name, node.ManagementInfo.IpAddress)
			if err := fetchAndInitNodeSnapshot(netconfBackend, node, secret); err != nil {
				log.Fatalf("Failed to fetch initial running config for node %s (%s:830): %v", node.Name, node.ManagementInfo.IpAddress, err)
			}
		}
	}

	// 4. Register Backend
	mappingEngine.RegisterBackend(netconfBackend)

	// 5. Apply Configuration
	secret := os.Getenv("NETCONF_PASSWORD")
	log.Printf("Applying configuration ID %s across nodes...", configID)
	if err := mappingEngine.ApplyConfiguration(topo, cfg, secret); err != nil {
		log.Fatalf("ApplyConfiguration failed: %v", err)
	}

	log.Printf("Successfully applied configuration ID %s to all target nodes!", configID)
}

// fetchAndInitNodeSnapshot connects to a node via NETCONF, retrieves its running configuration,
// and initializes the base snapshot in the NetconfBackend for testing purposes.
func fetchAndInitNodeSnapshot(backend *protocolbackends.NetconfBackend, node *topology.Node, secret string) error {
	if node == nil || node.ManagementInfo == nil {
		return fmt.Errorf("node or management info is nil")
	}
	session, err := managementSessions.CreateSession(
		node.ManagementInfo.IpAddress,
		node.ManagementInfo.UserName,
		secret,
	)
	if err != nil {
		return fmt.Errorf("failed connecting to node %s: %w", node.Name, err)
	}
	defer session.Close()

	rawXML, err := managementSessions.GetRunningConfig(session)
	if err != nil {
		return fmt.Errorf("failed fetching running config for node %s: %w", node.Name, err)
	}

	backend.InitSnapshot(node.Name, []byte(rawXML))
	return nil
}

func printUsageHelp() {
	fmt.Println("Usage: go run . -configId <CONFIGURATION_ID>")
}
