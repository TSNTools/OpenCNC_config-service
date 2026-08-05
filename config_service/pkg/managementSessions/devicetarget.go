package managementSessions

import (
	"OpenCNC_config-service/common/observability"
	topology "OpenCNC_config-service/common/structures/topology"

	"github.com/openshift-telco/go-netconf-client/netconf"
)

type DeviceTarget struct {
	Info          *topology.ManagementInfo
	Secret        string
	Logger        observability.Logger
	Session       *netconf.Session
	InterfaceName string
	// You can extend this with sessions, retry, TLS configs, etc.
}
