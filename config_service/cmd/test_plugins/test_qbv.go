package main

import (
	"fmt"
	"log"
	"os"

	qbv "OpenCNC_config_service/common/structures/qbv"
	"OpenCNC_config_service/common/structures/topology"
	opencncModel "OpenCNC_config_service/config_service/opencnc_model"
	managementSessions "OpenCNC_config_service/config_service/pkg/managementSessions"
	netconf "OpenCNC_config_service/config_service/pkg/plugins/netconf"

	"github.com/golang/protobuf/proto"
	"github.com/openconfig/ygot/ygot"
)

func TestQbvPlugin() {
	// Create plugin with nil logger
	plugin := netconf.NewQbvNetconfPlugin(nil)

	// Fake GateControlList message
	gcl := &qbv.GateControlList{
		ScheduleId: "sched1",
		AdminState: qbv.AdminState_ENABLED,
		BaseTime:   1_500_000_000, // ns
		CycleTime:  10_000_000,    // ns
		// NON-STANDARD FIELD: admin-gate-states for testing
		InterfaceTimeOffsetNs: ygot.Int64(0), // optional, not used here
		Entries: []*qbv.GateControlEntry{
			{
				Index:        0,
				TimeInterval: 500_000,
				GateStates:   []byte{0x0F}, // all gates open for testing
			},
			{
				Index:        1,
				TimeInterval: 500_000,
				GateStates:   []byte{0x03}, // partial gates open
			},
		},
	}

	// Map the message to YGOT structure
	mapped, err := plugin.Map(proto.Message(gcl))
	if err != nil {
		log.Fatalf("Map failed: %v", err)
	}

	// Build XML
	xmlStr, err := plugin.BuildFeatureXML(
		mapped.(*opencncModel.IETFInterfaces_Interfaces_Interface_BridgePort_GateParameterTable),
	)
	if err != nil {
		log.Fatalf("BuildXML failed: %v", err)
	}

	fmt.Println("===== GENERATED XML =====")
	fmt.Println(xmlStr)
	fmt.Println("=========================")
}

func TestQbvPlugin_tttech() {
	// device target
	target := managementSessions.DeviceTarget{
		InterfaceName: "sw0p3",
		Logger:        nil,
		Secret:        "", // password is empty
		Info: &topology.ManagementInfo{
			IpAddress:      "192.168.0.1", // IP address
			UserName:       "root",        // username
			ManagementPort: 830,           // default NETCONF port
			Protocol:       topology.ManagementProtocol_NETCONF,
		},
	}

	// Create plugin
	plugin := netconf.NewQbvNetconfPlugin_tttech(nil)

	// Fake GateControlList message
	gcl := &qbv.GateControlList{
		ScheduleId: "sched-tttech",
		AdminState: qbv.AdminState_ENABLED,
		BaseTime:   0, // can be 0 for relative time
		CycleTime:  1, // 1ms
		Entries: []*qbv.GateControlEntry{
			{Index: 0, TimeInterval: 10_000, GateStates: []byte{127}},
			{Index: 1, TimeInterval: 10_000, GateStates: []byte{128}},
			{Index: 2, TimeInterval: 650_000, GateStates: []byte{127}},
			{Index: 3, TimeInterval: 330_000, GateStates: []byte{127}},
		},
	}

	// Map the message to YGOT structure
	mapped, err := plugin.Map(proto.Message(gcl))
	if err != nil {
		log.Fatalf("Map failed: %v", err)
	}

	// Push the config
	err = plugin.Push(mapped, target)
	if err != nil {
		log.Fatalf("Push failed: %v", err)
	}
}

func TestQbvPlugin_relym() {
	password := os.Getenv("NETCONF_PASSWORD")
	if password == "" {
		password = ""
	}

	target := managementSessions.DeviceTarget{
		InterfaceName: "PORT_0",
		Logger:        nil,
		Secret:        password,
		Info: &topology.ManagementInfo{
			IpAddress:      "192.168.4.64", // IP address
			UserName:       "sys-admin",
			ManagementPort: 830,
			Protocol:       topology.ManagementProtocol_NETCONF,
		},
	}

	plugin := netconf.NewQbvNetconfPlugin(nil)

	gcl := &qbv.GateControlList{
		ScheduleId: "sched-relym",
		AdminState: qbv.AdminState_ENABLED,
		BaseTime:   0,
		CycleTime:  1_000_000,
		Entries: []*qbv.GateControlEntry{
			{Index: 0, TimeInterval: 250_000, GateStates: []byte{0x0F}},
			{Index: 1, TimeInterval: 250_000, GateStates: []byte{0x03}},
			{Index: 2, TimeInterval: 250_000, GateStates: []byte{0x0F}},
			{Index: 3, TimeInterval: 250_000, GateStates: []byte{0x03}},
		},
	}

	mapped, err := plugin.Map(proto.Message(gcl))
	if err != nil {
		log.Fatalf("Map failed: %v", err)
	}

	err = plugin.Push(mapped, target)
	if err != nil {
		log.Fatalf("Push failed: %v", err)
	}
}

func TestQbvPlugin_relym_multiPort() {
	password := os.Getenv("NETCONF_PASSWORD")
	if password == "" {
		password = "sys-admin"
	}

	plugin := netconf.NewQbvNetconfPlugin(nil)

	ports := []string{"PORT_0", "PORT_1", "PORT_2", "PORT_3"}
	for idx, portName := range ports {
		target := managementSessions.DeviceTarget{
			InterfaceName: portName,
			Logger:        nil,
			Secret:        password,
			Info: &topology.ManagementInfo{
				IpAddress:      "192.168.4.64",
				UserName:       "sys-admin",
				ManagementPort: 830,
				Protocol:       topology.ManagementProtocol_NETCONF,
			},
		}

		gcl := &qbv.GateControlList{
			ScheduleId: fmt.Sprintf("sched-relym-%s", portName),
			AdminState: qbv.AdminState_ENABLED,
			BaseTime:   uint64(idx) * 500_000_000,
			CycleTime:  1_000_000 + uint64(idx)*250_000,
			Entries:    []*qbv.GateControlEntry{},
		}

		switch idx {
		case 0:
			gcl.Entries = []*qbv.GateControlEntry{
				{Index: 0, TimeInterval: 150_000, GateStates: []byte{0x0F}},
				{Index: 1, TimeInterval: 350_000, GateStates: []byte{0x03}},
				{Index: 2, TimeInterval: 250_000, GateStates: []byte{0x0D}},
				{Index: 3, TimeInterval: 250_000, GateStates: []byte{0x01}},
			}
		case 1:
			gcl.Entries = []*qbv.GateControlEntry{
				{Index: 0, TimeInterval: 100_000, GateStates: []byte{0x0F}},
				{Index: 1, TimeInterval: 200_000, GateStates: []byte{0x07}},
				{Index: 2, TimeInterval: 300_000, GateStates: []byte{0x0B}},
				{Index: 3, TimeInterval: 400_000, GateStates: []byte{0x03}},
			}
		case 2:
			gcl.Entries = []*qbv.GateControlEntry{
				{Index: 0, TimeInterval: 125_000, GateStates: []byte{0x09}},
				{Index: 1, TimeInterval: 125_000, GateStates: []byte{0x05}},
				{Index: 2, TimeInterval: 125_000, GateStates: []byte{0x03}},
				{Index: 3, TimeInterval: 625_000, GateStates: []byte{0x01}},
			}
		default:
			gcl.Entries = []*qbv.GateControlEntry{
				{Index: 0, TimeInterval: 200_000, GateStates: []byte{0x0F}},
				{Index: 1, TimeInterval: 200_000, GateStates: []byte{0x01}},
				{Index: 2, TimeInterval: 200_000, GateStates: []byte{0x05}},
				{Index: 3, TimeInterval: 400_000, GateStates: []byte{0x0B}},
			}
		}

		mapped, err := plugin.Map(proto.Message(gcl))
		if err != nil {
			log.Fatalf("Map failed for %s: %v", portName, err)
		}

		err = plugin.Push(mapped, target)
		if err != nil {
			log.Fatalf("Push failed for %s: %v", portName, err)
		}
	}
}

func TestQbvPlugin_br2018_multiPort() {
	password := "admin"

	plugin := netconf.NewQbvNetconfPlugin_tttech(nil)
	ports := []string{"sw0p2", "sw0p3", "sw0p4", "sw0p5"}

	for idx, portName := range ports {
		target := managementSessions.DeviceTarget{
			InterfaceName: portName,
			Logger:        nil,
			Secret:        password,
			Info: &topology.ManagementInfo{
				IpAddress:      "192.168.4.65",
				UserName:       "admin",
				ManagementPort: 830,
				Protocol:       topology.ManagementProtocol_NETCONF,
			},
		}

		baseTime := uint64(idx) * 320_000_000

		gcl := &qbv.GateControlList{
			ScheduleId: fmt.Sprintf("sched-br2018-%s", portName),
			AdminState: qbv.AdminState_ENABLED,
			BaseTime:   baseTime,
			CycleTime:  125,
			Entries: []*qbv.GateControlEntry{
				{Index: 0, TimeInterval: 7360, GateStates: []byte{0xFF}},
				{Index: 1, TimeInterval: 3200, GateStates: []byte{0x00}},
				{Index: 2, TimeInterval: 16000, GateStates: []byte{0x08}},
				{Index: 3, TimeInterval: 3200, GateStates: []byte{0x00}},
				{Index: 4, TimeInterval: 95040, GateStates: []byte{0xF3}},
			},
		}

		mapped, err := plugin.Map(proto.Message(gcl))
		if err != nil {
			log.Fatalf("Map failed for %s: %v", portName, err)
		}

		err = plugin.Push(mapped, target)
		if err != nil {
			log.Fatalf("Push failed for %s: %v", portName, err)
		}
	}
}
