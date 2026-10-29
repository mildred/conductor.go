package status_socket

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path"

	"github.com/mildred/conductor.go/src/deployment"
)

const LifeStatusSocketName string = "live-status.json"

func StartDeploymentStatus(ctx context.Context, depl *deployment.Deployment, get_status func() interface{}) error {
	socket_path := path.Join(depl.DeploymentDir, LifeStatusSocketName)

	err := os.RemoveAll(socket_path)
	if err != nil {
		return err
	}

	log := log.New(os.Stderr, fmt.Sprintf("[depl %s %s] ", depl.DeploymentName, LifeStatusSocketName), 0)

	var lc net.ListenConfig
	sock, err := lc.Listen(ctx, "unix", socket_path)
	if err != nil {
		return err
	}

	go func() {
		defer func() {
			err := sock.Close()
			if err != nil {
				log.Printf("Cannot close %s, %v", socket_path, err)
			}
			err = os.RemoveAll(socket_path)
			if err != nil {
				log.Printf("Cannot remove %s, %v", socket_path, err)
			}
		}()

		for {
			conn, err := sock.Accept()
			if err != nil {
				if err == ctx.Err() {
					break
				}

				log.Printf("ERROR: cannot accept, %v", err)
				continue
			}

			err = SendStatusResponse(ctx, conn, get_status)
			if err != nil {
				if err == ctx.Err() {
					break
				}

				log.Printf("ERROR: %v", err)
				continue
			}
		}
	}()

	return nil
}

func SendStatusResponse(ctx context.Context, conn net.Conn, get_status func() interface{}) error {
	defer conn.Close()

	json, err := json.Marshal(get_status())
	if err != nil {
		if err == ctx.Err() {
			return err
		}

		return fmt.Errorf("cannot encode JSON, %v", err)
	}

	_, err = io.Copy(conn, bytes.NewReader(json))
	if err != nil {
		if err == ctx.Err() {
			return err
		}

		return fmt.Errorf("cannot send status, %v", err)
	}

	return nil
}
