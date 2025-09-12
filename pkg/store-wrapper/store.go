package storewrapper

/*
Running etcd in the Background
If you want to run etcd in the background as a service, you can add the & at the end of the command:
etcd --name node1 \
     --data-dir /var/lib/etcd \
     --listen-client-urls http://localhost:2379 \
     --advertise-client-urls http://localhost:2379

To stop etcd:
	sudo systemctl stop etcd

To read using etcdctl:
	etcdctl get "" --prefix
	etcdctl put $KEY $VALUE
	etcdctl del $KEY

*/

import (
	devicemodelregistry "config-service/pkg/structures/device-model-registry"
	moduleregistry "config-service/pkg/structures/module-registry"
	"config-service/pkg/structures/topology"

	"git.cs.kau.se/hamzchah/opencnc_kafka-exporter/logger/pkg/logger"
	"google.golang.org/protobuf/proto"
)

var log = logger.GetLogger()

// Changed from model plugin to device model registry
func GetDeviceModelRegistry() (*devicemodelregistry.DeviceModelRegistry, error) {

	// Build the prefix for the request data
	prefix := "device-models."

	rawData, err := getFromStoreWithPrefix(prefix)
	if err != nil {
		log.Errorf("Failed getting device model from store: %v", err)
		return &devicemodelregistry.DeviceModelRegistry{}, err
	}

	var dregistry = &devicemodelregistry.DeviceModelRegistry{}

	for _, model := range rawData.Kvs {
		var dmodel = &devicemodelregistry.DeviceModel{}
		if err = proto.Unmarshal([]byte(model.Value), dmodel); err != nil {
			log.Errorf("Failed unmarshaling device model: %v", err)
			return &devicemodelregistry.DeviceModelRegistry{}, err
		}
		dregistry.DeviceModels = append(dregistry.DeviceModels, dmodel)
	}

	return dregistry, nil
}

func GetDeviceModel(name string) (*devicemodelregistry.DeviceModel, error) {
	// Build the URN for the request data
	urn := "device-models." + name

	// Send request to specific path in k/v store "device-models"
	rawData, err := getFromStore(urn)
	if err != nil {
		log.Errorf("Failed getting request data from store: %v", err)
		return &devicemodelregistry.DeviceModel{}, err
	}

	var model = &devicemodelregistry.DeviceModel{}
	if err = proto.Unmarshal(rawData, model); err != nil {
		log.Errorf("Failed unmarshaling schedule: %v", err)
		return &devicemodelregistry.DeviceModel{}, err
	}
	return model, nil
}

func GetTopology() (*topology.Topology, error) {
	var topo = &topology.Topology{}

	endnodes := getNodes("endnodes")
	bridges := getNodes("bridges")

	topo.Nodes = append(endnodes, bridges...)

	links := getLinks("links")

	topo.Links = append(topo.Links, links...)

	return topo, nil
}

func GetModuleRegistry() (*moduleregistry.ModuleRegistry, error) {
	// Build the URN for the request data
	urn := "yang-modules."

	rawData, err := getFromStore(urn)
	if err != nil {
		log.Errorf("Failed getting request data from store: %v", err)
		return &moduleregistry.ModuleRegistry{}, err
	}

	var mregistry = &moduleregistry.ModuleRegistry{}
	if err = proto.Unmarshal(rawData, mregistry); err != nil {
		log.Errorf("Failed unmarshaling schedule: %v", err)
		return &moduleregistry.ModuleRegistry{}, err
	}
	return mregistry, nil
}
