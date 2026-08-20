package engine

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"sync"

	"OpenCNC_config_service/common/observability"
	"OpenCNC_config_service/common/structures/topology"
	"OpenCNC_config_service/common/structures/topology_config"
	protocolbackends "OpenCNC_config_service/config_service/pkg/protocolbackends"
)

type Operation struct {
	Node      *topology.Node
	Config    *topology_config.NodeConfig
	Backend   protocolbackends.ProtocolBackend
	Prepared  bool
	Committed bool
}

type ConfigurationTransaction struct {
	ConfigId   string
	Operations []Operation
}

// getMaxWorkers determines the concurrent worker limit based on CPU cores
// or an optional MAX_WORKERS environment variable override.
func getMaxWorkers() (int, error) {
	maxWorkers := runtime.NumCPU() * 2

	if envVal := os.Getenv("MAX_WORKERS"); envVal != "" {
		parsedVal, err := strconv.Atoi(envVal)
		if err != nil || parsedVal <= 0 {
			return 0, fmt.Errorf("invalid MAX_WORKERS value: %q", envVal)
		}
		maxWorkers = parsedVal
	}

	if maxWorkers < 1 {
		maxWorkers = 1
	}

	return maxWorkers, nil
}
func (t *ConfigurationTransaction) Commit() error {
	var wg sync.WaitGroup
	var errMu sync.Mutex
	var commitErrs []error
	maxWorkers, err := getMaxWorkers()
	if err != nil {
		return err
	}

	sem := make(chan struct{}, maxWorkers) // create the semaphore with the calculated limit

	for i := range t.Operations {

		op := &t.Operations[i]
		if !op.Prepared {
			continue
		}
		wg.Add(1)
		// Acquire a token
		sem <- struct{}{}
		go func(idx int) {
			defer wg.Done()
			// Release the token when this goroutine is done
			defer func() { <-sem }()
			op := &t.Operations[idx]
			if err := op.Backend.Commit(op.Node); err != nil {
				errMu.Lock()
				commitErrs = append(commitErrs, fmt.Errorf("node %s: %w", op.Node.Name, err))
				errMu.Unlock()
				return
			}
			op.Committed = true
			op.Prepared = false
		}(i)
	}
	wg.Wait()

	if len(commitErrs) > 0 {
		rollbackErr := t.Rollback()
		if rollbackErr != nil {
			return fmt.Errorf("commit failed for %d nodes: %v. Failed to roll back previous commits", len(commitErrs), commitErrs)
		}
		return fmt.Errorf("commit failed for %d nodes: %v. Aborted transaction and rolled back previous commits", len(commitErrs), commitErrs)
	}

	return nil
}

func (t *ConfigurationTransaction) Rollback() error {

	var wg sync.WaitGroup
	var errMu sync.Mutex
	var rollbackErrs []error
	maxWorkers, err := getMaxWorkers()
	if err != nil {
		return err
	}
	sem := make(chan struct{}, maxWorkers) // create the semaphore with the calculated limit

	for i := len(t.Operations) - 1; i >= 0; i-- {

		op := &t.Operations[i]

		if !op.Committed {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int) {
			defer wg.Done()
			defer func() { <-sem }()
			op := &t.Operations[idx]
			if err := op.Backend.Rollback(op.Node); err != nil {
				errMu.Lock()
				rollbackErrs = append(rollbackErrs, fmt.Errorf("node %s: %w", op.Node.Name, err))
				errMu.Unlock()
				return
			}

			op.Committed = false
		}(i)
	}
	wg.Wait()

	if len(rollbackErrs) > 0 {
		return fmt.Errorf("rollback failed for %d nodes: %v", len(rollbackErrs), rollbackErrs)
	}

	return nil
}

func (t *ConfigurationTransaction) Prepare() error {
	var wg sync.WaitGroup
	var errMu sync.Mutex
	var errs []error
	maxWorkers, err := getMaxWorkers()
	if err != nil {
		return err
	}

	sem := make(chan struct{}, maxWorkers) //create the semaphore with the caluclated limit
	for i := range t.Operations {
		wg.Add(1)
		// Acquire a token
		sem <- struct{}{}
		go func(idx int) {
			defer wg.Done()
			//release the token ehrn this goroutine is done
			defer func() { <-sem }()
			op := &t.Operations[idx]
			//execcute the Netconf preparation for the node
			if err := op.Backend.PrepareSnapshot(op.Config, op.Node); err != nil {
				errMu.Lock()
				errs = append(errs, fmt.Errorf("node %s: %w", op.Node.Name, err))
				errMu.Unlock()
				// return immediately so op,prepared is not set to true
				return
			}
			op.Prepared = true
		}(i)
	}
	wg.Wait()

	if len(errs) > 0 {
		return fmt.Errorf("prepare failed for %d nodes: %v", len(errs), errs)
	}

	return nil
}

func NewConfigurationTransaction(configId string) *ConfigurationTransaction {
	return &ConfigurationTransaction{
		ConfigId: configId,
	}
}

//=================================
// definition of the MappingEngine
//=================================

// MappingEngine is the top-level orchestrator for applying a topology-wide configuration.
type MappingEngine struct {
	logger observability.Logger

	lastTransaction *ConfigurationTransaction // last applied configuration transaction

	backends map[topology.ManagementProtocol]protocolbackends.ProtocolBackend
}

func NewMappingEngine(logger observability.Logger) *MappingEngine {
	return &MappingEngine{
		logger:   observability.NormalizeLogger(logger),
		backends: make(map[topology.ManagementProtocol]protocolbackends.ProtocolBackend),
	}
}

func (m *MappingEngine) RegisterBackend(backend protocolbackends.ProtocolBackend) {
	m.backends[backend.Protocol()] = backend
}

func (m *MappingEngine) GetLastTransactionId() *string {
	if m.lastTransaction == nil {
		return nil
	}
	return &m.lastTransaction.ConfigId
}

func (m *MappingEngine) ApplyConfiguration(topo *topology.Topology, cfg *topology_config.TopologyConfig, secret string) error {
	if topo == nil || cfg == nil {
		return fmt.Errorf("topology and config must not be nil")
	}

	tx := NewConfigurationTransaction(cfg.GetConfigId())

	for _, node := range topo.Nodes {
		if node == nil || node.ManagementInfo == nil {
			continue
		}

		nodeCfg := findNodeConfig(cfg, node.Name)
		if nodeCfg == nil {
			continue
		}

		backend, ok := m.backends[node.ManagementInfo.Protocol]
		if !ok {
			if m.logger != nil {
				m.logger.Printf("no backend registered for protocol %v", node.ManagementInfo.Protocol)
			}
			continue
		}

		tx.Operations = append(tx.Operations, Operation{
			Node:    node,
			Config:  nodeCfg,
			Backend: backend,
		})
	}

	if err := tx.Prepare(); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	// transaction promotion: update the current and previous transaction IDs
	m.lastTransaction = tx

	// TODO:
	// Persist the new configuration in the KV store only after all
	// backends have successfully committed.

	return nil
}

func findNodeConfig(cfg *topology_config.TopologyConfig, nodeName string) *topology_config.NodeConfig {
	for _, nodeCfg := range cfg.GetNodeConfigs() {
		if nodeCfg != nil && nodeCfg.GetNodeId() == nodeName {
			return nodeCfg
		}
	}
	return nil
}

func (m *MappingEngine) Rollback() error {

	if m.lastTransaction == nil {
		return fmt.Errorf("transaction rollback is available only after a successful configuration transaction!!")
	}

	if err := m.lastTransaction.Rollback(); err != nil {
		return err
	}

	m.lastTransaction = nil

	return nil
}

/*
func (m *MappingEngine) findPortConfig(
	nodeCfg *topology_config.NodeConfig,
	portID string,
) *topology_config.PortConfig {
	if nodeCfg == nil {
		return nil
	}

	for _, portCfg := range nodeCfg.GetPortConfigs() {
		if portCfg != nil && portCfg.GetPortId() == portID {
			return portCfg
		}
	}

	return nil
}


		if err := backend.MapAndPush(portCfg, target); err != nil {
			if m.logger != nil {
				m.logger.Printf(
					"failed to apply config to %s/%s: %v",
					node.Name,
					port.Name,
					err,
				)
			}
			return err
		}
	}

*/
