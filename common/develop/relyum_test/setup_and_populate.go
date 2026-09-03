package main

import (
	"encoding/json"
	"fmt"
	"os"

	store "OpenCNC/common/store-wrapper"
	"OpenCNC/common/structures/credentials"
	devicemodelregistry "OpenCNC/common/structures/devicemodelregistry"
	"OpenCNC/common/structures/topology"

	"google.golang.org/protobuf/encoding/protojson"
)

func main() {
	fmt.Println("--- Loading RELY-TSN4 Device Model ---")
	modelPath := "/home/jittin/config-service/new/OpenCNC_config-service/common/deviceModels/1_device_model.json"
	data, err := os.ReadFile(modelPath)
	if err != nil {
		fmt.Printf("Failed to read model file: %v\n", err)
		os.Exit(1)
	}

	model := &devicemodelregistry.DeviceModel{}
	err = protojson.UnmarshalOptions{DiscardUnknown: true}.Unmarshal(data, model)
	if err != nil {
		fmt.Printf("Failed to unmarshal device model via protojson: %v\n", err)
		// Fallback to json.Unmarshal into map
		var rawMap struct {
			ModelName string `json:"model-name"`
			Vendor    string `json:"vendor"`
			Version   string `json:"version"`
			YangFiles []struct {
				FileName     string `json:"file-name"`
				FileRevision string `json:"file-revision"`
				Description  string `json:"description"`
			} `json:"yang-files"`
		}
		if err2 := json.Unmarshal(data, &rawMap); err2 != nil {
			fmt.Printf("Fallback json unmarshal failed: %v\n", err2)
			os.Exit(1)
		}
		model.Name = rawMap.ModelName
		model.Vendor = rawMap.Vendor
		model.Version = rawMap.Version
		for _, y := range rawMap.YangFiles {
			model.YangFiles = append(model.YangFiles, &devicemodelregistry.YangModule{
				Name:        y.FileName,
				Revision:    y.FileRevision,
				Description: y.Description,
			})
		}
	}

	fmt.Printf("Device model unmarshaled: Name=%s, YangFiles=%d\n", model.Name, len(model.YangFiles))

	// Store device model
	if err := store.StoreDeviceModel(model); err != nil {
		fmt.Printf("Failed to store device model: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Successfully stored device model %s into etcd\n", model.Name)

	// Verify retrieval
	retrievedModel, err := store.GetDeviceModel(model.Name)
	if err != nil {
		fmt.Printf("Failed to retrieve device model: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Verified device model retrieval: %s (yang files: %d)\n", retrievedModel.Name, len(retrievedModel.YangFiles))

	// Store credentials for SWITCH
	fmt.Println("--- Storing Credentials for SWITCH ---")
	creds := &credentials.ManagementCredentials{
		Authentication: &credentials.ManagementCredentials_UsernamePassword{
			UsernamePassword: &credentials.UsernamePassword{
				Username: "sys-admin",
				Password: "sys-admin",
			},
		},
	}

	if err := store.StoreCredentials("SWITCH", creds); err != nil {
		fmt.Printf("Failed to store credentials: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Successfully stored credentials for SWITCH into etcd")

	// Store topology node for SWITCH
	fmt.Println("--- Storing Topology Node SWITCH ---")
	node := &topology.Node{
		Name: "SWITCH",
		Type: topology.NodeRole_BRIDGE,
		DeviceInfo: &topology.DeviceInfo{
			DeviceModel: "1",
		},
		ManagementInfo: &topology.ManagementInfo{
			IpAddress:      "192.168.4.64",
			ManagementPort: 830,
			Protocol:       topology.ManagementProtocol_NETCONF,
			Transport:      topology.ManagementTransport_SSH,
		},
		Ports: []*topology.Port{
			{
				Id:             "PORT_0",
				Name:           "PORT_0",
				NumberOfQueues: 8,
				Capabilities: &topology.InterfaceCapabilities{
					PortSpeed: 1000,
				},
			},
			{
				Id:             "PORT_1",
				Name:           "PORT_1",
				NumberOfQueues: 8,
				Capabilities: &topology.InterfaceCapabilities{
					PortSpeed: 1000,
				},
			},
			{
				Id:             "PORT_2",
				Name:           "PORT_2",
				NumberOfQueues: 8,
				Capabilities: &topology.InterfaceCapabilities{
					PortSpeed: 1000,
				},
			},
			{
				Id:             "PORT_3",
				Name:           "PORT_3",
				NumberOfQueues: 8,
				Capabilities: &topology.InterfaceCapabilities{
					PortSpeed: 1000,
				},
			},
		},
	}

	if err := store.StoreNode(node); err != nil {
		fmt.Printf("Failed to store node SWITCH: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Successfully stored node SWITCH into etcd")

	// Verify topology retrieval
	topo, err := store.GetTopology()
	if err != nil {
		fmt.Printf("Failed to retrieve topology: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Verified topology retrieval: %d nodes\n", len(topo.Nodes))
	for _, n := range topo.Nodes {
		fmt.Printf("  Node: Name=%s, Type=%v, Model=%s, IP=%s, Ports=%d\n",
			n.Name, n.Type, n.GetDeviceInfo().GetDeviceModel(),
			n.GetManagementInfo().GetIpAddress(), len(n.Ports))
	}

	credList, err := store.GetCredentialsList()
	if err != nil {
		fmt.Printf("Failed to get credentials list: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Verified credentials list: %d entries\n", len(credList))
	for _, c := range credList {
		fmt.Printf("  Cred: ID=%s, User=%s\n", c.ID, c.Credentials.GetUsernamePassword().GetUsername())
	}
}
