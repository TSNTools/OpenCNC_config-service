package netconf

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"

	opencncModel "OpenCNC_config_service/opencnc_model"
	"OpenCNC_config_service/pkg/managementSessions"
	"OpenCNC_config_service/pkg/plugins"
	devicemodelregistry "OpenCNC_config_service/pkg/structures/devicemodelregistry"
	qbv "OpenCNC_config_service/pkg/structures/qbv"
	utils "OpenCNC_config_service/pkg/utils"

	"github.com/golang/protobuf/proto"
	"github.com/openconfig/ygot/ygot"
)

// Ensure it implements the Plugin interface
var _ plugins.Plugin = (*QbvNetconfPlugin)(nil)

type QbvNetconfPlugin struct {
	logger *log.Logger
}

func NewQbvNetconfPlugin(logger *log.Logger) *QbvNetconfPlugin {
	return &QbvNetconfPlugin{logger: logger}
}

func (p *QbvNetconfPlugin) Name() string {
	return "qbv-netconf"
}

func (p *QbvNetconfPlugin) FeatureName() string {
	return "qbv"
}

func (p *QbvNetconfPlugin) SupportedByDevice(model *devicemodelregistry.DeviceModel) bool {
	required_Yangs := []string{"ieee802-dot1q-sched.yang", "ieee802-dot1q-sched-bridge.yang"}

	present := make(map[string]bool)
	for _, yf := range model.YangFiles {
		present[yf.Name] = true
	}

	for _, req := range required_Yangs {
		if !present[req] {
			return false
		}
	}

	return true
}

func (p *QbvNetconfPlugin) Supports(msg proto.Message) bool {
	// returns true if the message can be mapped by this features
	_, ok := msg.(*qbv.GateControlList)
	return ok
}

func (p *QbvNetconfPlugin) Map(msg proto.Message) (any, error) {
	gcl, ok := msg.(*qbv.GateControlList)
	if !ok {
		return nil, fmt.Errorf("invalid message type for QbvNetconfPlugin: %T", msg)
	}

	p.logger.Printf("[Qbv] Mapping schedule ID: %s", gcl.GetScheduleId())

	// Mapping GateControlList to YGOT IETFInterfaces_Interfaces_Interface_BridgePort_GateParameterTable
	ygotGcl := &opencncModel.IETFInterfaces_Interfaces_Interface_BridgePort_GateParameterTable{
		GateEnabled: ygot.Bool(gcl.GetAdminState() == qbv.AdminState_ENABLED),

		AdminBaseTime: &opencncModel.IETFInterfaces_Interfaces_Interface_BridgePort_GateParameterTable_AdminBaseTime{
			Seconds:     ygot.Uint64(gcl.GetBaseTime() / 1e9),
			Nanoseconds: ygot.Uint32(uint32(gcl.GetBaseTime() % 1e9)),
		},

		AdminCycleTime: &opencncModel.IETFInterfaces_Interfaces_Interface_BridgePort_GateParameterTable_AdminCycleTime{
			Numerator:   ygot.Uint32(uint32(gcl.GetCycleTime())), // already ns
			Denominator: ygot.Uint32(1000000000),                 // 1e9 ns per second
		},

		AdminControlList: &opencncModel.IETFInterfaces_Interfaces_Interface_BridgePort_GateParameterTable_AdminControlList{
			GateControlEntry: make(map[uint32]*opencncModel.IETFInterfaces_Interfaces_Interface_BridgePort_GateParameterTable_AdminControlList_GateControlEntry),
		},
	}

	// Constants for the OperationName enum values (based on ΛEnum in schedule.go)
	const (
		opSetAndHoldMac    = 1
		opSetAndReleaseMac = 2
		opSetGateStates    = 3
	)

	for _, entry := range gcl.GetEntries() {
		idx := entry.GetIndex()

		ygotEntry := &opencncModel.IETFInterfaces_Interfaces_Interface_BridgePort_GateParameterTable_AdminControlList_GateControlEntry{
			Index:             ygot.Uint32(idx),
			TimeIntervalValue: ygot.Uint32(uint32(entry.GetTimeInterval())),
			OperationName:     opencncModel.E_Ieee802Dot1QTypes_TypeOfOperation(opSetGateStates), // default operation
		}

		// If gate_states is present, use the first byte
		if gateStates := entry.GetGateStates(); len(gateStates) > 0 {
			ygotEntry.GateStatesValue = ygot.Uint8(gateStates[0])
			//ygotGcl.AdminGateStates = ygot.Uint8(gateStates[0])
		}

		ygotGcl.AdminControlList.GateControlEntry[idx] = ygotEntry
	}

	// Log optional vendor-specific field
	if gcl.InterfaceTimeOffsetNs != nil {
		p.logger.Printf("[Qbv] Interface time offset (ns): %d", *gcl.InterfaceTimeOffsetNs)
		// Note: Not mapped in YANG structure
	}

	return ygotGcl, nil
}

func (p *QbvNetconfPlugin) Push(mapped any, target managementSessions.DeviceTarget) error {

	// ---------- check input ----------
	root, ok := mapped.(*opencncModel.IETFInterfaces_Interfaces_Interface_BridgePort_GateParameterTable)
	if !ok {
		return fmt.Errorf("invalid mapped type for Push: %T", mapped)
	}

	// -------- warp GCL in a device ---------
	device := &opencncModel.Device{
		Interfaces: &opencncModel.IETFInterfaces_Interfaces{
			Interface: map[string]*opencncModel.IETFInterfaces_Interfaces_Interface{
				target.InterfaceName: {
					Name: ygot.String(target.InterfaceName),
					BridgePort: &opencncModel.IETFInterfaces_Interfaces_Interface_BridgePort{
						GateParameterTable: root,
					},
				},
			},
		},
	}

	// -------- export config to annotated json ---------
	jsonConfig := &ygot.EmitJSONConfig{
		Format: ygot.RFC7951,
		Indent: "  ",
		RFC7951Config: &ygot.RFC7951JSONConfig{
			AppendModuleName: true, // ✅ this enables module prefixes
		},
	}
	jsonStr, err := ygot.EmitJSON(device, jsonConfig)
	if err != nil {
		log.Fatalf("Failed to emit JSON: %v", err)
	}
	//fmt.Println(string(jsonStr))

	// -------- Convert Json to XML ---------
	var data interface{}
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		log.Fatalf("Failed to unmarshal emitted JSON: %v", err)
	}
	var buf bytes.Buffer
	if err := utils.ConvertToXML(data, &buf, 0); err != nil {
		log.Fatalf("Conversion failed: %v", err)
	}
	//fmt.Println(buf.String())

	// ---------- Push via NETCONF ----------

	// TODO: use existing session with full device/port push. you find it in target.Session

	session, err := managementSessions.CreateSession(
		target.Info.IpAddress,
		target.Info.UserName,
		target.Secret,
	)
	if err != nil {
		return fmt.Errorf("NETCONF session failed: %w", err)
	}
	defer session.Close()

	if err := managementSessions.EditConfig(session, buf.String()); err != nil {
		return fmt.Errorf("edit-config failed: %w", err)
	}

	target.Logger.Println("✅ Config pushed successfully")

	return nil
}
