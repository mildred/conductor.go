package deployment_internal

import (
	"fmt"
	"log"

	"github.com/coreos/go-systemd/v22/daemon"
)

func SdNotifyOrLog(status string) {
	notify := fmt.Sprintf("STATUS=%s", status)
	_, err := daemon.SdNotify(false, notify)
	if err != nil {
		log.Printf("ERROR: systemd-notify %s failed: %v", notify, err)
	}
}
