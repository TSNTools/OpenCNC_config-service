package main

import (
	"log"
	"os"

	"OpenCNC_config_service/pkg/managementSessions"
	"OpenCNC_config_service/pkg/plugins/netconf"
	qbv "OpenCNC_config_service/pkg/structures/qbv"
	topology "OpenCNC_config_service/pkg/structures/topology"

	"github.com/golang/protobuf/proto"
)

func main() {
	logger := log.New(os.Stdout, "", log.LstdFlags)
	plugin := netconf.NewQbvNetconfPlugin(logger)

	// Create a mock GateControlList message
	gcl := &qbv.GateControlList{
		ScheduleId: "test-sched-001",
		BaseTime:   1680000000000000000, // nanoseconds
		CycleTime:  1350000,             // nanoseconds
		AdminState: qbv.AdminState_ENABLED,
		Entries: []*qbv.GateControlEntry{
			{
				Index:        1,
				TimeInterval: 500000,
				GateStates:   []byte{0b11111111},
				Description:  "Open all gates",
			},
			{
				Index:        2,
				TimeInterval: 700000,
				GateStates:   []byte{0b00001111},
				Description:  "Open lower gates",
			},
		},
	}

	// Call the Map function
	mapped, err := plugin.Map(proto.Message(gcl))
	if err != nil {
		logger.Fatalf("Map failed: %v", err)
	}

	logger.Println("Mapping successful !")
	//logger.Printf("%+v\n", mapped.(*opencncModel.IETFInterfaces_Interfaces_Interface_BridgePort_GateParameterTable))

	// Define test target (set actual IP and credentials here)
	target := managementSessions.DeviceTarget{
		Info: &topology.ManagementInfo{
			IpAddress:      "192.168.4.64",
			ManagementPort: 830,
			UserName:       "sys-admin",
			Protocol:       topology.ManagementProtocol_NETCONF,
		},
		Secret:        "sys-admin",
		Logger:        logger,
		InterfaceName: "PORT_0",
	}

	// Call the Push function
	err = plugin.Push(mapped, target)
	if err != nil {
		logger.Fatalf("Push failed: %v", err)
	}
}
