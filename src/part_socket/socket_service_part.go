package socket

import (
	"github.com/mildred/conductor.go/src/service"

	. "github.com/mildred/conductor.go/src/parts"
)

func init() {
	service.ServicePartImplementations[SocketType] = func(p service.AbstractServicePart) service.ServicePartImplementation {
		return &SocketServicePart{p.(*service.ServiceSocket)}
	}
}

type SocketServicePart struct {
	part *service.ServiceSocket
}
