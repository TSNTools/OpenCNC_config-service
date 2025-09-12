package protocolbackends

import (
	"fmt"
	"structures"
)

var (
	backends = make(map[structures.ManagementProtocol]ProtocolBackend)
)

// RegisterBackend adds a backend for a specific protocol (e.g., NETCONF, SNMP)
func RegisterBackend(protocol structures.ManagementProtocol, backend ProtocolBackend) error {
	if _, exists := backends[protocol]; exists {
		return fmt.Errorf("backend for protocol %v already registered", protocol)
	}
	backends[protocol] = backend
	return nil
}

// GetBackend returns the backend for the given protocol
func GetBackend(protocol structures.ManagementProtocol) (ProtocolBackend, error) {
	if b, ok := backends[protocol]; ok {
		return b, nil
	}
	return nil, fmt.Errorf("no backend found for protocol: %v", protocol)
}

// ListBackends returns all registered protocol backends
func ListBackends() map[structures.ManagementProtocol]ProtocolBackend {
	return backends
}
