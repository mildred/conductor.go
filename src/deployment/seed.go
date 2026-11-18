package deployment

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/mildred/conductor.go/src/parts"
	"github.com/mildred/conductor.go/src/service"
)

type DeploymentSeed struct {
	ServiceDir string                   `json:"service_dir"`
	ServiceId  string                   `json:"service_id"`
	PartName   string                   `json:"part_name"`
	PartType   parts.PartType           `json:"part_type"`
	PartId     string                   `json:"part_id"`
	IsPod      bool                     `json:"is_pod"`
	IsFunction bool                     `json:"is_function"`
	IsDaemon   bool                     `json:"is_daemon"`
	IsSocket   bool                     `json:"is_socket"`
	Once       bool                     `json:"once"`
	OtherParts map[string]*OtherPart    `json:"other_parts,omitempty"`
	Pod        *service.ServicePod      `json:"-"`
	Function   *service.ServiceFunction `json:"-"`
	Daemon     *service.ServiceDaemon   `json:"-"`
	Socket     *service.ServiceSocket   `json:"-"`
}

const SeedName = "conductor-deployment-seed.json"

func (seed *DeploymentSeed) UnmarshalJSON(data []byte) error {
	type plainDeploymentSeed DeploymentSeed
	var tmp plainDeploymentSeed = plainDeploymentSeed(*seed)

	err := json.Unmarshal(data, &tmp)
	if err != nil {
		return err
	}

	*seed = DeploymentSeed(tmp)

	if seed.PartType == parts.NoPartType {
		if seed.IsPod {
			seed.PartType = parts.PodType
		} else if seed.IsDaemon {
			seed.PartType = parts.DaemonType
		} else if seed.IsFunction {
			seed.PartType = parts.FunctionType
		} else {
			return fmt.Errorf("Missing part_type when unmarshalling deployment seed")
		}
	}

	return nil
}

func (seed DeploymentSeed) Suffix() string {
	if seed.PartName == "" {
		return ""
	} else {
		return "-" + seed.PartName
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

func SeedFromService(ctx context.Context, svc *service.Service, part_name string) (*DeploymentSeed, error) {
	part_id, err := svc.PartId(ctx, part_name)
	if err != nil {
		return nil, err
	}

	part := svc.FindPart(part_name)

	seed := &DeploymentSeed{
		ServiceDir: svc.BasePath,
		ServiceId:  svc.Id,
		PartName:   part_name,
		PartId:     part_id,
		PartType:   part.PartType(),
	}

	if part.PartType() == parts.PodType {
		pod := part.(*service.ServicePod)
		seed.IsPod = true
		seed.Pod = pod
		return seed, nil
	} else if part.PartType() == parts.FunctionType {
		f := part.(*service.ServiceFunction)
		seed.IsFunction = true
		seed.Function = f
		return seed, nil
	} else if part.PartType() == parts.DaemonType {
		d := part.(*service.ServiceDaemon)
		seed.IsDaemon = true
		seed.Daemon = d
		return seed, nil
	} else if part.PartType() == parts.SocketType {
		s := part.(*service.ServiceSocket)
		seed.IsSocket = true
		seed.Socket = s
		seed.Once = s.Once
		return seed, nil
	}

	if part_name == "" {
		return nil, fmt.Errorf("Cannot find default service pod or function")
	} else {
		return nil, fmt.Errorf("Cannot find service pod or function %q", part_name)
	}
}
