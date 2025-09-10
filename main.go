package main

import (
	"log"

	"engine"
	"plugins/netconf"
	"protocolbackends"
	"structures"
	"structures/topology_config"
)

func main() {
	logger := log.Default()
	initBackends(logger)

	var config *topology_config.TopologyConfig
	// TODO: Load config externally

	if config == nil {
		logger.Fatal("TopologyConfig is nil — did you forget to load it?")
	}

	mappingEngine := engine.NewMappingEngine(logger)

	err := mappingEngine.ApplyTopologyConfig(config)
	if err != nil {
		logger.Fatalf("❌ Failed to apply configuration: %v", err)
	}

	logger.Println("✅ Mapping complete.")
}

// ---------------------------------------------------
// Backend initialization
// ---------------------------------------------------

func initBackends(logger *log.Logger) {
	netconfBackend := protocolbackends.NewNetconfBackend()
	netconfBackend.AddPlugin(netconf.NewQbvNetconfPlugin(logger))

	protocolbackends.RegisterBackend(structures.ManagementProtocol_NETCONF, netconfBackend)

	// Add other protocol backends like SNMP here as needed
}
