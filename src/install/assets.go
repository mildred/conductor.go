package install

import _ "embed"

//go:embed files/conductor-cgi-function@.service
var ConductorCGIFunctionService string

const ConductorCGIFunctionServiceLocation = "/etc/systemd/system/conductor-cgi-function@.service"
