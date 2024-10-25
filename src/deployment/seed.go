package deployment

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/mildred/conductor.go/src/service"
)

type DeploymentSeed struct {
	ServiceDir string `json:"service_dir"`
	PartName   string `json:"part_name"`
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
	pod := service.Pods.FindPod(part)
	if pod == nil {
		return nil, fmt.Errorf("Cannot find service part %q", part)
	}

	return &DeploymentSeed{
		ServiceDir: service.BasePath,
		PartName:   part,
	}, nil
}
