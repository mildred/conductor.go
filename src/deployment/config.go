package deployment

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"time"

	"github.com/coreos/go-systemd/v22/unit"
	"github.com/gandarez/go-realpath"

	"github.com/mildred/conductor.go/src/dirs"
	"github.com/mildred/conductor.go/src/service"
	"github.com/mildred/conductor.go/src/tmpl"
	"github.com/mildred/conductor.go/src/utils"
)

const ConfigName = "conductor-deployment.json"

var DeploymentRunDir = dirs.Join(dirs.SelfRuntimeDir, "deployments")

func DeploymentUnit(name string) string {
	return fmt.Sprintf("conductor-deployment@%s.service", unit.UnitNameEscape(name))
}

func DeploymentConfigUnit(name string) string {
	return fmt.Sprintf("conductor-deployment-config@%s.service", unit.UnitNameEscape(name))
}

type Deployment struct {
	*service.Service
	ServiceDir           string          `json:"service_dir"`
	ServiceId            string          `json:"service_id"`
	DeploymentName       string          `json:"conductor_deployment"`
	PodName              string          `json:"pod_name"`
	TemplatedPod         string          `json:"templated_pod"`
	TemplatedConfigMap   string          `json:"templated_config_map"`
	TemplatedProxyConfig json.RawMessage `json:"templated_proxy_config"`
	PodIpAddress         string          `json:"pod_ip_address"`
}

func NewDeploymentFromService(service *service.Service, deployment_name string) *Deployment {
	log.Printf("prepare: Set up deployment %q from service %q-%q\n", deployment_name, service.AppName, service.InstanceName)
	return &Deployment{
		Service:              service,
		ServiceDir:           service.BasePath,
		ServiceId:            service.Id,
		DeploymentName:       deployment_name,
		PodName:              "conductor-" + deployment_name,
		TemplatedPod:         "",
		TemplatedConfigMap:   "",
		TemplatedProxyConfig: nil,
	}
}

func ReadDeployment(dir, deployment_id string) (*Deployment, error) {
	_, err := os.Stat(path.Join(dir, ConfigName))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	} else if err != nil {
		service_file, err := realpath.Realpath(path.Join(dir, service.ConfigName))
		if err != nil {
			return nil, err
		}

		service, err := service.LoadServiceAndFillDefaults(service_file, false)
		if err != nil {
			return nil, err
		}

		if deployment_id == "" {
			cwd, err := os.Getwd()
			if err != nil {
				return nil, err
			}
			deployment_id = path.Base(cwd)
		}

		depl := NewDeploymentFromService(service, deployment_id)

		return depl, nil
	} else {
		return LoadDeployment(path.Join(dir, ConfigName))
	}
}

func (depl *Deployment) TemplatePod() error {
	log.Printf("prepare: Templating the pod\n")
	res, err := tmpl.RunTemplate(depl.PodTemplate, depl.Vars())
	if err != nil {
		return err
	}
	depl.TemplatedPod = res

	res, err = tmpl.RunTemplate(depl.ConfigMapTemplate, depl.Vars())
	if err != nil {
		return err
	}
	depl.TemplatedConfigMap = res

	return nil
}

func (depl *Deployment) TemplateProxyConfig() error {
	log.Printf("prepare: Templating the proxy config\n")
	res, err := tmpl.RunTemplate(depl.ProxyConfigTemplate, depl.Vars())
	if err != nil {
		return err
	}

	return json.Unmarshal([]byte(res), &depl.TemplatedProxyConfig)
}

func (depl *Deployment) TemplateAll() error {
	err := depl.TemplatePod()
	if err != nil {
		return err
	}

	return depl.TemplateProxyConfig()
}

func (depl *Deployment) Vars() []string {
	return append(depl.Service.Vars(),
		"CONDUCTOR_DEPLOYMENT="+depl.DeploymentName,
		"POD_NAME="+depl.PodName,
		"POD_IP_ADDRESS="+depl.PodIpAddress,
	)
}

func (depl *Deployment) Save(fname string) error {
	log.Printf("Save deployment to %s\n", fname)
	f, err := os.OpenFile(fname, os.O_TRUNC|os.O_CREATE|os.O_RDWR, 0)
	if err != nil {
		return err
	}

	defer f.Close()
	return json.NewEncoder(f).Encode(depl)
}

func LoadDeployment(fname string) (*Deployment, error) {
	log.Printf("Read file %s\n", fname)
	f, err := os.Open(fname)
	if err != nil {
		return nil, err
	}

	res := &Deployment{}
	err = json.NewDecoder(f).Decode(res)

	res.Service.BasePath = res.ServiceDir
	res.Service.Id = res.ServiceId

	log.Printf("Loaded deployment %s, service %s-%s\n", res.DeploymentName, res.AppName, res.InstanceName)
	return res, err
}

func (depl *Deployment) StartStopPod(start bool, dir string) error {
	var configmap_flag string
	if depl.TemplatedConfigMap != "" {
		err := os.WriteFile(path.Join(dir, "configmap.yml"), []byte(depl.TemplatedConfigMap), 0644)
		if err != nil {
			return err
		}
		configmap_flag = "--configmap=" + path.Join(dir, "configmap.yml")
	}

	err := os.WriteFile(path.Join(dir, "pod.yml"), []byte(depl.TemplatedPod), 0644)
	if err != nil {
		return err
	}

	if start {
		return exec.Command("podman", utils.Compact("kube", "play",
			"--replace",
			configmap_flag,
			"--annotation="+fmt.Sprintf("conductor_deployment=%s", depl.DeploymentName),
			"--annotation="+fmt.Sprintf("conductor_instance=%s", depl.InstanceName),
			"--annotation="+fmt.Sprintf("conductor_app=%s", depl.AppName),
			"--log-driver=journald",
			path.Join(dir, "pod.yml"))...).Run()
	} else {
		return exec.Command("podman", utils.Compact("kube", "down",
			configmap_flag,
			path.Join(dir, "pod.yml"))...).Run()
	}
}

func (depl *Deployment) FindPodIPAddress() (string, error) {
	max := 4
	for i := 0; i <= max; i++ {
		data, err := exec.Command("podman", "pod", "inspect", depl.PodName).Output()
		if err != nil {
			return "", err
		}

		var pod struct {
			Containers []struct {
				Id string
			}
		}

		err = json.Unmarshal(data, &pod)
		if err != nil {
			return "", err
		}

		if len(pod.Containers) > 0 {
			data, err := exec.Command("podman", "container", "inspect", pod.Containers[0].Id).Output()
			if err != nil {
				return "", err
			}

			var containers []struct {
				NetworkSettings struct {
					Networks map[string]struct {
						IPAddress string
					}
				}
			}

			err = json.Unmarshal(data, &containers)
			if err != nil {
				return "", err
			}

			if len(containers) > 0 {
				for _, net := range containers[0].NetworkSettings.Networks {
					if net.IPAddress != "" {
						return net.IPAddress, nil
					}
				}
			}
		}

		if i < max {
			log.Printf("No IP address found, will retry...")
			time.Sleep(time.Second * 5)
		}
	}

	return "", fmt.Errorf("could not find pod IP address")
}

func (depl *Deployment) RunHooks(when string) error {
	for _, hook := range depl.Hooks {
		if hook.When != when {
			continue
		}

		log.Printf("%s hook: Run %v\n", when, hook.Exec)
		cmd := exec.Command("systemd-run",
			append([]string{
				"--scope",
				"--unit=" + fmt.Sprintf("hook-%s-%s", depl.DeploymentName, when),
			}, hook.Exec...)...)
		cmd.Env = append(cmd.Environ(), depl.Vars()...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		if err != nil {
			log.Printf("%s hook: ERROR %v", when, err)
			return err
		}
	}
	log.Printf("%s hook: Completed hooks\n", when)
	return nil
}
