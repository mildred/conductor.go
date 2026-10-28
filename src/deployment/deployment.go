package deployment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
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

func DeploymentServiceUnit(name string) string {
	return fmt.Sprintf("conductor-deployment@%s.service", name)
}

func DeploymentSocketUnit(name string) string {
	return fmt.Sprintf("conductor-deployment@%s.socket", name)
}

func DeploymentConfigUnit(name string) string {
	return fmt.Sprintf("conductor-deployment-config@%s.service", name)
}

func DeploymentSocketConfigUnit(name string) string {
	return fmt.Sprintf("conductor-deployment-socket-config@%s.service", name)
}

func DeploymentRunPath(name string) string {
	return path.Join(DeploymentRunDir, name)
}

func DeploymentSocketPath(name string) string {
	return path.Join(DeploymentRunDir, name, "stream.socket")
}

func DeploymentDirByName(name string, allow_dir bool, allow_service_name bool) (string, string, error) {
	if allow_dir && (strings.Contains(name, "/") || name == ".") {
		depl_dir, err := filepath.Abs(name)
		return depl_dir, path.Base(depl_dir), err
	}

	depl_dir := DeploymentDirByNameOnly(name)

	if !allow_service_name {
		return depl_dir, name, nil
	}

	if _, e := os.Stat(depl_dir); os.IsNotExist(e) {
		service_dir, err := service.ServiceDirByName(name)
		if err != nil {
			return "", "", fmt.Errorf("deployment %s is not found in %s and service %s could not be loaded (%v)", name, depl_dir, name, err)
		}

		deployments, err := List(ListOpts{FilterServiceDir: service_dir})
		if err != nil {
			return "", "", fmt.Errorf("while listing deployments of %s, %v", name, err)
		}

		if len(deployments) == 1 {
			depl_dir = DeploymentDirByNameOnly(deployments[0].DeploymentName)
		} else if len(deployments) > 1 {
			var deployment_names []string
			for _, d := range deployments {
				deployment_names = append(deployment_names, d.DeploymentName)
			}
			return "", "", fmt.Errorf("no unique deployment for service %s: %s", name, strings.Join(deployment_names, ", "))
		} else {
			return "", "", fmt.Errorf("no deployment for service %s", name)
		}
	}

	return depl_dir, name, nil
}

func DeploymentDirByNameOnly(name string) string {
	return path.Join(DeploymentRunDir, name)
}

type Deployment struct {
	*service.Service
	Seed                 *DeploymentSeed     `json:"-"`
	ServiceDir           string              `json:"service_dir"`
	ServiceId            string              `json:"service_id"`
	DeploymentName       string              `json:"conductor_deployment"`
	DeploymentDir        string              `json:"-"`
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
		DeploymentDir:        DeploymentDirByNameOnly(deployment_name),
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

func ReadDeploymentByName(name string, allow_dir_in_name, allow_service_name bool) (*Deployment, error) {
	path, depl_name, err := DeploymentDirByName(name, allow_dir_in_name, allow_service_name)
	if err != nil {
		return nil, err
	}

	return ReadDeployment(path, depl_name)
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
		return LoadDeployment(dir, path.Join(dir, ConfigName))
	}
}

func (depl *Deployment) TemplateProxyConfig(ctx context.Context, config_name string) error {
	if depl.ProxyConfigTemplate == "" {
		depl.TemplatedProxyConfig = nil
		return nil
	}

	log.Printf("prepare: Templating the proxy config\n")
	res, err := tmpl.RunTemplate(ctx, depl.ProxyConfigTemplate, append(
		depl.Vars(),
		"CONDUCTOR_PROXY_CONFIG="+config_name))
	if err != nil {
		return err
	}

	err = json.Unmarshal([]byte(res), &depl.TemplatedProxyConfig)
	if err != nil {
		return fmt.Errorf("could not read proxy JSON config, %v. JSON: %v", err, res)
	}

	return nil
}

func (depl *Deployment) TemplateAll(ctx context.Context) error {
	if depl.Pod != nil {
		err := depl.Pod.TemplatePod(ctx, depl)
		if err != nil {
			return err
		}
	}

	if depl.SocketActivated() {
		// TODO: better validate errors, do not run template twice
		err := depl.TemplateProxyConfig(ctx, "deployment-socket")
		if err != nil {
			return err
		}

		depl.ProxyConfigValue, err = depl.ProxyConfig(ctx, "deployment-socket")
		if err != nil {
			return err
		}
	}

	// TODO: better validate errors, do not run template twice
	err := depl.TemplateProxyConfig(ctx, "deployment")
	if err != nil {
		return err
	}

	depl.ProxyConfigValue, err = depl.ProxyConfig(ctx, "deployment")
	if err != nil {
		return err
	}

	return nil
}

func (depl *Deployment) ProxyConfig(ctx context.Context, config_name string) (caddy.ConfigItems, error) {
	var configs caddy.ConfigItems

	if config_name == "" {
		config_name = "deployment"
	}

	if depl.Pod != nil {
		cfg, err := depl.Pod.ProxyConfig(depl)
		if err != nil {
			return nil, fmt.Errorf("reading %s pod proxy config", config_name, err)
		}

		configs = append(configs, cfg...)
	}

	if depl.Function != nil {
		cfg, err := depl.Function.ProxyConfig(ctx, depl)
		if err != nil {
			return nil, fmt.Errorf("reading %s function proxy config", config_name, err)
		}

		configs = append(configs, cfg...)
	}

	if depl.ProxyConfigTemplate != "" {
		var cfgs caddy.ConfigItems
		err := tmpl.RunTemplateJSON(ctx, depl.ProxyConfigTemplate, append(
			depl.Vars(),
			"CONDUCTOR_PROXY_CONFIG="+config_name,
		), &cfgs)
		if err != nil {
			return nil, fmt.Errorf("while running the %v proxy-config template, %v", config_name, err)
		}

		for _, c := range cfgs {
			c.Source = config_name
			configs = append(configs, c)
		}
	}

	err := configs.SetDefaults()
	if err != nil {
		return nil, fmt.Errorf("setting proxy config defaults for %v, %v", config_name, err)
	}

	return configs, nil
}

func (depl *Deployment) Vars() []string {
	var excluded []string = nil
	if depl.Pod != nil {
		excluded = depl.Pod.ExcludeVars
	} else if depl.Function != nil {
		excluded = depl.Function.ExcludeVars
	}

	var vars []string = append(depl.Service.VarsExcluding(excluded),
		"CONDUCTOR_SERVICE_PART="+depl.PartName,
		"CONDUCTOR_DEPLOYMENT="+depl.DeploymentName,
		"CONDUCTOR_DEPLOYMENT_RUN_DIR="+DeploymentRunPath(depl.DeploymentName),
		"CONDUCTOR_DEPLOYMENT_SERVICE_ID="+depl.ServiceId,
		"CONDUCTOR_DEPLOYMENT_UNIT="+DeploymentServiceUnit(depl.DeploymentName),
		"CONDUCTOR_DEPLOYMENT_MAIN_UNIT="+depl.MainUnit(),
		"CONDUCTOR_DEPLOYMENT_SOCKET_UNIT="+depl.SocketUnit(),
		"CONDUCTOR_DEPLOYMENT_SOCKET_CONFIG_UNIT="+depl.SocketConfigUnit(),
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
			"CONDUCTOR_POD_SOCKET_ACTIVATED="+strconv.FormatBool(depl.Pod.SocketActivated),
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
			"CONDUCTOR_POD_SOCKET_ACTIVATED="+strconv.FormatBool(false),
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
			"CONDUCTOR_POD_SOCKET_ACTIVATED="+strconv.FormatBool(false),
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
	return LoadDeployment(dir, path.Join(dir, ConfigName))
}

func LoadDeployment(deployment_dir, fname string) (*Deployment, error) {
	// log.Printf("Read file %s\n", fname)
	f, err := os.Open(fname)
	if err != nil {
		return nil, err
	}

	res := &Deployment{}
	err = json.NewDecoder(f).Decode(res)

	res.Service.BasePath = res.ServiceDir
	res.Service.Id = res.ServiceId
	res.DeploymentDir = deployment_dir

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

func (depl *Deployment) FindPodIPAddressContainer(id string) (string, error) {
	data, err := exec.Command("podman", "container", "inspect", id).Output()
	if ee, ok := err.(*exec.ExitError); ok {
		return "", fmt.Errorf("could not execute podman container inspect %s: %v", id, err, string(ee.Stderr))
	} else if err != nil {
		return "", fmt.Errorf("could not execute podman container inspect %s: %v", id, err)
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

	for _, cont := range containers {
		for _, net := range cont.NetworkSettings.Networks {
			if net.IPAddress != "" {
				return net.IPAddress, nil
			}
		}
	}

	return "", nil
}

func (depl *Deployment) FindPodIPAddressOnce() (string, error) {
	data, err := exec.Command("podman", "pod", "inspect", depl.PodName).Output()
	if ee, ok := err.(*exec.ExitError); ok {
		return "", fmt.Errorf("could not execute podman pod inspect %s: %v (%s)", depl.PodName, err, string(ee.Stderr))
	} else if err != nil {
		return "", fmt.Errorf("could not execute podman pod inspect %s: %v", depl.PodName, err)
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

	for _, cont := range pod.Containers {
		address, e := depl.FindPodIPAddressContainer(cont.Id)
		if e != nil {
			err = errors.Join(err, e)
		} else if address != "" {
			return address, nil
		}
	}

	if err == nil {
		err = fmt.Errorf("could not find pod IP address")
	}

	return "", err
}

func (depl *Deployment) FindPodIPAddress(ctx context.Context) (string, error) {
	var err error
	var address string

	err = utils.ExponentialSleep(ctx, 50*time.Millisecond, 2*time.Second, 20*time.Second, func(elapsed, delay time.Duration) error {
		address, err = depl.FindPodIPAddressOnce()
		if err == nil {
			return utils.STOP
		} else {
			log.Printf("No IP address found (%v), will retry in %v...", err, delay)
			return nil
		}
	})
	if err != nil {
		return "", err
	}

	return address, err
}

func (depl *Deployment) ConfigUnit() string {
	return DeploymentConfigUnit(depl.DeploymentName)
}

func (depl *Deployment) SocketActivated() bool {
	return depl.Pod != nil && depl.Pod.SocketActivated
}

func (depl *Deployment) ServiceUnit() string {
	return DeploymentServiceUnit(depl.DeploymentName)
}

func (depl *Deployment) SocketUnit() string {
	if depl.SocketActivated() {
		return DeploymentSocketUnit(depl.DeploymentName)
	} else {
		return ""
	}
}

func (depl *Deployment) SocketConfigUnit() string {
	if depl.SocketActivated() {
		return DeploymentSocketConfigUnit(depl.DeploymentName)
	} else {
		return ""
	}
}

func (depl *Deployment) MainUnit() string {
	if depl.SocketActivated() {
		return DeploymentSocketUnit(depl.DeploymentName)
	} else {
		return DeploymentServiceUnit(depl.DeploymentName)
	}
}

func DeploymentMainUnit(s *service.Service, part_name string, deployment_name string) string {
	pod := s.Pods.FindPod(part_name)
	if pod != nil && pod.SocketActivated {
		return DeploymentSocketUnit(deployment_name)
	}

	return DeploymentServiceUnit(deployment_name)
}
