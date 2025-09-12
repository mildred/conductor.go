//go:build dragonfly || freebsd || linux || netbsd

package deployment_internal

import (
	"context"
	"fmt"
	"os"
	"syscall"

	"github.com/coreos/go-systemd/v22/activation"

	"github.com/mildred/conductor.go/src/utils"

	. "github.com/mildred/conductor.go/src/deployment"
)

func StartSDActivateFunction(ctx context.Context, depl *Deployment, f *DeploymentFunction) error {
	if f.NoResponseHeaders {
		return fmt.Errorf("http-stdio function incompatible with no_response_headers")
	}

	if len(f.ResponseHeaders) > 0 {
		return fmt.Errorf("http-stdio function incompatible with response_headers (%v)", f.ResponseHeaders)
	}

	listeners := activation.Files(false)
	if len(listeners) < 1 {
		return fmt.Errorf("unexpected number of socket activation fds: %d < %d", len(listeners), 1)
	}

	for _, f := range listeners {
		err := utils.SetCloseOnExec(int(f.Fd()), false)
		if err != nil {
			return err
		}
	}

	env := append(os.Environ(), depl.Vars()...)

	return syscall.Exec(f.Exec[0], f.Exec[1:], env)
}
