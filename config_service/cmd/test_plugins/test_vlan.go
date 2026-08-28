package main

import (
	"fmt"
	"log"

	topology "OpenCNC_config_service/common/structures/topology"
	topology_config "OpenCNC_config_service/common/structures/topology_config"
	vlan "OpenCNC_config_service/common/structures/vlan"
	managementSessions "OpenCNC_config_service/config_service/pkg/managementSessions"
	netconf "OpenCNC_config_service/config_service/pkg/plugins/netconf"

	"google.golang.org/protobuf/proto"
)

func TestVlanPlugin_tttech() {
	target := managementSessions.DeviceTarget{
		InterfaceName: "PORT_0",
		Logger:        nil,
		Secret:        "",
		Info: &topology.ManagementInfo{
			IpAddress:      "192.168.4.64",
			UserName:       "sys-admin",
			ManagementPort: 830,
			Protocol:       topology.ManagementProtocol_NETCONF,
		},
	}

	pluginVlan := netconf.NewVlanNetconfPlugin(nil)

	// 1. Bridge VLAN Configuration
	fmt.Println("\n--- [1] Bridge VLAN Configuration ---")
	mappedBridge, err := pluginVlan.Map(VlanConfig)
	if err != nil {
		log.Fatalf("Bridge VLAN Map failed: %v", err)
	}
	bridgeFeatureXML, err := pluginVlan.BuildFeatureXML(mappedBridge)
	if err != nil {
		log.Fatalf("Bridge VLAN BuildFeatureXML failed: %v", err)
	}
	bridgePrettyXML := netconf.PrettyPrintXML(string(bridgeFeatureXML.XML))
	fmt.Printf("/*******************************************************************************\n[VLAN] Generated Bridge Configuration XML (br0):\n%s\n*******************************************************************************/\n", bridgePrettyXML)

	// 2. Port VLAN Configuration (sw0p3)
	fmt.Println("\n--- [2] Port VLAN Configuration (sw0p3) ---")
	mappedPort, err := pluginVlan.Map(PortConfig)
	if err != nil {
		log.Fatalf("Port VLAN Map failed: %v", err)
	}
	portFeatureXML, err := pluginVlan.BuildFeatureXML(mappedPort)
	if err != nil {
		log.Fatalf("Port VLAN BuildFeatureXML failed: %v", err)
	}
	portWrappedXML, err := pluginVlan.WrapXML(portFeatureXML, target)
	if err != nil {
		log.Fatalf("Port VLAN WrapXML failed: %v", err)
	}
	portPrettyXML := netconf.PrettyPrintXML(portWrappedXML)
	fmt.Printf("/*******************************************************************************\n[VLAN] Generated Port Configuration XML (sw0p3):\n%s\n*******************************************************************************/\n", portPrettyXML)

	// 3. Node VLAN Configuration
	fmt.Println("\n--- [3] Node VLAN Configuration ---")
	mappedNode, err := pluginVlan.Map(nodeConfig)
	if err != nil {
		log.Fatalf("Node VLAN Map failed: %v", err)
	}
	nodeFeatureXML, err := pluginVlan.BuildFeatureXML(mappedNode)
	if err != nil {
		log.Fatalf("Node VLAN BuildFeatureXML failed: %v", err)
	}
	nodePrettyXML := netconf.PrettyPrintXML(string(nodeFeatureXML.XML))
	fmt.Printf("/*******************************************************************************\n[VLAN] Generated Node Configuration XML:\n%s\n*******************************************************************************/\n", nodePrettyXML)

	// 4. Attempt Push (optional / device live test)
	fmt.Println("\n--- [4] Pushing Bridge VLAN Config to Device (192.168.4.64) ---")
	if err := pluginVlan.Push(mappedBridge, target); err != nil {
		log.Printf("Push skipped / failed (expected if device is offline): %v", err)
	}
}

var PortConfig = &topology_config.PortConfig{
	PortId:        "PORT_0",
	DefaultVlanId: proto.Uint32(30),
	VlanMemberships: []*vlan.VlanMembership{
		{VlanId: 10, Tagged: true},
		{VlanId: 20, Tagged: true},
		{VlanId: 30, Tagged: true},
	},
	VlanAdvanced: &vlan.PortVlanAdvancedConfig{
		AcceptableFrame:                   vlan.AcceptableFrameType_ACCEPTABLE_FRAME_TYPE_ADMIT_ALL_FRAMES.Enum(),
		IngressFilteringEnabled:           proto.Bool(true),
		RestrictedVlanRegistrationEnabled: proto.Bool(false),
		VidTranslationTableEnabled:        proto.Bool(true),
		EgressVidTranslationTableEnabled:  proto.Bool(true),
		VidTranslations: []*vlan.VidTranslation{
			{LocalVid: 100, RelayVid: 10},
		},
		EgressVidTranslations: []*vlan.VidTranslation{
			{LocalVid: 10, RelayVid: 100},
		},
		ProtocolGroupVidSets: []*vlan.ProtocolGroupVidSet{
			{GroupId: 100, VlanIds: []uint32{10}},
			{GroupId: 200, VlanIds: []uint32{30}},
		},
	},
}

var VlanConfig = &vlan.BridgeVlanConfig{
	VlanRegistrationEntries: []*vlan.VlanRegistrationEntry{
		{
			DatabaseId: 1,
			VlanIds:    []uint32{10, 20, 30},
			EntryType:  vlan.VlanRegistrationEntryType_VLAN_REG_ENTRY_TYPE_STATIC,
			PortMaps: []*vlan.VlanPortMap{
				{
					PortId:                "PORT_0",
					RegistrarAdminControl: vlan.RegistrarAdminControl_REGISTRAR_ADMIN_CONTROL_NORMAL,
					VlanTransmitted:       vlan.VlanTransmitted_VLAN_TRANSMITTED_TAGGED,
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
		{Vid: 10, Fid: 1},
		{Vid: 20, Fid: 2},
		{Vid: 30, Fid: 2},
	},
}

var Bridge = &topology_config.BridgeConfig{
	BridgeName:    proto.String("SWITCH"),
	ComponentName: proto.String("SWITCH"),
	VlanConfig:    VlanConfig,
}
var nodeConfig = &topology_config.NodeConfig{
	NodeId:      "SWITCH",
	Bridge:      Bridge,
	PortConfigs: []*topology_config.PortConfig{PortConfig}}
