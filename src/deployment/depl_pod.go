package deployment

import (
	"log"

	"github.com/mildred/conductor.go/src/service"
	"github.com/mildred/conductor.go/src/tmpl"
)

type DeploymentPod struct {
	*service.ServicePod
	IPAddress string `json:"ip_address"`
}

func (pod *DeploymentPod) TemplatePod(depl *Deployment) error {
	log.Printf("prepare: Templating the pod\n")
	res, err := tmpl.RunTemplate(pod.PodTemplate, depl.Vars())
	if err != nil {
		return err
	}
	depl.TemplatedPod = res

	res, err = tmpl.RunTemplate(pod.ConfigMapTemplate, depl.Vars())
	if err != nil {
		return err
	}
	depl.TemplatedConfigMap = res

	return nil
}
