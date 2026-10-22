package utils

import (
	"context"
	"errors"
	"time"
)

// Delay returns nil after the specified duration or error if interrupted.
// https://stackoverflow.com/a/72066605
func Delay(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	select {
	case <-ctx.Done():
		t.Stop()
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

var STOP = errors.New("STOP")

func ExponentialSleep(ctx context.Context, init, max_sleep, timeout time.Duration, action func(elapsed time.Duration, delay time.Duration) error) error {
	elapsed := 0 * time.Second
	delay := init
	if max_sleep > 0 && delay > max_sleep {
		delay = max_sleep
	}

	if e := ctx.Err(); e != nil {
		return e
	}

	err := action(elapsed, delay)
	if err == STOP {
		return nil
	} else if err != nil {
		return err
	}

	for timeout == 0 || elapsed < timeout {
		if e := ctx.Err(); e != nil {
			return e
		}

		Delay(ctx, delay)
		elapsed += delay
		delay = delay * 2
		if max_sleep > 0 && delay > max_sleep {
			delay = max_sleep
		}

		if e := ctx.Err(); e != nil {
			return e
		}

		err = action(elapsed, delay)
		if err == STOP {
			return nil
		} else if err != nil {
			return err
		}
	}

	return nil
}
