package install

import (
	_ "embed"

	"github.com/mildred/conductor.go/src/dirs"
)

///////////////////////////////////////////////////////////////////////////////

var ConductorCGIFunctionServiceLocation = dirs.Join(dirs.ConfigHome, "systemd", dirs.SystemdMode(), "conductor-cgi-function@.service")

//go:embed files/conductor-cgi-function@.service
var ConductorCGIFunctionService string

///////////////////////////////////////////////////////////////////////////////

var ConductorServiceServiceLocation = dirs.Join(dirs.ConfigHome, "systemd", dirs.SystemdMode(), "conductor-service@.service")

//go:embed files/conductor-service@.service
var ConductorServiceService string

///////////////////////////////////////////////////////////////////////////////

var ConductorServiceConfigServiceLocation = dirs.Join(dirs.ConfigHome, "systemd", dirs.SystemdMode(), "conductor-service-config@.service")

//go:embed files/conductor-service-config@.service
var ConductorServiceConfigService string

///////////////////////////////////////////////////////////////////////////////

var ConductorDeploymentServiceLocation = dirs.Join(dirs.ConfigHome, "systemd", dirs.SystemdMode(), "conductor-deployment@.service")

//go:embed files/conductor-deployment@.service
var ConductorDeploymentService string

///////////////////////////////////////////////////////////////////////////////

var ConductorDeploymentConfigServiceLocation = dirs.Join(dirs.ConfigHome, "systemd", dirs.SystemdMode(), "conductor-deployment-config@.service")

//go:embed files/conductor-deployment-config@.service
var ConductorDeploymentConfigService string

///////////////////////////////////////////////////////////////////////////////

var ConductorFunctionSocketLocation = dirs.Join(dirs.ConfigHome, "systemd", dirs.SystemdMode(), "conductor-cgi-function@.socket")

//go:embed files/conductor-cgi-function@.socket
var ConductorFunctionSocket string

///////////////////////////////////////////////////////////////////////////////

var ConductorPolicyServerServiceLocation = dirs.Join(dirs.ConfigHome, "systemd", dirs.SystemdMode(), "conductor-policy-server.service")

//go:embed files/conductor-policy-server.service
var ConductorPolicyServerService string

///////////////////////////////////////////////////////////////////////////////

var ConductorPolicyServerSocketLocation = dirs.Join(dirs.ConfigHome, "systemd", dirs.SystemdMode(), "conductor-policy-server.socket")

//go:embed files/conductor-policy-server.socket
var ConductorPolicyServerSocket string
