package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	storewrapper "OpenCNC_config_service/common/store-wrapper"
	"OpenCNC_config_service/common/structures/devicemodelregistry"
	"OpenCNC_config_service/common/structures/service"
	"OpenCNC_config_service/common/structures/topology"
	"OpenCNC_config_service/common/structures/topology_config"
	"OpenCNC_config_service/common/structures/vlan"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
)

func main() {
	grpcServerAddr := flag.String("addr", "localhost:5150", "Address of config-service gRPC server")
	switchIP := flag.String("ip", "192.168.4.64", "IP address of the target switch")
	switchUser := flag.String("user", "sys-admin", "NETCONF username for the switch")
	bridgeName := flag.String("bridge", "SWITCH", "Bridge and component name")
	vlanId := flag.Uint("vid", 11, "VLAN ID to configure")
	flag.Parse()

	log.Println("=================================================================")
	log.Println("  Functional Test: Bridge + Bridge-Port VLAN Config (gRPC)")
	log.Println("=================================================================")

	// -------------------------------------------------------------------------
	// Step 1: Save Device Model 'TTTech-EVB' to etcd
	// -------------------------------------------------------------------------
	log.Println("[Step 1] Saving device model 'TTTech-EVB' to the store...")
	deviceModel := &devicemodelregistry.DeviceModel{
		Name: "TTTech-EVB",
		YangFiles: []*devicemodelregistry.YangFile{
			{
				Name:        "ieee802-dot1q-bridge.yang",
				Revision:    "2021-04-09",
				Description: "IEEE 802.1Q Bridge YANG Module",
			},
			{
				Name:        "ietf-interfaces.yang",
				Revision:    "2018-02-20",
				Description: "IETF Interfaces YANG Module",
			},
		},
	}

	if err := storewrapper.StoreDeviceModel(deviceModel); err != nil {
		log.Fatalf("Failed to store device model: %v", err)
	}
	log.Println("✓ Device model stored successfully.")

	// -------------------------------------------------------------------------
	// Step 2: Clean and Save Target Switch Topology Node to etcd
	// -------------------------------------------------------------------------
	log.Println("[Step 2] Cleaning up existing stale topology entries in store...")
	if err := storewrapper.DeleteTopology(); err != nil {
		log.Printf("Warning: Failed to clean topology before test: %v", err)
	}

	log.Printf("Saving topology node '%s' (IP: %s) to the store...\n", *bridgeName, *switchIP)
	topo := &topology.Topology{
		Nodes: []*topology.Node{
			{
				Name: *bridgeName,
				Type: topology.NodeRole_BRIDGE,
				Ports: []*topology.Port{
					{
						Id:   "PORT_0",
						Name: "PORT_0",
					},
					{
						Id:   "PORT_1",
						Name: "PORT_1",
					},
				},
				ManagementInfo: &topology.ManagementInfo{
					IpAddress:      *switchIP,
					UserName:       *switchUser,
					ManagementPort: 830,
					Protocol:       topology.ManagementProtocol_NETCONF,
				},
				DeviceInfo: &topology.DeviceInfo{
					DeviceModel: "TTTech-EVB",
				},
			},
		},
	}

	if err := storewrapper.StoreTopology(topo); err != nil {
		log.Fatalf("Failed to store topology: %v", err)
	}
	log.Println("✓ Topology stored successfully.")

	// -------------------------------------------------------------------------
	// Step 3: Build TopologyConfig with BOTH Bridge AND Port VLAN settings
	// -------------------------------------------------------------------------
	log.Println("[Step 3] Building TopologyConfig with Bridge + Port VLAN settings...")

	// 3A. Bridge-level VLAN Configuration (<bridges>)
	vlanConfig := &vlan.BridgeVlanConfig{
		VlanRegistrationEntries: []*vlan.VlanRegistrationEntry{
			{
				DatabaseId: 1,
				VlanIds:    []uint32{uint32(*vlanId)},
				EntryType:  vlan.VlanRegistrationEntryType_VLAN_REG_ENTRY_TYPE_STATIC,
				PortMaps: []*vlan.VlanPortMap{
					{
						PortId:                "PORT_0",
						RegistrarAdminControl: vlan.RegistrarAdminControl_REGISTRAR_ADMIN_CONTROL_NORMAL,
						VlanTransmitted:       vlan.VlanTransmitted_VLAN_TRANSMITTED_UNTAGGED,
					},
					{
						PortId:                "PORT_1",
						RegistrarAdminControl: vlan.RegistrarAdminControl_REGISTRAR_ADMIN_CONTROL_NORMAL,
						VlanTransmitted:       vlan.VlanTransmitted_VLAN_TRANSMITTED_TAGGED,
					},
				},
			},
		},
		VidToFidMappings: []*vlan.VidToFidMapping{
			{Vid: uint32(*vlanId), Fid: 1},
		},
	}

	bridgeConfig := &topology_config.BridgeConfig{
		BridgeName:    proto.String(*bridgeName),
		ComponentName: proto.String(*bridgeName),
		VlanConfig:    vlanConfig,
	}

	// 3B. Port-level VLAN Configuration (<bridge-port>)
	portConfig0 := &topology_config.PortConfig{
		PortId:        "PORT_0",
		DefaultVlanId: proto.Uint32(uint32(*vlanId)), // PVID = 10 for untagged ingress
		VlanAdvanced: &vlan.PortVlanAdvancedConfig{
			AcceptableFrame:         vlan.AcceptableFrameType_ACCEPTABLE_FRAME_TYPE_ADMIT_ALL_FRAMES.Enum(),
			IngressFilteringEnabled: proto.Bool(true),
		},
	}

	portConfig1 := &topology_config.PortConfig{
		PortId:        "PORT_1",
		DefaultVlanId: proto.Uint32(1), // Default PVID = 1 for trunk port
		VlanAdvanced: &vlan.PortVlanAdvancedConfig{
			AcceptableFrame:         vlan.AcceptableFrameType_ACCEPTABLE_FRAME_TYPE_ADMIT_ALL_FRAMES.Enum(),
			IngressFilteringEnabled: proto.Bool(true),
		},
	}

	configID := fmt.Sprintf("vlan-full-config-%d", time.Now().Unix())
	topologyConfig := &topology_config.TopologyConfig{
		ConfigId: configID,
		NodeConfigs: []*topology_config.NodeConfig{
			{
				NodeId:      *bridgeName,
				Bridge:      bridgeConfig,
				PortConfigs: []*topology_config.PortConfig{portConfig0, portConfig1},
			},
		},
	}
	log.Printf("✓ Created TopologyConfig (ID: %s) with Bridge and Port VLAN configs\n", configID)

	// -------------------------------------------------------------------------
	// Step 4: Connect to Config Service via gRPC
	// -------------------------------------------------------------------------
	log.Printf("[Step 4] Connecting to Config Service via gRPC at %s...\n", *grpcServerAddr)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(
		ctx,
		*grpcServerAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		log.Fatalf("Failed to connect to Config Service gRPC server at %s: %v", *grpcServerAddr, err)
	}
	defer conn.Close()
	log.Println("✓ Connected to Config Service gRPC server.")

	// -------------------------------------------------------------------------
	// Step 5: Send ApplyConfiguration RPC Message
	// -------------------------------------------------------------------------
	log.Println("[Step 5] Calling ApplyConfiguration RPC...")
	client := service.NewConfigServiceClient(conn)

	req := &service.ConfigurationRequest{
		Id:            proto.String(configID),
		Configuration: topologyConfig,
	}

	resp, err := client.ApplyConfiguration(ctx, req)
	if err != nil {
		log.Fatalf("ApplyConfiguration gRPC call failed: %v", err)
	}

	log.Println("=================================================================")
	log.Printf("gRPC Response Success : %t\n", resp.GetSuccess())
	log.Printf("gRPC Response Message : %s\n", resp.GetMessage())
	log.Println("=================================================================")
}
