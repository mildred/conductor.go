package deployment_internal

import (
	"context"
	"log"
	"time"

	"github.com/coreos/go-systemd/v22/daemon"

	"github.com/mildred/conductor.go/src/deployment"
)

func RunStartPostHooks(ctx context.Context, depl *deployment.Deployment) error {

	//
	// Run the post-start hook (via systemd-run specific scope)
	//
	//     this can generate prometheus config for monitoring
	//     (generic systemd-run command or script)
	//
	//     a generic script can wait for the pod to be ready
	//     (better as a command inside a container or HTTP request)
	//
	//     this can also perform data migrations
	//

	_, err := daemon.SdNotify(false, "EXTEND_TIMEOUT_USEC=60000000") // 60s
	if err != nil {
		return err
	}

	vars, err := depl.Vars(ctx)
	if err != nil {
		return err
	}

	log.Printf("executing post-start hooks...")
	err = depl.RunHooks(ctx, "post-start", depl.PartName, vars, 60*time.Second)
	if err != nil {
		log.Printf("post-start hooks failed, continuing...")
	}
	return nil
}
