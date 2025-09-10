package protocolbackends

import (
	"managementSessions"
	"plugins"
	"structures"

	"github.com/golang/protobuf/proto"
)

type ProtocolBackend interface {
	Name() string
	AddPlugin(p plugins.Plugin)
	GetPlugin(feature string) (plugins.Plugin, bool)
	MapAndPush(msg proto.Message, model *structures.DeviceModel, target managementSessions.DeviceTarget) error
	SupportedFeatures() []string
}
