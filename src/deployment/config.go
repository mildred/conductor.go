package deployment

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"time"

	"github.com/mildred/conductor.go/src/service"
)

const ConfigName = "conductor-deployment.json"

type Deployment struct {
	*service.Service
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
		DeploymentName:       deployment_name,
		PodName:              "conductor-" + deployment_name,
		TemplatedPod:         "",
		TemplatedConfigMap:   "",
		TemplatedProxyConfig: nil,
	}
}

func ReadDeployment(dir, deployment_id string) (*Deployment, error) {
	_, err := os.Stat(path.Join(dir, ConfigName))
	if err != nil {
		service, err := service.LoadService(service.ConfigName, false, nil)
		if err != nil {
			return nil, err
		}

		err = service.FillDefaults()
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
	res, err := depl.template(depl.PodTemplate)
	if err != nil {
		return err
	}
	depl.TemplatedPod = res

	res, err = depl.template(depl.ConfigMapTemplate)
	if err != nil {
		return err
	}
	depl.TemplatedConfigMap = res

	return nil
}

func (depl *Deployment) TemplateProxyConfig() error {
	log.Printf("prepare: Templating the proxy config\n")
	res, err := depl.template(depl.PodTemplate)
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

func (depl *Deployment) vars() []string {
	var vars []string = []string{
		"CONDUCTOR_APP=" + depl.AppName,
		"CONDUCTOR_INSTANCE=" + depl.InstanceName,
		"CONDUCTOR_DEPLOYMENT=" + depl.DeploymentName,
		"POD_NAME=" + depl.PodName,
		"POD_IP_ADDRESS=" + depl.PodIpAddress,
	}
	for k, v := range depl.Config {
		vars = append(vars, fmt.Sprintf("%s=%s", k, v))
	}
	return vars
}

func (depl *Deployment) template(fname string) (string, error) {
	if fname == "" {
		return "", nil
	}

	fmt.Printf("templating: execute %s\n", fname)
	vars := depl.vars()
	cmd := exec.Command(fname, vars...)
	cmd.Env = append(cmd.Environ(), vars...)
	res, err := cmd.Output()
	return string(res), err
}

func (depl *Deployment) Save(fname string) error {
	log.Printf("Save deployment to %s\n", fname)
	f, err := os.OpenFile(fname, os.O_RDWR, 0)
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
		return exec.Command("podman", compact("kube", "play",
			"--replace",
			configmap_flag,
			"--annotation="+fmt.Sprintf("conductor_deployment=%s", depl.DeploymentName),
			"--annotation="+fmt.Sprintf("conductor_instance=%s", depl.InstanceName),
			"--annotation="+fmt.Sprintf("conductor_app=%s", depl.AppName),
			"--log-driver=journald",
			path.Join(dir, "pod.yml"))...).Run()
	} else {
		return exec.Command("podman", compact("kube", "down",
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

func compact(args ...string) []string {
	var res []string
	for _, s := range args {
		if s != "" {
			res = append(res, s)
		}
	}
	return res
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
		cmd.Env = append(cmd.Environ(), depl.vars()...)
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
