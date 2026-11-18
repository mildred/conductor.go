package daemon

import (
	"github.com/mildred/conductor.go/src/service"

	. "github.com/mildred/conductor.go/src/parts"
)

func init() {
	service.ServicePartImplementations[DaemonType] = func(p service.AbstractServicePart) service.ServicePartImplementation {
		return &DaemonServicePart{p.(*service.ServiceDaemon)}
	}
}

type DaemonServicePart struct {
	part *service.ServiceDaemon
}
