package deployment_util

import (
	"context"

	"github.com/coreos/go-systemd/v22/dbus"

	"github.com/mildred/conductor.go/src/utils"

	. "github.com/mildred/conductor.go/src/deployment"
)

func ListUnitStatus(ctx context.Context, deployments []*Deployment, config_unit bool) ([]dbus.UnitStatus, error) {
	sd, err := utils.NewSystemdClient(ctx)
	if err != nil {
		return nil, err
	}

	var deployment_units []string
	for _, depl := range deployments {
		if config_unit {
			deployment_units = append(deployment_units, depl.ConfigUnit())
		} else {
			deployment_units = append(deployment_units, depl.MainUnit())
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
			if config_unit && st.Name == depl.ConfigUnit() {
				status = st
				break
			} else if !config_unit && st.Name == depl.MainUnit() {
				status = st
				break
			}
		}

		res = append(res, status)
	}

	return res, nil
}
