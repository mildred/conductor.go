package pod

import (
	"github.com/mildred/conductor.go/src/service"

	. "github.com/mildred/conductor.go/src/parts"
)

func init() {
	service.ServicePartImplementations[PodType] = func(p service.AbstractServicePart) service.ServicePartImplementation {
		return &PodServicePart{p.(*service.ServicePod)}
	}
}

type PodServicePart struct {
	part *service.ServicePod
}
