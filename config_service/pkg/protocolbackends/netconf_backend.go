package protocolbackends

import (
	"fmt"
	"math"
	"os"
	"reflect"
	"strings"
	"sync"

	"OpenCNC_config_service/common/observability"
	storewrapper "OpenCNC_config_service/common/store-wrapper"
	devicemodelregistry "OpenCNC_config_service/common/structures/devicemodelregistry"
	"OpenCNC_config_service/common/structures/topology"
	topology_config "OpenCNC_config_service/common/structures/topology_config"
	"OpenCNC_config_service/config_service/pkg/managementSessions"
	"OpenCNC_config_service/config_service/pkg/plugins"

	"github.com/beevik/etree"
	"github.com/golang/protobuf/proto"
)

var _ ProtocolBackend = (*NetconfBackend)(nil)

type NetconfSnapshot struct {
	XML     []byte // parsed model, cached payload, metadata...
	Timeout int32  // timeout in seconds for NETCONF operations
	Node    *Node  // optional reference to the associated node
}
type Node struct {
	Name string
	IP   string
}

func (s *NetconfSnapshot) Clone() Snapshot {
	//todo: check it, this is a placeholder so far
	return &NetconfSnapshot{
		XML:     append([]byte(nil), s.XML...),
		Timeout: s.Timeout,
	}
}

func (s *NetconfSnapshot) Update(feature *plugins.FeatureXML, target managementSessions.DeviceTarget) error {
	if feature == nil {
		return fmt.Errorf("feature XML is nil")
	}
	if len(feature.XML) == 0 {
		return fmt.Errorf("feature XML is empty")
	}
	if len(s.XML) == 0 {
		return fmt.Errorf("snapshot XML is empty")
	}

	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(s.XML); err != nil {
		return fmt.Errorf("failed parsing snapshot XML: %w", err)
	}

	// 1. SANITIZE: Extract <interfaces> and <bridges> if wrapped in <rpc-reply><data>
	// We use AddChild directly on the doc so it acts as a fragment holding both elements.
	if doc.Root() != nil && doc.Root().Tag != "interfaces" && doc.Root().Tag != "bridges" {
		newDoc := etree.NewDocument()
		if intf := doc.FindElement("//interfaces"); intf != nil {
			newDoc.AddChild(intf.Copy())
		}
		if br := doc.FindElement("//bridges"); br != nil {
			newDoc.AddChild(br.Copy())
		}
		if len(newDoc.ChildElements()) > 0 {
			doc = newDoc
		}
	}

	// 2. PARSE NEW FEATURE
	featureDoc := etree.NewDocument()
	if err := featureDoc.ReadFromBytes(feature.XML); err != nil {
		return fmt.Errorf("failed parsing feature XML: %w", err)
	}
	if featureDoc.Root() == nil {
		return fmt.Errorf("feature XML has no root element")
	}
	featureRoot := featureDoc.Root()

	// =========================================================================
	// 3. HANDLE BRIDGE UPDATES FIRST
	// =========================================================================
	if featureRoot.Tag == "bridges" {
		if existingBridges := doc.FindElement("//bridges"); existingBridges != nil {
			doc.RemoveChild(existingBridges)
		}
		doc.AddChild(featureRoot.Copy())
		doc.Indent(2)
		s.XML, _ = doc.WriteToBytes()
		return nil
	}

	// =========================================================================
	// 4. HANDLE INTERFACE UPDATES SECOND
	// =========================================================================
	interfaces := doc.FindElement("//interfaces")
	if interfaces == nil {
		return fmt.Errorf("snapshot does not contain <interfaces>")
	}

	var interfaceElement *etree.Element
	for _, intf := range interfaces.FindElements("interface") {
		name := intf.FindElement("name")
		if name == nil {
			continue
		}
		if name.Text() == target.InterfaceName {
			interfaceElement = intf
			break
		}
	}

	// STRICT VALIDATION: Fail if the specific interface doesn't exist
	if interfaceElement == nil {
		return fmt.Errorf("interface %q not found in snapshot", target.InterfaceName)
	}

	// MERGE LOGIC: Deep merge for 'bridge-port', overwrite for everything else
	if featureRoot.Tag == "bridge-port" {
		existingBridgePort := interfaceElement.FindElement("bridge-port")
		if existingBridgePort == nil {
			interfaceElement.AddChild(featureRoot.Copy())
		} else {
			for _, child := range featureRoot.ChildElements() {
				if existingChild := existingBridgePort.FindElement(child.Tag); existingChild != nil {
					existingBridgePort.RemoveChild(existingChild)
				}
				existingBridgePort.AddChild(child.Copy())
			}
		}
	} else {
		if existing := interfaceElement.FindElement(feature.Container); existing != nil {
			interfaceElement.RemoveChild(existing)
		}
		interfaceElement.AddChild(featureRoot.Copy())
	}

	// Store updated snapshot.
	doc.Indent(2)
	updatedXML, err := doc.WriteToBytes()
	if err != nil {
		return fmt.Errorf("failed serializing updated snapshot: %w", err)
	}

	s.XML = updatedXML
	return nil
}

//-----------------------------------
// Definition of the NetconfBackend
//-----------------------------------

type NetconfBackend struct {
	name      string
	mu        sync.RWMutex
	protocol  topology.ManagementProtocol
	plugins   []plugins.Plugin
	logger    observability.Logger
	snapshots map[string]*SnapshotSet[*NetconfSnapshot]
}

func NewNetconfBackend(name string, logger observability.Logger, plugins ...plugins.Plugin) *NetconfBackend {
	return &NetconfBackend{
		name:      name,
		protocol:  topology.ManagementProtocol_NETCONF,
		plugins:   plugins,
		logger:    observability.NormalizeLogger(logger),
		snapshots: make(map[string]*SnapshotSet[*NetconfSnapshot]),
	}
}

func (b *NetconfBackend) Name() string {
	return b.name
}

func (b *NetconfBackend) Protocol() topology.ManagementProtocol {
	return b.protocol
}

func (b *NetconfBackend) AddPlugin(plugin plugins.Plugin) {
	b.plugins = append(b.plugins, plugin)
}

func (b *NetconfBackend) Plugins() []plugins.Plugin {
	return b.plugins
}
func (b *NetconfBackend) InitSnapshot(nodeName string, xmlBytes []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.snapshots == nil {
		b.snapshots = make(map[string]*SnapshotSet[*NetconfSnapshot])
	}
	b.snapshots[nodeName] = &SnapshotSet[*NetconfSnapshot]{
		Current: &NetconfSnapshot{XML: append([]byte(nil), xmlBytes...)},
	}
}

func (b *NetconfBackend) PrepareSnapshot(msg *topology_config.NodeConfig, node *topology.Node) error {
	logger := b.logger

	if node == nil {
		return fmt.Errorf("PrepareSnapshot: node is nil")
	}

	nodeConfig := msg
	if nodeConfig == nil {
		return fmt.Errorf("PrepareSnapshot: nodeConfig is nil")
	}

	b.mu.RLock()
	snapshotSet, ok := b.snapshots[node.Name]
	b.mu.RUnlock()

	if !ok {
		logger.Printf("No snapshot found for node %s. Auto-initializing baseline snapshot via NETCONF get-config...", node.Name)
		var xmlBytes []byte

		if node.ManagementInfo != nil && node.ManagementInfo.IpAddress != "" {
			secret := os.Getenv("NETCONF_PASSWORD")
			if secret == "" {
				secret = node.ManagementInfo.UserName
			}
			session, err := managementSessions.CreateSession(
				node.ManagementInfo.IpAddress, node.ManagementInfo.UserName,
				secret,
			)
			if err == nil {
				rawXML, err := managementSessions.GetRunningConfig(session)
				session.Close()
				if err == nil && len(rawXML) > 0 {
					logger.Printf("Successfully auto-initialized snapshot for node %s via NETCONF get-config (%d bytes)", node.Name, len(rawXML))
					xmlBytes = []byte(rawXML)
				} else {
					b.logger.Warnf("[NETCONF Auto-Init Warning] NETCONF get-config for node %s (%s) failed: %v", node.Name, node.ManagementInfo.IpAddress, err)
				}
			} else {
				b.logger.Warnf("[NETCONF Auto-Init Warning] NETCONF session for auto-initializing node %s (%s) failed: %v", node.Name, node.ManagementInfo.IpAddress, err)
			}
		}

		b.mu.Lock()
		if b.snapshots == nil {
			b.snapshots = make(map[string]*SnapshotSet[*NetconfSnapshot])
		}
		if existing, exists := b.snapshots[node.Name]; exists && existing != nil && existing.Current != nil {
			snapshotSet = existing
		} else {
			snapshotSet = &SnapshotSet[*NetconfSnapshot]{
				Current: &NetconfSnapshot{XML: xmlBytes},
			}
			b.snapshots[node.Name] = snapshotSet
		}
		b.mu.Unlock()
	}

	//
	// Snapshot starts:
	// Current -> Working snapshot
	//
	snapshotSet.Working = snapshotSet.Current.Clone().(*NetconfSnapshot)

	if snapshotSet.Working == nil {
		err := fmt.Errorf("failed creating working snapshot")
		b.logger.Errorf("[NETCONF Prepare Failed] Node %s: %v", node.Name, err)
		return err
	}

	modelName := node.DeviceInfo.GetDeviceModel()

	nodeDeviceModel, err := storewrapper.GetDeviceModel(modelName)
	if err != nil {
		b.logger.Errorf("[NETCONF Prepare Failed] Failed to retrieve device model %q for node %s: %v", modelName, node.Name, err)
		return fmt.Errorf(
			"failed to retrieve device model %q: %w",
			modelName,
			err,
		)
	}

	// Calculate custom timeout for 2018 device models based on GCL length and port count:
	// formula: 5 + 0.03 * max_gcl_len * number_of_ports
	timeout := CalculateNetconfTimeout(nodeConfig, nodeDeviceModel)
	snapshotSet.Working.Timeout = timeout
	logger.Printf("[NETCONF Timeout] Calculated commit timeout for node %s (%s): %ds (ports: %d)",
		node.Name, modelName, timeout, len(nodeConfig.GetPortConfigs()))

	// =========================================================================
	// NEW: Process Bridge-level configuration (e.g. VLAN database) FIRST
	// =========================================================================
	if nodeConfig.Bridge != nil {
		logger.Printf("======================================================")
		logger.Printf("Processing Bridge Configuration for node %s", node.Name)

		secret := os.Getenv("NETCONF_PASSWORD")
		if secret == "" && node.ManagementInfo != nil {
			secret = node.ManagementInfo.UserName
		}

		bridgeTarget := managementSessions.DeviceTarget{
			InterfaceName: nodeConfig.Bridge.GetBridgeName(),
			Logger:        logger,
			Secret:        secret,
			Info:          node.ManagementInfo,
		}

		for _, plugin := range b.plugins {
			if !plugin.SupportedByDevice(nodeDeviceModel) {
				continue
			}

			fields := plugin.SupportedFields(nodeConfig.Bridge)
			if len(fields) == 0 {
				continue
			}

			logger.Printf("Plugin %-20s : mapping Bridge Config", plugin.Name())
			mapped, err := plugin.Map(nodeConfig.Bridge)
			if err != nil {
				b.logger.Errorf("[NETCONF Mapping Failed] Plugin %s failed on bridge for node %s: %v", plugin.Name(), node.Name, err)
				return fmt.Errorf("%s: %w", plugin.Name(), err)
			}

			featureXML, err := plugin.BuildFeatureXML(mapped)
			if err != nil {
				b.logger.Errorf("[NETCONF Feature XML Failed] Plugin %s failed building XML on bridge for node %s: %v", plugin.Name(), node.Name, err)
				return fmt.Errorf("%s: %w", plugin.Name(), err)
			}

			if featureXML != nil && len(featureXML.XML) > 0 {
				doc := etree.NewDocument()
				prettyXML := string(featureXML.XML)
				if err := doc.ReadFromBytes(featureXML.XML); err == nil {
					doc.Indent(2)
					if formatted, err := doc.WriteToString(); err == nil {
						prettyXML = formatted
					}
				}
				logger.Printf("\n/*******************************************************************************\n[%s] Generated Feature XML for Bridge (%s):\n%s\n*******************************************************************************/", plugin.FeatureName(), plugin.Name(), prettyXML)

				if err := snapshotSet.Working.Update(featureXML, bridgeTarget); err != nil {
					b.logger.Errorf("[NETCONF Snapshot Update Failed] Plugin %s update failed on bridge for node %s: %v", plugin.Name(), node.Name, err)
					return fmt.Errorf("failed to update snapshot with bridge data: %w", err)
				}
				logger.Printf("  -> snapshot updated for plugin %s (Bridge)", plugin.Name())
			}
		}
	}
	// =========================================================================
	for _, portConfig := range nodeConfig.PortConfigs {

		if portConfig == nil {
			continue
		}

		logger.Printf("======================================================")
		logger.Printf("Processing port %q", portConfig.PortId)

		secret := os.Getenv("NETCONF_PASSWORD")
		if secret == "" && node.ManagementInfo != nil {
			secret = node.ManagementInfo.UserName
		}

		target := managementSessions.DeviceTarget{
			InterfaceName: portConfig.PortId,
			Logger:        logger,
			Secret:        secret,
			Info:          node.ManagementInfo,
		}

		used := make(map[string]struct{})

		src := reflect.ValueOf(portConfig).Elem()
		srcType := src.Type()

		for _, plugin := range b.plugins {

			if !plugin.SupportedByDevice(nodeDeviceModel) {
				logger.Printf(
					"Skipping plugin %s: unsupported by device model %s",
					plugin.Name(),
					modelName,
				)
				continue
			}

			fields := plugin.SupportedFields(portConfig)

			if len(fields) == 0 {
				logger.Printf(
					"Plugin %-20s : no supported fields declared",
					plugin.Name(),
				)
				continue
			}

			logger.Printf(
				"Plugin %-20s : supports %v",
				plugin.Name(),
				fields,
			)

			var input proto.Message

			if len(fields) == 1 {

				f := src.FieldByName(fields[0])

				if !f.IsValid() || f.IsNil() {
					logger.Printf(
						"  -> field %q is not valid, skipping",
						fields[0],
					)
					continue
				}

				fieldMsg, ok := f.Interface().(proto.Message)
				if !ok {
					err := fmt.Errorf("field %q does not implement proto.Message", fields[0])
					b.logger.Errorf("[NETCONF Mapping Failed] Node %s port %s: %v", node.Name, target.InterfaceName, err)
					return err
				}

				logger.Printf(
					"  -> mapping field %q (%T)",
					fields[0],
					fieldMsg,
				)

				input = fieldMsg
				used[fields[0]] = struct{}{}

			} else {

				dst := &topology_config.PortConfig{}
				dstVal := reflect.ValueOf(dst).Elem()

				found := false
				var mappedFields []string

				for _, name := range fields {

					sf := src.FieldByName(name)

					if !sf.IsValid() {
						logger.Printf(
							"  -> field %q does not exist",
							name,
						)
						continue
					}

					switch sf.Kind() {
					case reflect.Pointer, reflect.Slice, reflect.Map:
						if sf.IsNil() {
							continue
						}
					}

					dstVal.FieldByName(name).Set(sf)

					used[name] = struct{}{}
					mappedFields = append(mappedFields, name)
					found = true
				}

				if !found {
					logger.Printf(
						"  -> none of the supported fields are present",
					)
					continue
				}

				logger.Printf(
					"  -> mapping PortConfig with fields %v",
					mappedFields,
				)

				input = dst
			}

			logger.Printf("  -> calling Map()")

			mapped, err := plugin.Map(input)
			if err != nil {
				b.logger.Errorf("[NETCONF Mapping Failed] Plugin %s failed on port %s for node %s: %v", plugin.Name(), target.InterfaceName, node.Name, err)
				return fmt.Errorf(
					"%s: %w",
					plugin.Name(),
					err,
				)
			}

			logger.Printf(
				"  <- Map() returned %T",
				mapped,
			)

			logger.Printf("  -> building feature XML")
			featureXML, err := plugin.BuildFeatureXML(mapped)
			if err != nil {
				b.logger.Errorf("[NETCONF Feature XML Failed] Plugin %s failed building XML on port %s for node %s: %v", plugin.Name(), target.InterfaceName, node.Name, err)
				return fmt.Errorf("%s: %w", plugin.Name(), err)
			}
			if featureXML != nil && len(featureXML.XML) > 0 {
				doc := etree.NewDocument()
				prettyXML := string(featureXML.XML)
				if err := doc.ReadFromBytes(featureXML.XML); err == nil {
					doc.Indent(2)
					if formatted, err := doc.WriteToString(); err == nil {
						prettyXML = formatted
					}
				}
				logger.Printf("\n/*******************************************************************************\n[%s] Generated Feature XML for %s (%s):\n%s\n*******************************************************************************/", plugin.FeatureName(), plugin.Name(), target.InterfaceName, prettyXML)
			}
			if err := snapshotSet.Working.Update(featureXML, target); err != nil {
				b.logger.Errorf("[NETCONF Snapshot Update Failed] Plugin %s update failed on port %s for node %s: %v", plugin.Name(), target.InterfaceName, node.Name, err)
				return fmt.Errorf("failed to update snapshot: %w", err)
			}

			logger.Printf("  -> snapshot updated for plugin %s (interface %s)", plugin.Name(), target.InterfaceName)
		}

		var unused []string

		for i := 0; i < src.NumField(); i++ {

			field := srcType.Field(i)

			if _, ok := used[field.Name]; ok {
				continue
			}

			v := src.Field(i)

			switch v.Kind() {
			case reflect.Pointer, reflect.Slice, reflect.Map:
				if v.IsNil() {
					continue
				}
			}

			unused = append(unused, field.Name)
		}

		if len(unused) == 0 {
			logger.Printf("All populated fields were handled.")
		} else {
			logger.Printf(
				"Unused populated fields: %v",
				unused,
			)
		}

		logger.Printf(
			"Finished processing port %q",
			portConfig.PortId,
		)

		logger.Printf("======================================================")
	}

	logger.Printf(
		"Snapshot prepared successfully for node %s",
		node.Name,
	)

	return nil
}

func (b *NetconfBackend) Commit(target *topology.Node) error {

	if target == nil {
		err := fmt.Errorf("Commit: node is nil")
		b.logger.Errorf("[NETCONF Commit Failed] %v", err)
		return err
	}
	b.mu.RLock()

	snapshotSet, ok := b.snapshots[target.Name]
	b.mu.RUnlock()
	if !ok {
		err := fmt.Errorf("no snapshot exists for node %s", target.Name)
		b.logger.Errorf("[NETCONF Commit Failed] %v", err)
		return err
	}

	if snapshotSet.Working == nil {
		err := fmt.Errorf("no working snapshot for node %s", target.Name)
		b.logger.Errorf("[NETCONF Commit Failed] %v", err)
		return err
	}

	b.logger.Printf(
		"Committing configuration for node %s",
		target.Name,
	)

	//
	// Push working configuration
	//
	if err := b.pushSnapshot(
		snapshotSet.Working,
		target,
	); err != nil {
		b.logger.Errorf("[NETCONF Commit Failed] Node %s: %v", target.Name, err)
		return fmt.Errorf(
			"commit failed for node %s: %w",
			target.Name,
			err,
		)
	}

	//
	// Snapshot promotion
	//
	snapshotSet.LastStable = snapshotSet.Current
	snapshotSet.Current = snapshotSet.Working
	snapshotSet.Working = nil

	b.logger.Printf(
		"Commit successful for node %s",
		target.Name,
	)

	return nil
}

func (b *NetconfBackend) Rollback(target *topology.Node) error {

	if target == nil {
		err := fmt.Errorf("Rollback: node is nil")
		b.logger.Errorf("[NETCONF Rollback Failed] %v", err)
		return err
	}
	b.mu.RLock()

	snapshotSet, ok := b.snapshots[target.Name]
	b.mu.RUnlock()
	if !ok {
		err := fmt.Errorf("no snapshot exists for node %s", target.Name)
		b.logger.Errorf("[NETCONF Rollback Failed] %v", err)
		return err
	}

	if snapshotSet.LastStable == nil {
		err := fmt.Errorf("no last stable snapshot for node %s", target.Name)
		b.logger.Errorf("[NETCONF Rollback Failed] %v", err)
		return err
	}

	b.logger.Printf(
		"Rolling back configuration for node %s",
		target.Name,
	)

	//
	// Restore device configuration
	//
	if err := b.pushSnapshot(
		snapshotSet.LastStable,
		target,
	); err != nil {
		b.logger.Errorf("[NETCONF Rollback Failed] Node %s: %v", target.Name, err)
		return fmt.Errorf(
			"rollback failed for node %s: %w",
			target.Name,
			err,
		)
	}

	//
	// Restore runtime state
	//
	snapshotSet.Current = snapshotSet.LastStable.Clone().(*NetconfSnapshot)
	snapshotSet.Working = nil

	b.logger.Printf(
		"Rollback successful for node %s",
		target.Name,
	)

	return nil
}

func (b *NetconfBackend) pushSnapshot(snapshot *NetconfSnapshot, node *topology.Node) error {

	if node == nil {
		err := fmt.Errorf("node is nil")
		b.logger.Errorf("[NETCONF Push Failed] %v", err)
		return err
	}

	if snapshot == nil {
		err := fmt.Errorf("snapshot is nil for node %s", node.GetName())
		b.logger.Errorf("[NETCONF Push Failed] %v", err)
		return err
	}

	if len(snapshot.XML) == 0 {
		err := fmt.Errorf("snapshot XML is empty for node %s", node.GetName())
		b.logger.Errorf("[NETCONF Push Failed] %v", err)
		return err
	}

	secret := os.Getenv("NETCONF_PASSWORD")
	if secret == "" && node.ManagementInfo != nil {
		secret = node.ManagementInfo.UserName
	}

	ip := ""
	user := ""
	if node.ManagementInfo != nil {
		ip = node.ManagementInfo.IpAddress
		user = node.ManagementInfo.UserName
	}

	timeout := snapshot.Timeout
	if timeout <= 0 {
		timeout = 5
	}

	b.logger.Printf("Pushing consolidated configuration snapshot (%d bytes, timeout %ds) to node %s (%s)...",
		len(snapshot.XML), timeout, node.Name, ip)

	session, err := managementSessions.CreateSession(
		ip,
		user,
		secret,
	)
	if err != nil {
		b.logger.Errorf("[NETCONF Session Failed] Node %s (%s): %v", node.Name, ip, err)
		return fmt.Errorf("NETCONF session failed for node %s (%s): %w", node.Name, ip, err)
	}
	defer session.Close()

	if err := managementSessions.EditConfig(
		session,
		string(snapshot.XML),
	); err != nil {
		b.logger.Errorf("[NETCONF EditConfig Failed] Node %s (%s) rejected RPC (timeout %ds): %v", node.Name, ip, timeout, err)
		return fmt.Errorf("failed pushing snapshot to node %s (%s): %w", node.Name, ip, err)
	}

	return nil
}

// Is2018DeviceModel checks if the provided device model represents a 2018 TSN device model
// (e.g. implementing ieee802-dot1q-sched.yang revision 2018-09-10).
func Is2018DeviceModel(model *devicemodelregistry.DeviceModel) bool {
	if model == nil {
		return false
	}
	for _, yf := range model.YangFiles {
		if yf.Name == "ieee802-dot1q-sched.yang" && (yf.Revision == "2018-09-10" || strings.HasPrefix(yf.Revision, "2018")) {
			return true
		}
	}
	return false
}

// CalculateNetconfTimeout computes the NETCONF timeout in seconds based on GCL length and port count:
// For 2018 device models: timeout = 5 + 0.03 * max_gcl_len * number_of_ports
// If different ports have different GCL lengths, the one with the maximum length is used.
// Defaults to 5 seconds (or if non-2018 or no GCL configured).
func CalculateNetconfTimeout(nodeConfig *topology_config.NodeConfig, model *devicemodelregistry.DeviceModel) int32 {
	const defaultTimeout int32 = 5
	if nodeConfig == nil || model == nil {
		return defaultTimeout
	}

	if !Is2018DeviceModel(model) {
		return defaultTimeout
	}

	maxGclLen := 0
	numPorts := len(nodeConfig.GetPortConfigs())

	for _, portConfig := range nodeConfig.GetPortConfigs() {
		if portConfig == nil || portConfig.GetGcl() == nil {
			continue
		}
		gclLen := len(portConfig.GetGcl().GetEntries())
		if gclLen > maxGclLen {
			maxGclLen = gclLen
		}
	}

	if maxGclLen == 0 || numPorts == 0 {
		return defaultTimeout
	}

	calculated := 5.0 + 0.03*float64(maxGclLen)*float64(numPorts)
	timeout := int32(math.Ceil(calculated))
	if timeout < defaultTimeout {
		timeout = defaultTimeout
	}

	return timeout
}
