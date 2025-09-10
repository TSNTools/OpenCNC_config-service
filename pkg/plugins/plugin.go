package plugins

import (
	managementSessions "OpenCNC_config_service/pkg/managementSessions"
	devicemodelregistry "OpenCNC_config_service/pkg/structures/devicemodelregistry"

	"github.com/golang/protobuf/proto"
)

type Plugin interface {
	Name() string
	FeatureName() string
	SupportedByDevice(model *devicemodelregistry.DeviceModel) bool // returns true if the feature is supported by the device model
	Supports(msg proto.Message) bool                               // returns true if the message can be mapped by this features
	Map(msg proto.Message) (any, error)
	Push(mapped any, target managementSessions.DeviceTarget) error
}
