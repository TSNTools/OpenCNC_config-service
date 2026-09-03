package main

import (
	"fmt"
	"os"

	"OpenCNC/common/managementSessions"
	"OpenCNC/common/observability"
	"OpenCNC/common/structures/credentials"
	"OpenCNC/common/structures/topology"
	topology_config "OpenCNC/common/structures/topology_config"
	vlan "OpenCNC/common/structures/vlan"
	"OpenCNC/config_service/pkg/plugins/netconf"
	protocolnetconf "OpenCNC/config_service/pkg/protocolbackends/netconf"
	"github.com/openshift-telco/go-netconf-client/netconf/message"
)

func main() {
	logger := observability.NormalizeLogger(nil)

	target := &topology.Node{
		Name: "SWITCH",
		Type: topology.NodeRole_BRIDGE,
		DeviceInfo: &topology.DeviceInfo{
			DeviceModel: "RELY-TSN4",
		},
		ManagementInfo: &topology.ManagementInfo{
			IpAddress:      "192.168.4.64",
			ManagementPort: 830,
			Protocol:       topology.ManagementProtocol_NETCONF,
		},
	}

	cred := &credentials.ManagementCredentials{
		Authentication: &credentials.ManagementCredentials_UsernamePassword{
			UsernamePassword: &credentials.UsernamePassword{
				Username: "sys-admin",
				Password: "sys-admin",
			},
		},
	}

	factory := managementSessions.NewNetconfFactory(target, cred)
	session, err := factory.NewSession()
	if err != nil {
		fmt.Printf("NewSession failed: %v\n", err)
		os.Exit(1)
	}
	defer session.Close()

	snapshot, err := protocolnetconf.GetRunningSnapshot(session)
	if err != nil {
		fmt.Printf("GetRunningSnapshot failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Got running snapshot, length: %d\n", len(snapshot.XML))

	working := snapshot.Clone().(*protocolnetconf.NetconfSnapshot)

	// Test 1: Let us see what VlanNetconfPlugin_2021 produces for PORT_0
	plugin := netconf.NewVlanNetconfPlugin2021(logger)

	portCfg0 := &topology_config.PortConfig{
		PortId: "PORT_0",
		VlanMemberships: []*vlan.VlanMembership{
			{VlanId: 2, Tagged: true},
		},
	}
	mapped0, err := plugin.Map(portCfg0, target)
	if err != nil {
		fmt.Printf("Map port 0 failed: %v\n", err)
		os.Exit(1)
	}
	featureXML0, err := plugin.BuildFeatureXML(mapped0)
	if err != nil {
		fmt.Printf("BuildFeatureXML 0 failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("PORT_0 FeatureXML:\n%s\n", string(featureXML0.XML))

	err = working.Update(featureXML0, target, "PORT_0")
	if err != nil {
		fmt.Printf("Update PORT_0 failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Updated PORT_0 in working snapshot")

	// Now try pushing working snapshot and print exact raw reply
	pushSession, err := factory.NewSession()
	if err != nil {
		fmt.Printf("NewSession for push failed: %v\n", err)
		os.Exit(1)
	}
	defer pushSession.Close()

	rpc := message.NewEditConfig(
		message.DatastoreRunning,
		message.DefaultOperationTypeMerge,
		string(working.XML),
	)
	reply, err := pushSession.SyncRPC(rpc, 30)
	if err != nil {
		fmt.Printf("SyncRPC error: %v\n", err)
	}
	if reply != nil {
		fmt.Printf("RAW REPLY FROM SWITCH:\n%s\n", reply.RawReply)
	}
}
