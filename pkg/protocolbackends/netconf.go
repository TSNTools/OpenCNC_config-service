package protocolbackends

import (
	"fmt"

	"managementSessions"
	"plugins"
	"structures"

	"github.com/golang/protobuf/proto"
)

type NetconfBackend struct {
	plugins map[string]plugins.Plugin
}

func NewNetconfBackend() *NetconfBackend {
	return &NetconfBackend{
		plugins: make(map[string]plugins.Plugin),
	}
}

func (n *NetconfBackend) Name() string {
	return "netconf"
}

func (n *NetconfBackend) AddPlugin(p plugins.Plugin) {
	n.plugins[p.FeatureName()] = p
}

func (n *NetconfBackend) GetPlugin(feature string) (plugins.Plugin, bool) {
	p, ok := n.plugins[feature]
	return p, ok
}

func (n *NetconfBackend) SupportedFeatures() []string {
	keys := make([]string, 0, len(n.plugins))
	for k := range n.plugins {
		keys = append(keys, k)
	}
	return keys
}

func (n *NetconfBackend) MapAndPush(msg proto.Message, model *structures.DeviceModel, target managementSessions.DeviceTarget) error {
	for _, plugin := range n.plugins {
		if plugin.Supports(msg) {
			mapped, err := plugin.Map(msg, model)
			if err != nil {
				return fmt.Errorf("plugin %s mapping failed: %w", plugin.Name(), err)
			}

			if err := plugin.Push(mapped, target); err != nil {
				return fmt.Errorf("plugin %s push failed: %w", plugin.Name(), err)
			}

			return nil
		}
	}

	return fmt.Errorf("no netconf plugin supports message: %T", msg)
}
