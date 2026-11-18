package deployment_util

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"time"

	"github.com/mildred/conductor.go/src/dirs"
	"github.com/mildred/conductor.go/src/utils"
	_ "github.com/mildred/conductor.go/src/utils"

	. "github.com/mildred/conductor.go/src/deployment"
)

func RemoveTimeout(ctx0 context.Context, deployment_name string, timeout, term_timeout time.Duration) (err error) {
	//
	// Delete specific deployment units
	//

	stop_units_order, e := AllUnitsStopOrder(ctx0, deployment_name)
	if e != nil {
		err = errors.Join(err, e)
	}

	var orders []int
	var units_by_order = map[int][]string{}
	for _, u := range stop_units_order {
		if !slices.Contains(orders, u.Order) {
			orders = append(orders, u.Order)
		}
		units_by_order[u.Order] = append(units_by_order[u.Order], u.Unit)
	}
	slices.Sort(orders)

	var stopped_units []string

	for _, order := range orders {
		units := utils.UniqueAnywhere(units_by_order[order])

		e := utils.StopExistingUnits(ctx0, timeout, term_timeout, units...)
		if e != nil {
			err = errors.Join(err, e)
		}

		stopped_units = append(stopped_units, units...)
	}

	//
	// Delete deployment directory
	//

	fmt.Fprintf(os.Stderr, "+ rm -rf %q\n", DeploymentDirByNameOnly(deployment_name))
	err = os.RemoveAll(DeploymentDirByNameOnly(deployment_name))
	if err != nil {
		return err
	}

	//
	// Delete generated units
	//

	systemd_run_dirs := []string{
		dirs.Join(dirs.RuntimeDir, "systemd", dirs.SystemdMode(), DeploymentServiceUnit(deployment_name)+".d"),
		dirs.Join(dirs.RuntimeDir, "systemd", dirs.SystemdMode(), DeploymentSocketUnit(deployment_name)),
		dirs.Join(dirs.RuntimeDir, "systemd", dirs.SystemdMode(), CGIFunctionSocketUnit(deployment_name)+".d"),
		dirs.Join(dirs.RuntimeDir, "systemd", dirs.SystemdMode(), CGIFunctionSocketUnit(deployment_name)),
		dirs.Join(dirs.RuntimeDir, "systemd", dirs.SystemdMode(), CGIFunctionServiceUnitSingle(deployment_name)),
		dirs.Join(dirs.RuntimeDir, "systemd", dirs.SystemdMode(), CGIFunctionServiceUnit(deployment_name, "")),
	}
	for _, u := range stopped_units {
		systemd_run_dirs = append(systemd_run_dirs,
			dirs.Join(dirs.RuntimeDir, "systemd", dirs.SystemdMode(), u+".d"),
			dirs.Join(dirs.RuntimeDir, "systemd", dirs.SystemdMode(), u),
		)
	}
	for _, systemd_run_dir := range systemd_run_dirs {
		fmt.Fprintf(os.Stderr, "+ rm -rf %q\n", systemd_run_dir)
		err = os.RemoveAll(systemd_run_dir)
		if err != nil {
			return err
		}
	}

	return nil
}
