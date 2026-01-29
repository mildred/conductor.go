package deployment_util

import (
	"context"
	"errors"
	"os"
	"path"

	"github.com/coreos/go-systemd/v22/dbus"

	"github.com/mildred/conductor.go/src/utils"

	. "github.com/mildred/conductor.go/src/deployment"
)

type ListOpts struct {
	FilterServiceDir     string
	FilterDeploymentName string
	FilterServiceId      string
	FilterPartName       *string
	FilterPartId         string
}

func List(opts ListOpts) ([]*Deployment, error) {
	var errs error

	entries, err := os.ReadDir(DeploymentRunDir)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	var res []*Deployment
	for _, ent := range entries {
		depl, err := ReadDeployment(path.Join(DeploymentRunDir, ent.Name()), ent.Name())
		if err != nil {
			errs = errors.Join(errs, err)
			continue
		}

		if opts.FilterServiceDir != "" && opts.FilterServiceDir != depl.ServiceDir {
			continue
		}

		if opts.FilterDeploymentName != "" && opts.FilterDeploymentName != depl.DeploymentName {
			continue
		}

		if opts.FilterServiceId != "" && opts.FilterServiceId != depl.ServiceId {
			continue
		}

		if opts.FilterPartId != "" && opts.FilterPartId != depl.PartId {
			continue
		}

		if opts.FilterPartName != nil && *opts.FilterPartName != depl.PartName {
			continue
		}

		res = append(res, depl)
	}

	return res, errs
}

func ListUnitStatus(ctx context.Context, deployments []*Deployment, config_unit bool) ([]dbus.UnitStatus, error) {
	sd, err := utils.NewSystemdClient(ctx)
	if err != nil {
		return nil, err
	}

	var deployment_units []string
	for _, depl := range deployments {
		if config_unit {
			deployment_units = append(deployment_units, DeploymentConfigUnit(depl.DeploymentName))
		} else {
			deployment_units = append(deployment_units, DeploymentUnit(depl.DeploymentName))
		}
	}

	statuses, err := sd.ListUnitsByNamesContext(ctx, deployment_units)
	if err != nil {
		return nil, err
	}

	var res []dbus.UnitStatus
	for _, depl := range deployments {
		var status dbus.UnitStatus

		for _, st := range statuses {
			if config_unit && st.Name == DeploymentConfigUnit(depl.DeploymentName) {
				status = st
				break
			} else if !config_unit && st.Name == DeploymentUnit(depl.DeploymentName) {
				status = st
				break
			}
		}

		res = append(res, status)
	}

	return res, nil
}
