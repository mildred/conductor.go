package function

import (
	"github.com/mildred/conductor.go/src/service"

	. "github.com/mildred/conductor.go/src/parts"
)

func init() {
	service.ServicePartImplementations[FunctionType] = func(p service.AbstractServicePart) service.ServicePartImplementation {
		return &FunctionServicePart{p.(*service.ServiceFunction)}
	}
}

type FunctionServicePart struct {
	part *service.ServiceFunction
}
