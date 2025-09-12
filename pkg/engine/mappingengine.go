package engine

import (
	"structures"
)

type MappingEngine interface {
	ApplyTopologyConfig(config *structures.TopologyConfig) error
}
