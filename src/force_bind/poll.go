package force_bind

import (
	"context"

	"golang.org/x/sys/unix"
)

func goPollFd(ctx context.Context, fd int) <-chan error {
	done := make(chan error)

	go func() {
		defer close(done)
		p := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
		for ctx.Err() == nil {
			n, err := unix.Poll(p, 1000)
			if err == unix.EINTR {
				continue
			} else if err != nil {
				done <- err
				return
			}
			if n > 0 && p[0].Revents&(unix.POLLIN|unix.POLLERR|unix.POLLHUP) != 0 {
				return // one readiness notification
			}
		}
	}()

	return done
}
