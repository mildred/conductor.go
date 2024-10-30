package install

import _ "embed"

///////////////////////////////////////////////////////////////////////////////

const ConductorCGIFunctionServiceLocation = "/etc/systemd/system/conductor-cgi-function@.service"

//go:embed files/conductor-cgi-function@.service
var ConductorCGIFunctionService string

///////////////////////////////////////////////////////////////////////////////

const ConductorServiceServiceLocation = "/etc/systemd/system/conductor-service@.service"

//go:embed files/conductor-service@.service
var ConductorServiceService string

///////////////////////////////////////////////////////////////////////////////

const ConductorServiceConfigServiceLocation = "/etc/systemd/system/conductor-service-config@.service"

//go:embed files/conductor-service-config@.service
var ConductorServiceConfigService string

///////////////////////////////////////////////////////////////////////////////

const ConductorDeploymentServiceLocation = "/etc/systemd/system/conductor-deployment@.service"

//go:embed files/conductor-deployment@.service
var ConductorDeploymentService string

///////////////////////////////////////////////////////////////////////////////

const ConductorDeploymentConfigServiceLocation = "/etc/systemd/system/conductor-deployment-config@.service"

//go:embed files/conductor-deployment-config@.service
var ConductorDeploymentConfigService string

///////////////////////////////////////////////////////////////////////////////

const ConductorFunctionSocketLocation = "/etc/systemd/system/conductor-cgi-function@.socket"

//go:embed files/conductor-cgi-function@.socket
var ConductorFunctionSocket string

///////////////////////////////////////////////////////////////////////////////

const ConductorPolicyServerServiceLocation = "/etc/systemd/system/conductor-policy-server.service"

//go:embed files/conductor-policy-server.service
var ConductorPolicyServerService string

///////////////////////////////////////////////////////////////////////////////

const ConductorPolicyServerSocketLocation = "/etc/systemd/system/conductor-policy-server.socket"

//go:embed files/conductor-policy-server.socket
var ConductorPolicyServerSocket string
