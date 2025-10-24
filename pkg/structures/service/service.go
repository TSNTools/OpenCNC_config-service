package service

import (
	"context"
	"fmt"
	"log"
	"os"

	"OpenCNC_config_service/pkg/managementSessions"
	"OpenCNC_config_service/pkg/plugins/netconf"
	storewrapper "OpenCNC_config_service/pkg/store-wrapper"
	"OpenCNC_config_service/pkg/structures/topology"

	"github.com/golang/protobuf/proto"
)

// ConfigServiceServer implements the generated gRPC interface.
type ConfigServiceServerImpl struct {
	UnimplementedConfigServiceServer
	logger *log.Logger
}

// Constructor
func NewConfigServiceServerImpl(logger *log.Logger) *ConfigServiceServerImpl {
	return &ConfigServiceServerImpl{logger: logger}
}

// ApplyConfiguration receives a config request ID, retrieves it from store,
// maps it via the plugin, and pushes it to all NETCONF-capable devices.
func (s *ConfigServiceServerImpl) ApplyConfiguration(
	ctx context.Context,
	req *ConfigurationRequest,
) (*ConfigurationResponse, error) {

	s.logger.Printf("[Config-Service] Received ApplyConfiguration request for ID: %s", req.Id)

	secret := os.Getenv("NETCONF_PASSWORD")
	plugin := netconf.NewQbvNetconfPlugin(s.logger)

	confReq, err := storewrapper.GetConfiguration(req.Id)
	if err != nil {
		msg := fmt.Sprintf("Failed to get configuration request from store: %v", err)
		s.logger.Println(msg)
		return &ConfigurationResponse{Success: false, Message: msg}, err
	}

	s.logger.Println("[Config-Service] Mapping configuration model...")
	mapped, err := plugin.Map(proto.Message(confReq))
	if err != nil {
		msg := fmt.Sprintf("Mapping failed: %v", err)
		s.logger.Println(msg)
		return &ConfigurationResponse{Success: false, Message: msg}, err
	}
	s.logger.Println("[Config-Service] Mapping successful!")

	topo, err := storewrapper.GetTopology()
	if err != nil {
		msg := fmt.Sprintf("Failed to get topology: %v", err)
		s.logger.Println(msg)
		return &ConfigurationResponse{Success: false, Message: msg}, err
	}

	for _, node := range topo.Nodes {
		if node.ManagementInfo == nil || node.ManagementInfo.Protocol != topology.ManagementProtocol_NETCONF {
			continue
		}
		for _, port := range node.Ports {
			target := managementSessions.DeviceTarget{
				Info:          node.ManagementInfo,
				Secret:        secret,
				Logger:        s.logger,
				InterfaceName: port.Name,
			}
			if err := plugin.Push(mapped, target); err != nil {
				s.logger.Printf("❌ Failed to push config to %s/%s: %v", node.Name, port.Name, err)
			} else {
				s.logger.Printf("✅ Successfully pushed config to %s/%s", node.Name, port.Name)
			}
		}
	}

	s.logger.Println("[Config-Service] Configuration applied successfully!")
	return &ConfigurationResponse{
		Success: true,
		Message: "Configuration applied successfully",
	}, nil
}

// Optional: simple health check RPC.
func (s *ConfigServiceServerImpl) Ping(ctx context.Context, _ *ConfigurationRequest) (*ConfigurationResponse, error) {
	return &ConfigurationResponse{Success: true, Message: "pong"}, nil
}

// Helper: apply last known configuration at startup
func (s *ConfigServiceServerImpl) ApplyLastConfiguration() {
	_, id, err := storewrapper.GetLastConfiguration()
	if err != nil {
		s.logger.Printf("No previous configuration found: %v", err)
		return
	}

	s.logger.Println("Loaded last known configuration from store")
	resp, err := s.ApplyConfiguration(context.Background(), &ConfigurationRequest{Id: id})
	if err != nil {
		s.logger.Printf("Failed to apply last configuration: %v", err)
	} else {
		s.logger.Println("Successfully applied last configuration:", resp.Message)
	}

}
