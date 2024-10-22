package utils

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/coreos/go-systemd/v22/daemon"
)

func ExtendTimeout(ctx context.Context, extra_time time.Duration) {
	const loop_duration = 30 * time.Second
	for ctx.Err() == nil {
		extend := loop_duration + extra_time
		_, err := daemon.SdNotify(false, fmt.Sprintf("EXTEND_TIMEOUT_USEC=%d", extend.Microseconds()))
		if err != nil {
			log.Printf("Error extending systemd timeout: %s", err.Error())
		}

		ctx1, cancel := context.WithTimeout(ctx, loop_duration)
		<-ctx1.Done()
		cancel()
	}
}
