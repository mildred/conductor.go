package deployment_internal

import (
	"context"
	"path"

	"github.com/mildred/conductor.go/src/deployment"
	"github.com/mildred/conductor.go/src/tmpl"
)

func Template(ctx context.Context, dir string, template string) error {
	depl, err := deployment.LoadDeployment(dir, path.Join(dir, deployment.ConfigName))
	if err != nil {
		return err
	}

	vars, err := depl.Vars(ctx)
	if err != nil {
		return err
	}

	return tmpl.RunTemplateStdout(context.Background(), template, vars)
}
