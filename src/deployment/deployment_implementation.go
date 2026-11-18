package deployment

import (
	"context"
	"fmt"

	"github.com/mildred/conductor.go/src/parts"
)

type UnitOrder struct {
	Order int
	Unit  string
}

type DeploymentPartImplementation interface {
	AllServiceUnits() []string
	CreateDeploymentUnits(ctx context.Context) (reload bool, err error)
	AllUnitsStopOrder(context.Context) ([]UnitOrder, error)

	StartDeployment(ctx context.Context, depl *Deployment) error
	StartPostDeployment(ctx context.Context, depl *Deployment) error
	StopDeployment(ctx context.Context, depl *Deployment) error
}

var DeploymentPartImplementations = map[parts.PartType]func(depl *Deployment) DeploymentPartImplementation{}

type DefaultDeploymentImplementation struct {
	Depl *Deployment
}

func (d *DefaultDeploymentImplementation) AllServiceUnits() []string {
	return nil
}

func (d *DefaultDeploymentImplementation) CreateDeploymentUnits(context.Context) (bool, error) {
	return false, nil
}

func (d *DefaultDeploymentImplementation) AllUnitsStopOrder(context.Context) ([]UnitOrder, error) {
	return nil, nil
}

func (d *DefaultDeploymentImplementation) StartDeployment(ctx context.Context, depl *Deployment) error {
	return fmt.Errorf("cannot start %v deployment, not implemented", d.Depl.PartType)
}

func (d *DefaultDeploymentImplementation) StartPostDeployment(ctx context.Context, depl *Deployment) error {
	return nil
}

func (d *DefaultDeploymentImplementation) StopDeployment(ctx context.Context, depl *Deployment) error {
	return fmt.Errorf("cannot stop %v deployment, not implemented", d.Depl.PartType)
}
