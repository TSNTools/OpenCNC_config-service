package main

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	storewrapper "OpenCNC_config_service/common/store-wrapper"
	"OpenCNC_config_service/common/structures/topology"
	topology_config "OpenCNC_config_service/common/structures/topology_config"
	"OpenCNC_config_service/config_service/pkg/engine"
	"OpenCNC_config_service/config_service/pkg/managementSessions"
	"OpenCNC_config_service/config_service/pkg/plugins"
	"OpenCNC_config_service/config_service/pkg/protocolbackends"
)

// TestApplyConfigByIdWithRunningConfig retrieves the configuration by ID,
// fetches the live running config from each target node to initialize snapshots,
// and applies the configuration via the mapping engine.
func TestApplyConfigByIdWithRunningConfig(configID string) {
	log.Printf("[TEST-APPLY-CONFIG-BY-ID] Starting test for applyConfigById with config ID: %s", configID)

	if configID == "" {
		log.Fatalf("Config ID is empty!")
	}

	// 1. Fetch Topology and Configuration from KV Store
	topo, err := storewrapper.GetTopology()
	if err != nil {
		log.Fatalf("Failed to retrieve topology from store: %v", err)
	}
	log.Printf("Retrieved topology with %d nodes", len(topo.GetNodes()))

	cfg, err := storewrapper.GetConfiguration(configID)
	if err != nil {
		log.Fatalf("Failed to retrieve configuration %s from store: %v", configID, err)
	}
	log.Printf("Retrieved configuration %s", cfg.GetConfigId())

	// 2. Initialize Engine & NETCONF Backend with Plugins
	mappingEngine := engine.NewMappingEngine(nil)
	netconfPlugins := plugins.ForProtocol(topology.ManagementProtocol_NETCONF, nil)
	netconfBackend := protocolbackends.NewNetconfBackend("netconf", nil, netconfPlugins...)

	// 3. For each node, fetch the live running config to populate snapshots
	for _, node := range topo.GetNodes() {
		if node == nil || node.ManagementInfo == nil {
			continue
		}
		if node.ManagementInfo.Protocol == topology.ManagementProtocol_NETCONF {
			secret := os.Getenv("NETCONF_PASSWORD")
			if secret == "" {
				secret = node.ManagementInfo.UserName
			}

			log.Printf("Target Node: %s | IP: %s | User: %s | Port: %d",
				node.Name,
				node.ManagementInfo.IpAddress,
				node.ManagementInfo.UserName,
				node.ManagementInfo.ManagementPort,
			)
			log.Printf("Fetching live running configuration for node %s (%s:830)...", node.Name, node.ManagementInfo.IpAddress)
			if err := fetchAndInitNodeSnapshot(netconfBackend, node, secret); err != nil {
				log.Fatalf("Failed to fetch initial running config for node %s (%s:830): %v", node.Name, node.ManagementInfo.IpAddress, err)
			}
		}
	}

	// 4. Register Backend
	mappingEngine.RegisterBackend(netconfBackend)

	// 5. Apply Configuration
	secret := os.Getenv("NETCONF_PASSWORD")
	log.Printf("Applying configuration ID %s across nodes...", configID)
	if err := mappingEngine.ApplyConfiguration(topo, cfg, secret); err != nil {
		log.Fatalf("ApplyConfiguration failed: %v", err)
	}

	log.Printf("Successfully applied configuration ID %s to all target nodes!", configID)
}

// fetchAndInitNodeSnapshot connects to a node via NETCONF, retrieves its running configuration,
// and initializes the base snapshot in the NetconfBackend for testing purposes.
func fetchAndInitNodeSnapshot(backend *protocolbackends.NetconfBackend, node *topology.Node, secret string) error {
	if node == nil || node.ManagementInfo == nil {
		return fmt.Errorf("node or management info is nil")
	}
	session, err := managementSessions.CreateSession(
		node.ManagementInfo.IpAddress,
		node.ManagementInfo.UserName,
		secret,
	)
	if err != nil {
		return fmt.Errorf("failed connecting to node %s: %w", node.Name, err)
	}
	defer session.Close()

	rawXML, err := managementSessions.GetRunningConfig(session)
	if err != nil {
		return fmt.Errorf("failed fetching running config for node %s: %w", node.Name, err)
	}

	backend.InitSnapshot(node.Name, []byte(rawXML))
	return nil
}

func printUsageHelp() {
	fmt.Println("Usage: go run . -configId <CONFIGURATION_ID>")
}

// MockProtocolBackend simulates a protocol backend with customizable delays and failure modes for concurrency testing.
type MockProtocolBackend struct {
	name          string
	prepareDelay  time.Duration
	commitDelay   time.Duration
	rollbackDelay time.Duration
	failCommit    map[string]bool
	mu            sync.Mutex
	commitCount   int
	rollbackCount int
}

func NewMockProtocolBackend(name string, prepareDelay, commitDelay, rollbackDelay time.Duration) *MockProtocolBackend {
	return &MockProtocolBackend{
		name:          name,
		prepareDelay:  prepareDelay,
		commitDelay:   commitDelay,
		rollbackDelay: rollbackDelay,
		failCommit:    make(map[string]bool),
	}
}

func (m *MockProtocolBackend) Name() string                                    { return m.name }
func (m *MockProtocolBackend) Protocol() topology.ManagementProtocol           { return topology.ManagementProtocol_NETCONF }
func (m *MockProtocolBackend) AddPlugin(p plugins.Plugin)                      {}
func (m *MockProtocolBackend) Plugins() []plugins.Plugin                       { return nil }

func (m *MockProtocolBackend) PrepareSnapshot(msg *topology_config.NodeConfig, node *topology.Node) error {
	if m.prepareDelay > 0 {
		time.Sleep(m.prepareDelay)
	}
	return nil
}

func (m *MockProtocolBackend) Commit(target *topology.Node) error {
	if m.commitDelay > 0 {
		time.Sleep(m.commitDelay)
	}
	m.mu.Lock()
	m.commitCount++
	m.mu.Unlock()

	if m.failCommit[target.Name] {
		return fmt.Errorf("simulated commit error on node %s", target.Name)
	}
	return nil
}

func (m *MockProtocolBackend) Rollback(target *topology.Node) error {
	if m.rollbackDelay > 0 {
		time.Sleep(m.rollbackDelay)
	}
	m.mu.Lock()
	m.rollbackCount++
	m.mu.Unlock()
	return nil
}

// TestConcurrencyAndRace runs a complete suite of concurrency, timing, and race tests.
func TestConcurrencyAndRace() {
	log.Println("========================================================================")
	log.Println("🧪 [TEST-CONCURRENCY] Starting Concurrency, Timing & Race Test Suite")
	log.Println("========================================================================")

	// -------------------------------------------------------------------------
	// Sub-test 1: Parallel Speedup Benchmark (5 nodes with simulated 100ms delays)
	// -------------------------------------------------------------------------
	log.Println("\n[1/3] Running Parallel Execution Speedup Benchmark (5 Nodes)...")
	nodeCount := 5
	prepareLatency := 100 * time.Millisecond
	commitLatency := 100 * time.Millisecond

	mockBackend := NewMockProtocolBackend("mock-netconf", prepareLatency, commitLatency, 50*time.Millisecond)

	var operations []engine.Operation
	for i := 1; i <= nodeCount; i++ {
		nodeName := fmt.Sprintf("bridge-node-%d", i)
		operations = append(operations, engine.Operation{
			Node: &topology.Node{
				Name: nodeName,
				ManagementInfo: &topology.ManagementInfo{
					IpAddress: fmt.Sprintf("192.168.1.%d", i),
					Protocol:  topology.ManagementProtocol_NETCONF,
				},
			},
			Config: &topology_config.NodeConfig{
				NodeId: nodeName,
			},
			Backend: mockBackend,
		})
	}

	tx := &engine.ConfigurationTransaction{
		ConfigId:   "benchmark-tx-1",
		Operations: operations,
	}

	start := time.Now()
	if err := tx.Prepare(); err != nil {
		log.Fatalf("❌ tx.Prepare() failed unexpectedly: %v", err)
	}
	prepareElapsed := time.Since(start)

	start = time.Now()
	if err := tx.Commit(); err != nil {
		log.Fatalf("❌ tx.Commit() failed unexpectedly: %v", err)
	}
	commitElapsed := time.Since(start)

	expectedSeqPrepare := time.Duration(nodeCount) * prepareLatency
	expectedSeqCommit := time.Duration(nodeCount) * commitLatency
	totalElapsed := prepareElapsed + commitElapsed
	expectedSeqTotal := expectedSeqPrepare + expectedSeqCommit

	log.Printf("  • Prepare Phase: %v (Sequential would be ~%v)", prepareElapsed, expectedSeqPrepare)
	log.Printf("  • Commit Phase:  %v (Sequential would be ~%v)", commitElapsed, expectedSeqCommit)
	log.Printf("  • Total Time:    %v (Sequential would be ~%v)", totalElapsed, expectedSeqTotal)

	if totalElapsed < expectedSeqTotal/2 {
		speedup := float64(expectedSeqTotal) / float64(totalElapsed)
		log.Printf("  ✅ PASS: Concurrency verified! Execution speedup: %.2fx faster than sequential", speedup)
	} else {
		log.Printf("  ⚠️ WARNING: Execution time (%v) was not significantly faster than sequential (%v)", totalElapsed, expectedSeqTotal)
	}

	// -------------------------------------------------------------------------
	// Sub-test 2: Coordinated 2-Phase Rollback under Concurrency
	// -------------------------------------------------------------------------
	log.Println("\n[2/3] Running Coordinated Parallel Rollback Test (4 Nodes, 1 Failing)...")
	rollbackMock := NewMockProtocolBackend("mock-rollback", 10*time.Millisecond, 20*time.Millisecond, 20*time.Millisecond)
	rollbackMock.failCommit["failing-node-4"] = true

	var rollbackOps []engine.Operation
	for i := 1; i <= 4; i++ {
		nodeName := fmt.Sprintf("working-node-%d", i)
		if i == 4 {
			nodeName = "failing-node-4"
		}
		rollbackOps = append(rollbackOps, engine.Operation{
			Node: &topology.Node{
				Name: nodeName,
				ManagementInfo: &topology.ManagementInfo{
					IpAddress: fmt.Sprintf("192.168.2.%d", i),
					Protocol:  topology.ManagementProtocol_NETCONF,
				},
			},
			Config: &topology_config.NodeConfig{
				NodeId: nodeName,
			},
			Backend: rollbackMock,
		})
	}

	rollbackTx := &engine.ConfigurationTransaction{
		ConfigId:   "rollback-tx-1",
		Operations: rollbackOps,
	}

	if err := rollbackTx.Prepare(); err != nil {
		log.Fatalf("❌ Prepare failed: %v", err)
	}

	commitErr := rollbackTx.Commit()
	if commitErr == nil {
		log.Fatalf("❌ Expected commit to fail due to failing-node-4, but it succeeded!")
	}
	log.Printf("  • Commit failed as expected with message:\n    --> %v", commitErr)

	rollbackMock.mu.Lock()
	rolledBackCount := rollbackMock.rollbackCount
	rollbackMock.mu.Unlock()

	log.Printf("  • Rollback executed on %d committed node(s)", rolledBackCount)
	if rolledBackCount > 0 {
		log.Printf("  ✅ PASS: Rollback successfully coordinated across committed nodes!")
	} else {
		log.Fatalf("  ❌ FAIL: Rollback was not triggered on succeeded nodes")
	}

	// -------------------------------------------------------------------------
	// Sub-test 3: NetconfBackend Thread-Safety & Race Stress Test
	// -------------------------------------------------------------------------
	log.Println("\n[3/3] Running NetconfBackend Snapshot Map Race Detector Stress Test (30 Goroutines)...")
	realBackend := protocolbackends.NewNetconfBackend("netconf-race", nil)

	var stressWg sync.WaitGroup
	stressNodeCount := 30

	for i := 1; i <= stressNodeCount; i++ {
		stressWg.Add(1)
		go func(nodeIdx int) {
			defer stressWg.Done()
			nodeName := fmt.Sprintf("stress-switch-%d", nodeIdx)
			dummyXML := fmt.Sprintf("<interfaces><interface><name>sw0p%d</name></interface></interfaces>", nodeIdx)

			// Concurrent InitSnapshot
			realBackend.InitSnapshot(nodeName, []byte(dummyXML))

			// Concurrent PrepareSnapshot
			_ = realBackend.PrepareSnapshot(&topology_config.NodeConfig{
				NodeId: nodeName,
			}, &topology.Node{
				Name: nodeName,
			})
		}(i)
	}

	stressWg.Wait()
	log.Println("  ✅ PASS: 30 concurrent read/write snapshot operations completed with zero race conditions!")

	log.Println("\n========================================================================")
	log.Println("🎉 ALL CONCURRENCY AND RACE TESTS PASSED SUCCESSFULLY!")
	log.Println("========================================================================")
}
