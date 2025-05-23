package deployment

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/mildred/conductor.go/src/service"
)

type DeploymentSeed struct {
	ServiceDir string                   `json:"service_dir"`
	ServiceId  string                   `json:"service_id"`
	PartName   string                   `json:"part_name"`
	PartId     string                   `json:"part_id"`
	IsPod      bool                     `json:"is_pod"`
	IsFunction bool                     `json:"is_function"`
	Pod        *service.ServicePod      `json:"-"`
	Function   *service.ServiceFunction `json:"-"`
}

const SeedName = "conductor-deployment-seed.json"

func (seed DeploymentSeed) Prefix() string {
	if seed.PartName == "" {
		return ""
	} else {
		return seed.PartName + "-"
	}
}

func ReadSeed(fname string) (*DeploymentSeed, error) {
	f, err := os.Open(fname)
	if err != nil {
		return nil, err
	}

	defer f.Close()

	res := &DeploymentSeed{}

	dec := json.NewDecoder(f)
	err = dec.Decode(res)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func SeedFromService(service *service.Service, part string) (*DeploymentSeed, error) {
	part_id, err := service.PartId(part)
	if err != nil {
		return nil, err
	}

	seed := &DeploymentSeed{
		ServiceDir: service.BasePath,
		ServiceId:  service.Id,
		PartName:   part,
		PartId:     part_id,
	}

	if pod := service.Pods.FindPod(part); pod != nil {
		seed.IsPod = true
		seed.Pod = pod
		return seed, nil
	}

	if f := service.Functions.FindFunction(part); f != nil {
		seed.IsFunction = true
		seed.Function = f
		return seed, nil
	}

	return nil, fmt.Errorf("Cannot find service pod or function %q", part)
}
