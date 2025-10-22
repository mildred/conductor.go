package deployment

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/mildred/conductor.go/src/caddy"
	"github.com/mildred/conductor.go/src/dirs"
	"github.com/mildred/conductor.go/src/service"
	"github.com/mildred/conductor.go/src/tmpl"
	"github.com/mildred/conductor.go/src/utils"
)

const ConfigName = "conductor-deployment.json"

var DeploymentRunDir = dirs.Join(dirs.SelfRuntimeDir, "deployments")

func CGIFunctionSocketUnit(name string) string {
	return fmt.Sprintf("conductor-cgi-function-%s.socket", name)
}

func CGIFunctionServiceUnitSingle(name string) string {
	return fmt.Sprintf("conductor-cgi-function-%s.service", name)
}

func CGIFunctionServiceUnit(name string, instance string) string {
	return fmt.Sprintf("conductor-cgi-function-%s@%s.service", name, instance)
}

func DeploymentUnit(name string) string {
	return fmt.Sprintf("conductor-deployment@%s.service", name)
}

func DeploymentConfigUnit(name string) string {
	return fmt.Sprintf("conductor-deployment-config@%s.service", name)
}

func DeploymentSocketPath(name string) string {
	return path.Join(DeploymentRunDir, name, "stream.socket")
}

func DeploymentDirByName(name string, allow_dir bool) string {
	if allow_dir && strings.Contains(name, "/") {
		return name
	} else {
		return path.Join(DeploymentRunDir, name)
	}
}

type Deployment struct {
	*service.Service
	Seed                 *DeploymentSeed     `json:"-"`
	ServiceDir           string              `json:"service_dir"`
	ServiceId            string              `json:"service_id"`
	DeploymentName       string              `json:"conductor_deployment"`
	PartName             string              `json:"part_name"`
	PartId               string              `json:"part_id"`
	PodName              string              `json:"pod_name"`
	TemplatedPod         string              `json:"templated_pod"`
	TemplatedConfigMap   string              `json:"templated_config_map"`
	TemplatedProxyConfig json.RawMessage     `json:"templated_proxy_config"`
	Pod                  *DeploymentPod      `json:"pod,omitempty"`
	Function             *DeploymentFunction `json:"function,omitempty"`
	ProxyConfigValue     caddy.ConfigItems   `json:"_proxy_config"`
}

func NewDeploymentFromService(service *service.Service, deployment_name string, seed *DeploymentSeed) *Deployment {
	log.Printf("prepare: Set up deployment %q from service %q-%q\n", deployment_name, service.AppName, service.InstanceName)
	var pod *DeploymentPod
	var f *DeploymentFunction

	if service_pod := service.Pods.FindPod(seed.PartName); service_pod != nil {
		pod = &DeploymentPod{
			ServicePod: service_pod,
		}
	}

	if service_func := service.Functions.FindFunction(seed.PartName); service_func != nil {
		f = &DeploymentFunction{
			ServiceFunction: service_func,
		}
	}

	return &Deployment{
		Seed:                 seed,
		Service:              service,
		ServiceDir:           service.BasePath,
		ServiceId:            service.Id,
		DeploymentName:       deployment_name,
		PartName:             seed.PartName,
		PartId:               seed.PartId,
		PodName:              "conductor-" + deployment_name,
		TemplatedPod:         "",
		TemplatedConfigMap:   "",
		TemplatedProxyConfig: nil,
		Pod:                  pod,
		Function:             f,
	}
}

func ReadDeploymentByName(name string, allow_dir bool) (*Deployment, error) {
	return ReadDeployment(DeploymentDirByName(name, allow_dir), name)
}

func ReadDeployment(dir, deployment_id string) (*Deployment, error) {
	_, err := os.Stat(path.Join(dir, ConfigName))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	} else if err != nil {
		service, err := service.LoadServiceDir(dir)
		if err != nil {
			return nil, err
		}

		seed, err := ReadSeed(path.Join(dir, SeedName))
		if err != nil && os.IsNotExist(err) {
			// default seed if file is not present
			seed = &DeploymentSeed{
				ServiceDir: service.BasePath,
			}
		} else if err != nil {
			return nil, err
		}

		if deployment_id == "" {
			cwd, err := os.Getwd()
			if err != nil {
				return nil, err
			}
			deployment_id = path.Base(cwd)
		}

		depl := NewDeploymentFromService(service, deployment_id, seed)

		return depl, nil
	} else {
		return LoadDeployment(path.Join(dir, ConfigName))
	}
}

func (depl *Deployment) TemplateProxyConfig(ctx context.Context) error {
	if depl.ProxyConfigTemplate == "" {
		depl.TemplatedProxyConfig = nil
		return nil
	}

	log.Printf("prepare: Templating the proxy config\n")
	res, err := tmpl.RunTemplate(ctx, depl.ProxyConfigTemplate, depl.Vars())
	if err != nil {
		return err
	}

	return json.Unmarshal([]byte(res), &depl.TemplatedProxyConfig)
}

func (depl *Deployment) TemplateAll(ctx context.Context) error {
	if depl.Pod != nil {
		err := depl.Pod.TemplatePod(ctx, depl)
		if err != nil {
			return err
		}
	}

	err := depl.TemplateProxyConfig(ctx)
	if err != nil {
		return err
	}

	depl.ProxyConfigValue, err = depl.ProxyConfig(ctx)
	if err != nil {
		return err
	}

	return nil
}

func (depl *Deployment) ProxyConfig(ctx context.Context) (caddy.ConfigItems, error) {
	var configs caddy.ConfigItems

	if depl.Pod != nil {
		cfg, err := depl.Pod.ProxyConfig(depl)
		if err != nil {
			return nil, err
		}

		configs = append(configs, cfg...)
	}

	if depl.Function != nil {
		cfg, err := depl.Function.ProxyConfig(depl)
		if err != nil {
			return nil, err
		}

		configs = append(configs, cfg...)
	}

	if depl.ProxyConfigTemplate != "" {
		var c caddy.ConfigItems
		err := tmpl.RunTemplateJSON(ctx, depl.ProxyConfigTemplate, depl.Vars(), &c)
		if err != nil {
			return nil, fmt.Errorf("while running the proxy-config template, %v", err)
		}

		configs = append(configs, c...)
	}

	err := configs.SetDefaults()
	if err != nil {
		return nil, err
	}

	return configs, nil
}

func (depl *Deployment) Vars() []string {
	var vars []string = append(depl.Service.Vars(),
		"CONDUCTOR_SERVICE_PART="+depl.PartName,
		"CONDUCTOR_DEPLOYMENT="+depl.DeploymentName,
		"CONDUCTOR_DEPLOYMENT_SERVICE_ID="+depl.ServiceId,
		"CONDUCTOR_DEPLOYMENT_UNIT="+DeploymentUnit(depl.DeploymentName),
		"CONDUCTOR_DEPLOYMENT_CONFIG_UNIT="+DeploymentConfigUnit(depl.DeploymentName),
		"CONDUCTOR_PART_ID="+depl.PartId,
	)

	if depl.Pod != nil {
		vars = append(vars,
			"CONDUCTOR_DEPLOYMENT_KIND=pod",
			"POD_IP_ADDRESS="+depl.Pod.IPAddress,
			"POD_NAME="+depl.PodName,
			"CONDUCTOR_POD_IP_ADDRESS="+depl.Pod.IPAddress,
			"CONDUCTOR_POD_NAME="+depl.PodName,
			"CONDUCTOR_FUNCTION_FORMAT=",
			"CONDUCTOR_FUNCTION_ID=",
			"CONDUCTOR_FUNCTION_SOCKET=",
			"CONDUCTOR_FUNCTION_POLICIES=",
		)
	} else if depl.Function != nil {
		vars = append(vars,
			"CONDUCTOR_DEPLOYMENT_KIND=function",
			"POD_IP_ADDRESS=",
			"POD_NAME=",
			"CONDUCTOR_POD_IP_ADDRESS=",
			"CONDUCTOR_POD_NAME=",
			"CONDUCTOR_FUNCTION_FORMAT="+depl.Function.Format,
			"CONDUCTOR_FUNCTION_ID="+depl.PartId,
			"CONDUCTOR_FUNCTION_SOCKET="+DeploymentSocketPath(depl.DeploymentName),
			"CONDUCTOR_FUNCTION_POLICIES="+strings.Join(depl.Function.Policies, " "),
		)
	} else {
		vars = append(vars,
			"CONDUCTOR_DEPLOYMENT_KIND=",
			"POD_IP_ADDRESS=",
			"POD_NAME=",
			"CONDUCTOR_POD_IP_ADDRESS=",
			"CONDUCTOR_POD_NAME=",
			"CONDUCTOR_FUNCTION_FORMAT=",
			"CONDUCTOR_FUNCTION_ID=",
			"CONDUCTOR_FUNCTION_SOCKET=",
			"CONDUCTOR_FUNCTION_POLICIES=",
		)
	}

	return vars
}

func (depl *Deployment) Save(fname string) error {
	log.Printf("Save deployment to %s\n", fname)
	f, err := os.OpenFile(fname, os.O_TRUNC|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return err
	}

	defer f.Close()
	return json.NewEncoder(f).Encode(depl)
}

func LoadDeploymentDir(dir string) (*Deployment, error) {
	return LoadDeployment(path.Join(dir, ConfigName))
}

func LoadDeployment(fname string) (*Deployment, error) {
	// log.Printf("Read file %s\n", fname)
	f, err := os.Open(fname)
	if err != nil {
		return nil, err
	}

	res := &Deployment{}
	err = json.NewDecoder(f).Decode(res)

	res.Service.BasePath = res.ServiceDir
	res.Service.Id = res.ServiceId

	// log.Printf("Loaded deployment %s, service %s-%s\n", res.DeploymentName, res.AppName, res.InstanceName)
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

	var args []string
	if start {
		args = utils.Compact("kube", "play",
			"--replace",
			configmap_flag,
			"--annotation="+fmt.Sprintf("conductor_deployment=%s", depl.DeploymentName),
			"--annotation="+fmt.Sprintf("conductor_instance=%s", depl.InstanceName),
			"--annotation="+fmt.Sprintf("conductor_app=%s", depl.AppName),
			"--log-driver=journald",
			path.Join(dir, "pod.yml"))
	} else {
		args = utils.Compact("kube", "down",
			path.Join(dir, "pod.yml"))
	}
	fmt.Fprintf(os.Stderr, "+ podman %q\n", args)
	cmd := exec.Command("podman", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
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

func (depl *Deployment) RunHooks(ctx context.Context, when string, extend_timeout time.Duration) error {
	for _, hook := range depl.Hooks {
		if hook.When != when {
			continue
		}
		if len(hook.Exec) < 1 {
			continue
		}

		var ctx1 context.Context
		var cancel context.CancelFunc
		if hook.TimeoutSec > 0 {
			ctx1, cancel = context.WithTimeout(ctx, time.Duration(hook.TimeoutSec*int64(time.Second)))
		} else if hook.TimeoutSec == 0 {
			ctx1 = ctx
		} else {
			ctx1, cancel = context.WithCancel(ctx)
		}

		go utils.ExtendTimeout(ctx1, extend_timeout)

		err := func() error {
			if cancel != nil {
				defer cancel()
			}

			log.Printf("%s hook: Run %v\n", when, hook.Exec)
			//  cmd := exec.Command("systemd-run",
			//  	append([]string{
			//  		"--scope",
			//  		"--pipe",
			//  		"--collect",
			//  		"--unit=" + fmt.Sprintf("hook-%s-%s", depl.DeploymentName, when),
			//  	}, hook.Exec...)...)
			cmd := exec.Command(hook.Exec[0], hook.Exec[1:]...)
			cmd.Env = append(cmd.Environ(), depl.Vars()...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			return cmd.Run()
		}()
		if err != nil {
			log.Printf("%s hook: ERROR %v", when, err)
			return err
		}
	}
	log.Printf("%s hook: Completed hooks\n", when)
	return nil
}
