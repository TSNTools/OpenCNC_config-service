package main

import (
	"fmt"
	"os"

	"OpenCNC/common/managementSessions"
	"OpenCNC/common/structures/credentials"
	"OpenCNC/common/structures/topology"
	protocolnetconf "OpenCNC/config_service/pkg/protocolbackends/netconf"

	"github.com/openshift-telco/go-netconf-client/netconf/message"
)

func main() {
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

	// Test pushing the EXACT unmodified snapshot to edit-config!
	pushSession, err := factory.NewSession()
	if err != nil {
		fmt.Printf("NewSession for push failed: %v\n", err)
		os.Exit(1)
	}
	defer pushSession.Close()

	rpc := message.NewEditConfig(
		message.DatastoreRunning,
		message.DefaultOperationTypeMerge,
		string(snapshot.XML),
	)
	reply, err := pushSession.SyncRPC(rpc, 30)
	if err != nil {
		fmt.Printf("SyncRPC error: %v\n", err)
	}
	if reply != nil {
		fmt.Printf("RAW REPLY FOR UNMODIFIED SNAPSHOT:\n%s\n", reply.RawReply)
	}
}
