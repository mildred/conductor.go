package install

import _ "embed"

//go:embed files/conductor-cgi-function@.service
var ConductorCGIFunctionService string

const ConductorCGIFunctionServiceLocation = "/etc/systemd/system/conductor-cgi-function@.service"

//go:embed files/conductor-service@.service
var ConductorServiceService string

const ConductorServiceServiceLocation = "/etc/systemd/system/conductor-service@.service"

//go:embed files/conductor-deployment@.service
var ConductorDeploymentService string

const ConductorDeploymentServiceLocation = "/etc/systemd/system/conductor-deployment@.service"
