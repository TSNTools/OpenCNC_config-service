package engine

import (
	"fmt"
	"log"

	"managementSessions"
	"protocolbackends"
	"structures/topology_config"
)

// QbvMappingEngine implements MappingEngine interface specifically for IEEE 802.1Qbv feature.
type QbvMappingEngine struct {
	logger *log.Logger
}

// NewQbvMappingEngine creates a new QbvMappingEngine with the provided logger.
func NewQbvMappingEngine(logger *log.Logger) *QbvMappingEngine {
	return &QbvMappingEngine{logger: logger}
}

// ApplyTopologyConfig applies the Qbv configuration found in the given TopologyConfig.
// It maps the GateControlList (GCL) from topology_config.PortConfig to the device target using the
// appropriate protocol backend.
func (e *QbvMappingEngine) ApplyTopologyConfig(config *topology_config.TopologyConfig) error {
	for _, nodeCfg := range config.NodeConfigs {
		e.logger.Printf("Processing node config: %s", nodeCfg.NodeId)

		// In your model, you have to find the ManagementInfo for this node by node ID
		// (Assuming you have a way to get the node info or you can pass it together)

		// Here we mock getting ManagementInfo and DeviceInfo for demonstration
		// Replace with actual lookup logic or pass it as parameter
		mgmtInfo := getManagementInfoForNode(nodeCfg.NodeId)
		if mgmtInfo == nil {
			return fmt.Errorf("missing management info for node %s", nodeCfg.NodeId)
		}

		backend, err := protocolbackends.GetBackend(mgmtInfo.Protocol)
		if err != nil {
			return fmt.Errorf("unsupported protocol for node %s: %w", nodeCfg.NodeId, err)
		}

		target := managementSessions.DeviceTarget{
			Info:   mgmtInfo,
			Logger: e.logger,
		}

		// You may want to build or lookup DeviceModel here based on your device info
		model := getDeviceModelForNode(nodeCfg.NodeId)

		for _, portCfg := range nodeCfg.PortConfigs {
			if portCfg.Gcl != nil {
				gcl := portCfg.Gcl
				e.logger.Printf("Applying Qbv config on node %s port %s schedule ID: %s",
					nodeCfg.NodeId, portCfg.PortId, gcl.ScheduleId)

				// Call the backend to map and push the GCL config
				if err := backend.MapAndPush(gcl, model, target); err != nil {
					return fmt.Errorf("failed to apply Qbv config on node %s port %s: %w",
						nodeCfg.NodeId, portCfg.PortId, err)
				}
			}
		}
	}

	return nil
}

// Dummy placeholder to get ManagementInfo for a node ID.
// Replace this with your actual data retrieval logic.
func getManagementInfoForNode(nodeID string) *managementSessions.ManagementInfo {
	// Implementation depends on your data structures/storage
	return nil
}

// Dummy placeholder to get DeviceModel for a node ID.
// Replace this with your actual data retrieval logic.
func getDeviceModelForNode(nodeID string) *managementSessions.DeviceModel {
	// Implementation depends on your data structures/storage
	return nil
}
