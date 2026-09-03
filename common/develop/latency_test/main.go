package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	devicemodelregistry "OpenCNC/common/structures/devicemodelregistry"
	"OpenCNC/common/structures/topology_config"
	"OpenCNC/common/structures/vlan"
	service "OpenCNC/config_service/grpc_server"

	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// Record holds latency data for a single gRPC configuration request
type Record struct {
	Iteration       int
	VlanID          uint32
	NumPorts        int
	Ports           string
	TaggedMode      string
	Status          string
	LatencyMs       float64
	LatencyUs       int64
	ResponseMessage string
	Timestamp       string
}

func main() {
	// =========================================================================
	// Configuration & CLI Flags
	// Easily extensible via command-line flags or by modifying defaults below:
	//   2 ports: -ports PORT_1,PORT_2
	//   3 ports: -ports PORT_2,PORT_3,PORT_4
	//   4 ports: -ports PORT_1,PORT_2,PORT_3,PORT_4
	// =========================================================================
	grpcAddr := flag.String("addr", "localhost:5150", "gRPC server address (host:port)")
	nodeID := flag.String("node", "SWITCH", "Target bridge node name in topology")
	startVID := flag.Uint("start-vid", 2, "Start VLAN ID (inclusive)")
	endVID := flag.Uint("end-vid", 52, "End VLAN ID (inclusive)")
	portsFlag := flag.String("ports", "PORT_1,PORT_2", "Comma-separated list of ports (e.g. PORT_0,PORT_1 or PORT_0,PORT_1,PORT_2 or PORT_0,PORT_1,PORT_2,PORT_3)")
	untagged := flag.Bool("untagged", true, "Transmit VLAN frames untagged")
	delayMs := flag.Int("delay", 10000, "Delay in milliseconds between successive gRPC requests")
	timeoutSec := flag.Int("timeout", 30, "Per-request gRPC timeout in seconds")
	outputCSV := flag.String("csv", "", "Output CSV filename (leave empty for auto-generated relevant name)")
	databaseID := flag.Uint("db-id", 1, "Filtering database ID")
	saveModels := flag.Bool("save-models", true, "Save device model(s) to localhost etcd before running latency test")
	saveOnly := flag.Bool("save-only", false, "Only save device models to etcd and exit (do not run gRPC benchmark)")
	listModels := flag.Bool("list-models", false, "List device models currently stored in etcd and exit")
	modelsDir := flag.String("models-dir", "", "Directory containing device model JSON files (default auto-detected: common/deviceModels)")
	modelFile := flag.String("model-file", "", "Specific device model JSON file to save to etcd (e.g. common/deviceModels/1_device_model.json)")
	etcdAddr := flag.String("etcd-addr", "http://127.0.0.1:2379", "etcd server endpoint (host:port)")

	flag.Parse()

	// Handle -list-models standalone action
	if *listModels {
		if err := listEtcdDeviceModels(*etcdAddr); err != nil {
			log.Fatalf("Error listing device models from etcd: %v", err)
		}
		return
	}

	// Handle -save-only standalone action
	if *saveOnly {
		fmt.Printf("Connecting to etcd (%s) to save device models...\n", *etcdAddr)
		if err := ensureDeviceModelsInEtcd(*etcdAddr, *modelsDir, *modelFile); err != nil {
			log.Fatalf("Failed to save device models to etcd: %v", err)
		}
		fmt.Println("[DeviceModel] All device models have been saved to etcd. Exiting.")
		return
	}

	// Parse configured ports
	ports := parsePorts(*portsFlag)
	if len(ports) == 0 {
		log.Fatalf("Error: at least one port must be specified in -ports")
	}

	if *startVID > *endVID {
		log.Fatalf("Error: -start-vid (%d) cannot be greater than -end-vid (%d)", *startVID, *endVID)
	}

	// Generate relevant CSV file name if not explicitly provided
	csvFilename := *outputCSV
	if csvFilename == "" {
		timestamp := time.Now().Format("20060102_150405")
		taggedStr := "untagged"
		if !*untagged {
			taggedStr = "tagged"
		}
		csvFilename = fmt.Sprintf("vlan_bridge_latency_%dports_vid%d_to_%d_%s_%s.csv",
			len(ports), *startVID, *endVID, taggedStr, timestamp)
	}

	// Ensure destination directory exists
	dir := filepath.Dir(csvFilename)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Fatalf("Failed to create directory %s: %v", dir, err)
		}
	}

	csvFile, err := os.Create(csvFilename)
	if err != nil {
		log.Fatalf("Failed to create CSV file %s: %v", csvFilename, err)
	}
	defer csvFile.Close()

	csvWriter := csv.NewWriter(csvFile)
	defer csvWriter.Flush()

	// Write CSV Header
	header := []string{
		"iteration",
		"vlan_id",
		"num_ports",
		"ports",
		"tagged_mode",
		"status",
		"latency_ms",
		"latency_us",
		"response_message",
		"timestamp",
	}
	if err := csvWriter.Write(header); err != nil {
		log.Fatalf("Failed to write CSV header: %v", err)
	}
	csvWriter.Flush()

	fmt.Println("================================================================")
	fmt.Println("            OpenCNC VLAN Bridge Latency Benchmark              ")
	fmt.Println("================================================================")
	fmt.Printf(" Target gRPC Server : %s\n", *grpcAddr)
	fmt.Printf(" Target Bridge Node : %s\n", *nodeID)
	fmt.Printf(" VLAN ID Range      : %d to %d (total: %d requests)\n", *startVID, *endVID, *endVID-*startVID+1)
	fmt.Printf(" Configured Ports   : %v (%d ports)\n", ports, len(ports))
	fmt.Printf(" Transmission Mode  : Untagged=%t\n", *untagged)
	fmt.Printf(" Request Timeout    : %ds\n", *timeoutSec)
	fmt.Printf(" Inter-request Delay: %dms\n", *delayMs)
	fmt.Printf(" Output CSV File    : %s\n", csvFilename)
	fmt.Printf(" Etcd Endpoint      : %s (Save Models: %t)\n", *etcdAddr, *saveModels)
	fmt.Println("================================================================")

	// Ensure device models are saved into etcd in the format expected by config-service
	if *saveModels {
		fmt.Println("\n[DeviceModel] Checking and populating device models in etcd...")
		if err := ensureDeviceModelsInEtcd(*etcdAddr, *modelsDir, *modelFile); err != nil {
			log.Printf("[DeviceModel] WARNING: %v\n", err)
			log.Printf("[DeviceModel] Note: If config-service requires the device model from etcd, requests may fail.\n\n")
		} else {
			fmt.Println("[DeviceModel] Device models verified in etcd.\n")
		}
	}

	// Establish gRPC connection to config-service
	conn, err := grpc.NewClient(
		*grpcAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("Failed to connect to gRPC server at %s: %v", *grpcAddr, err)
	}
	defer conn.Close()

	client := service.NewConfigServiceClient(conn)

	// Quick health check ping
	pingCtx, pingCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer pingCancel()
	if _, err := client.Ping(pingCtx, &service.ConfigurationRequest{}); err != nil {
		log.Printf("Warning: Ping check failed (service might be busy or unreachable): %v\n", err)
	} else {
		fmt.Println("gRPC connection verified (Ping successful). Starting latency benchmark...\n")
	}

	var records []Record
	var latenciesMs []float64
	successCount := 0
	failCount := 0
	iteration := 0

	// Determine tagged mode string
	taggedModeStr := "UNTAGGED"
	if !*untagged {
		taggedModeStr = "TAGGED"
	}
	portsStr := strings.Join(ports, ";")

	// =========================================================================
	// Benchmark Loop: VLAN IDs startVID to endVID
	// =========================================================================
	for vid := uint32(*startVID); vid <= uint32(*endVID); vid++ {
		iteration++

		// Build VLAN bridge configuration request
		req := buildConfigurationRequest(*nodeID, vid, uint32(*databaseID), ports, *untagged)

		// Create per-request timeout context
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*timeoutSec)*time.Second)
		reqTimestamp := time.Now().Format(time.RFC3339Nano)

		// -------------------------------------------------------------
		// Measure Latency: strictly between gRPC send and response
		// -------------------------------------------------------------
		sendTime := time.Now()
		resp, err := client.ApplyConfiguration(ctx, req)
		latency := time.Since(sendTime)
		cancel()

		latencyMs := float64(latency.Microseconds()) / 1000.0
		latencyUs := latency.Microseconds()

		status := "FAILED"
		respMsg := ""
		if err != nil {
			failCount++
			respMsg = err.Error()
			fmt.Printf("[%3d/%3d] VID: %-4d | %s | Latency: %8.3f ms | FAIL: %v\n",
				iteration, (*endVID - *startVID + 1), vid, portsStr, latencyMs, respMsg)
		} else if resp != nil && !resp.GetSuccess() {
			failCount++
			respMsg = resp.GetMessage()
			fmt.Printf("[%3d/%3d] VID: %-4d | %s | Latency: %8.3f ms | FAIL (server): %s\n",
				iteration, (*endVID - *startVID + 1), vid, portsStr, latencyMs, respMsg)
		} else {
			successCount++
			status = "SUCCESS"
			if resp != nil {
				respMsg = resp.GetMessage()
			}
			latenciesMs = append(latenciesMs, latencyMs)
			fmt.Printf("[%3d/%3d] VID: %-4d | %s | Latency: %8.3f ms | SUCCESS: %s\n",
				iteration, (*endVID - *startVID + 1), vid, portsStr, latencyMs, respMsg)
		}

		// Clean up newline or comma characters for clean CSV formatting
		cleanedMsg := strings.ReplaceAll(respMsg, "\n", " ")
		cleanedMsg = strings.ReplaceAll(cleanedMsg, "\r", " ")

		record := Record{
			Iteration:       iteration,
			VlanID:          vid,
			NumPorts:        len(ports),
			Ports:           portsStr,
			TaggedMode:      taggedModeStr,
			Status:          status,
			LatencyMs:       latencyMs,
			LatencyUs:       latencyUs,
			ResponseMessage: cleanedMsg,
			Timestamp:       reqTimestamp,
		}
		records = append(records, record)

		// Write record immediately to CSV
		row := []string{
			strconv.Itoa(record.Iteration),
			strconv.FormatUint(uint64(record.VlanID), 10),
			strconv.Itoa(record.NumPorts),
			record.Ports,
			record.TaggedMode,
			record.Status,
			fmt.Sprintf("%.3f", record.LatencyMs),
			strconv.FormatInt(record.LatencyUs, 10),
			record.ResponseMessage,
			record.Timestamp,
		}
		if err := csvWriter.Write(row); err != nil {
			log.Printf("Warning: failed to write CSV row for VID %d: %v", vid, err)
		}
		csvWriter.Flush()

		// Optional delay between requests
		if *delayMs > 0 && vid < uint32(*endVID) {
			time.Sleep(time.Duration(*delayMs) * time.Millisecond)
		}
	}

	// =========================================================================
	// Statistical Summary
	// =========================================================================
	fmt.Println("\n================================================================")
	fmt.Println("                       Benchmark Summary                        ")
	fmt.Println("================================================================")
	fmt.Printf(" Total Requests  : %d\n", len(records))
	fmt.Printf(" Successful      : %d\n", successCount)
	fmt.Printf(" Failed          : %d\n", failCount)

	if len(latenciesMs) > 0 {
		sort.Float64s(latenciesMs)
		min := latenciesMs[0]
		max := latenciesMs[len(latenciesMs)-1]
		sum := 0.0
		for _, l := range latenciesMs {
			sum += l
		}
		mean := sum / float64(len(latenciesMs))
		median := percentile(latenciesMs, 50)
		p95 := percentile(latenciesMs, 95)
		p99 := percentile(latenciesMs, 99)

		fmt.Println("----------------------------------------------------------------")
		fmt.Printf(" Min Latency     : %8.3f ms\n", min)
		fmt.Printf(" Max Latency     : %8.3f ms\n", max)
		fmt.Printf(" Mean Latency    : %8.3f ms\n", mean)
		fmt.Printf(" Median (P50)    : %8.3f ms\n", median)
		fmt.Printf(" 95th Percentile : %8.3f ms\n", p95)
		fmt.Printf(" 99th Percentile : %8.3f ms\n", p99)
	}
	fmt.Println("----------------------------------------------------------------")
	fmt.Printf(" Results saved to: %s\n", csvFilename)
	fmt.Println("================================================================")
}

// buildConfigurationRequest creates a ConfigurationRequest for the specified VLAN and ports.
// To extend to 3 ports or 4 ports, simply pass additional ports into the ports slice.
func buildConfigurationRequest(nodeID string, vid uint32, databaseID uint32, ports []string, untagged bool) *service.ConfigurationRequest {
	// Determine VLAN transmission mode
	transMode := vlan.VlanTransmitted_VLAN_TRANSMITTED_UNTAGGED
	if !untagged {
		transMode = vlan.VlanTransmitted_VLAN_TRANSMITTED_TAGGED
	}

	// Build PortMaps for all configured ports (e.g. 2 ports, 3 ports, 4 ports)
	var portMaps []*vlan.VlanPortMap
	for _, port := range ports {
		portMaps = append(portMaps, &vlan.VlanPortMap{
			PortId:                port,
			RegistrarAdminControl: vlan.RegistrarAdminControl_REGISTRAR_ADMIN_CONTROL_NORMAL,
			VlanTransmitted:       transMode,
		})
	}

	// Build Bridge VLAN configuration
	bridgeVlanConfig := &vlan.BridgeVlanConfig{
		VlanRegistrationEntries: []*vlan.VlanRegistrationEntry{
			{
				DatabaseId: databaseID,
				VlanIds:    []uint32{vid},
				EntryType:  vlan.VlanRegistrationEntryType_VLAN_REG_ENTRY_TYPE_STATIC,
				PortMaps:   portMaps,
			},
		},
		VidToFidMappings: []*vlan.VidToFidMapping{
			{
				Vid: vid,
				Fid: vid, // maps VID to FID (Filtering Database ID)
			},
		},
	}

	configID := fmt.Sprintf("vlan-config-%d-vid-%d", len(ports), vid)

	return &service.ConfigurationRequest{
		Configuration: &topology_config.TopologyConfig{
			ConfigId: configID,
			NodeConfigs: []*topology_config.NodeConfig{
				{
					NodeId: nodeID,
					Bridge: &topology_config.BridgeConfig{
						VlanConfig: bridgeVlanConfig,
					},
				},
			},
		},
	}
}

// parsePorts trims whitespace and returns clean port names
func parsePorts(input string) []string {
	var result []string
	parts := strings.Split(input, ",")
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// percentile calculates the p-th percentile from a sorted slice of float64s
func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0.0
	}
	if p <= 0 {
		return sorted[0]
	}
	if p >= 100 {
		return sorted[len(sorted)-1]
	}
	index := (p / 100.0) * float64(len(sorted)-1)
	lower := int(index)
	upper := lower + 1
	weight := index - float64(lower)
	if upper >= len(sorted) {
		return sorted[lower]
	}
	return sorted[lower]*(1.0-weight) + sorted[upper]*weight
}

// listEtcdDeviceModels queries etcd for all stored device models and prints them
func listEtcdDeviceModels(etcdAddr string) error {
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{etcdAddr},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return fmt.Errorf("failed to connect to etcd at %s: %w", etcdAddr, err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.Get(ctx, "device-models/", clientv3.WithPrefix())
	if err != nil {
		return fmt.Errorf("failed to query etcd for device-models/: %w", err)
	}

	if len(resp.Kvs) == 0 {
		fmt.Println("[DeviceModel] No device models found in etcd under 'device-models/'.")
		return nil
	}

	fmt.Printf("[DeviceModel] Found %d device model(s) in etcd:\n", len(resp.Kvs))
	for i, kv := range resp.Kvs {
		var model devicemodelregistry.DeviceModel
		if err := proto.Unmarshal(kv.Value, &model); err != nil {
			fmt.Printf("  %d) Key: %s (Failed to unmarshal protobuf: %v)\n", i+1, string(kv.Key), err)
			continue
		}
		fmt.Printf("  %d) Model: %q | Vendor: %q | Version: %q | YANG Modules: %d | Key: %s\n",
			i+1, model.Name, model.Vendor, model.Version, len(model.YangFiles), string(kv.Key))
		for _, y := range model.YangFiles {
			fmt.Printf("      - %s (rev: %s)\n", y.Name, y.Revision)
		}
	}
	return nil
}

// ensureDeviceModelsInEtcd loads device model JSON files and stores them into etcd
func ensureDeviceModelsInEtcd(etcdAddr, modelsDir, modelFile string) error {
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{etcdAddr},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return fmt.Errorf("failed to connect to etcd: %w", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var filesToLoad []string
	if modelFile != "" {
		filesToLoad = append(filesToLoad, modelFile)
	} else {
		dir := findDeviceModelsDir(modelsDir)
		if dir != "" {
			entries, err := os.ReadDir(dir)
			if err == nil {
				for _, e := range entries {
					if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
						filesToLoad = append(filesToLoad, filepath.Join(dir, e.Name()))
					}
				}
			}
		}
	}

	if len(filesToLoad) == 0 {
		return fmt.Errorf("no device model files found")
	}

	savedTotal := 0
	for _, fpath := range filesToLoad {
		models, err := loadDeviceModelsFromFile(fpath)
		if err != nil {
			log.Printf("[DeviceModel] Warning: skipping %s: %v", fpath, err)
			continue
		}
		for _, model := range models {
			if err := saveSingleDeviceModel(ctx, client, model); err != nil {
				return fmt.Errorf("failed to save model %s from %s: %w", model.Name, fpath, err)
			}
			savedTotal++
			fmt.Printf("[DeviceModel] Saved model %q (Vendor: %s, Version: %s, YANG modules: %d) -> etcd key: device-models/%s (from %s)\n",
				model.Name, model.Vendor, model.Version, len(model.YangFiles), model.Name, filepath.Base(fpath))
		}
	}

	if savedTotal == 0 {
		return fmt.Errorf("no device models could be loaded and saved from files: %v", filesToLoad)
	}

	return nil
}

// saveSingleDeviceModel stores a DeviceModel in etcd using Protobuf binary wire format
// under key "device-models/<name>", which is the exact schema config-service expects.
func saveSingleDeviceModel(ctx context.Context, client *clientv3.Client, model *devicemodelregistry.DeviceModel) error {
	if model == nil || strings.TrimSpace(model.Name) == "" {
		return fmt.Errorf("invalid device model: name is empty")
	}

	// Format expected by config service:
	// Key: "device-models/<name>"
	// Value: Protobuf binary serialization (proto.Marshal(model))
	key := fmt.Sprintf("device-models/%s", model.Name)

	wireData, err := proto.Marshal(model)
	if err != nil {
		return fmt.Errorf("failed to protobuf marshal device model %s: %w", model.Name, err)
	}

	putCtx, putCancel := context.WithTimeout(ctx, 3*time.Second)
	defer putCancel()
	if _, err := client.Put(putCtx, key, string(wireData)); err != nil {
		return fmt.Errorf("etcd put failed for key %s: %w", key, err)
	}

	// Verification: read back and unmarshal exactly as config service does
	getCtx, getCancel := context.WithTimeout(ctx, 3*time.Second)
	defer getCancel()
	resp, err := client.Get(getCtx, key)
	if err != nil {
		return fmt.Errorf("failed to read back key %s from etcd: %w", key, err)
	}
	if len(resp.Kvs) == 0 {
		return fmt.Errorf("key %s not found in etcd after put", key)
	}

	var verified devicemodelregistry.DeviceModel
	if err := proto.Unmarshal(resp.Kvs[0].Value, &verified); err != nil {
		return fmt.Errorf("unmarshal verification failed for %s (corrupt wire data): %w", key, err)
	}

	return nil
}

// findDeviceModelsDir locates the common/deviceModels directory
func findDeviceModelsDir(preferredDir string) string {
	if preferredDir != "" {
		if fi, err := os.Stat(preferredDir); err == nil && fi.IsDir() {
			return preferredDir
		}
	}

	candidates := []string{
		"common/deviceModels",
		"../../deviceModels",
		"../deviceModels",
		"/home/jittin/config-service/new/OpenCNC_config-service/common/deviceModels",
	}

	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && fi.IsDir() {
			return c
		}
	}
	return ""
}

type rawDeviceModelJSON struct {
	ModelName      string              `json:"model-name"`
	Name           string              `json:"name"`
	Vendor         string              `json:"vendor"`
	Version        string              `json:"version"`
	YangFiles      []rawYangModuleJSON `json:"yang-files"`
	YangFilesAlt   []rawYangModuleJSON `json:"yang_files"`
	YangFilesCamel []rawYangModuleJSON `json:"yangFiles"`
}

type rawYangModuleJSON struct {
	FileName    string `json:"file-name"`
	Name        string `json:"name"`
	Revision    string `json:"file-revision"`
	RevisionAlt string `json:"revision"`
	Description string `json:"description"`
	Structure   string `json:"structure"`
}

type rawWrapperJSON struct {
	DeviceModel    *rawDeviceModelJSON   `json:"device-model"`
	DeviceModelAlt *rawDeviceModelJSON   `json:"device_model"`
	DeviceModels   []*rawDeviceModelJSON `json:"device-models"`
}

// loadDeviceModelsFromFile parses a device model JSON file into DeviceModel structs
func loadDeviceModelsFromFile(fpath string) ([]*devicemodelregistry.DeviceModel, error) {
	data, err := os.ReadFile(fpath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", fpath, err)
	}

	// 1. First try parsing directly via protojson
	singleModel := &devicemodelregistry.DeviceModel{}
	if err := protojson.UnmarshalOptions{DiscardUnknown: true}.Unmarshal(data, singleModel); err == nil && singleModel.Name != "" {
		return []*devicemodelregistry.DeviceModel{singleModel}, nil
	}

	// 2. Try parsing wrapper JSON
	var wrapper rawWrapperJSON
	if err := json.Unmarshal(data, &wrapper); err == nil {
		if wrapper.DeviceModel != nil {
			return []*devicemodelregistry.DeviceModel{convertRawModel(wrapper.DeviceModel)}, nil
		}
		if wrapper.DeviceModelAlt != nil {
			return []*devicemodelregistry.DeviceModel{convertRawModel(wrapper.DeviceModelAlt)}, nil
		}
		if len(wrapper.DeviceModels) > 0 {
			var models []*devicemodelregistry.DeviceModel
			for _, rm := range wrapper.DeviceModels {
				if rm != nil {
					models = append(models, convertRawModel(rm))
				}
			}
			if len(models) > 0 {
				return models, nil
			}
		}
	}

	// 3. Try parsing raw single model JSON
	var raw rawDeviceModelJSON
	if err := json.Unmarshal(data, &raw); err == nil && (raw.ModelName != "" || raw.Name != "") {
		return []*devicemodelregistry.DeviceModel{convertRawModel(&raw)}, nil
	}

	return nil, fmt.Errorf("unrecognized device model format in %s", fpath)
}

func convertRawModel(raw *rawDeviceModelJSON) *devicemodelregistry.DeviceModel {
	name := raw.ModelName
	if name == "" {
		name = raw.Name
	}
	model := &devicemodelregistry.DeviceModel{
		Name:    name,
		Vendor:  raw.Vendor,
		Version: raw.Version,
	}

	var yangList []rawYangModuleJSON
	if len(raw.YangFiles) > 0 {
		yangList = raw.YangFiles
	} else if len(raw.YangFilesAlt) > 0 {
		yangList = raw.YangFilesAlt
	} else if len(raw.YangFilesCamel) > 0 {
		yangList = raw.YangFilesCamel
	}

	for _, y := range yangList {
		yName := y.FileName
		if yName == "" {
			yName = y.Name
		}
		yRev := y.Revision
		if yRev == "" {
			yRev = y.RevisionAlt
		}
		model.YangFiles = append(model.YangFiles, &devicemodelregistry.YangModule{
			Name:        yName,
			Revision:    yRev,
			Description: y.Description,
			Structure:   y.Structure,
		})
	}
	return model
}
