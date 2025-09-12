//go:build !(dragonfly || freebsd || linux || netbsd)

package deployment_internal

import (
	"context"
	"fmt"

	. "github.com/mildred/conductor.go/src/deployment"
)

func StartSDActivateFunction(ctx context.Context, depl *Deployment, f *DeploymentFunction) error {
	return fmt.Errorf("sdactivate not supported on this platform")
}
