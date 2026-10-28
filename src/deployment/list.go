package deployment

import (
	"errors"
	"os"
	"path"
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
