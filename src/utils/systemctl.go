package utils

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/alessio/shellescape"
	"github.com/taigrr/systemctl"
	"github.com/taigrr/systemctl/properties"

	"github.com/mildred/conductor.go/src/dirs"
)

func RunWithJournal(ctx context.Context, cmd *exec.Cmd, unfiltered_units ...string) error {
	current_unit := os.Getenv("CONDUCTOR_SYSTEMD_UNIT")
	var units []string
	for _, u := range unfiltered_units {
		if u != current_unit {
			units = append(units, u)
		}
	}

	if len(units) > 0 {
		ctx1, cancel := context.WithCancel(ctx)
		defer cancel()

		args := []string{dirs.SystemdModeFlag(), "-f", "--lines=0", "-o", "cat"}
		for _, unit := range units {
			args = append(args, "-u", unit)
		}
		journal := exec.CommandContext(ctx1, "journalctl", args...)
		stdout, err := journal.StdoutPipe()
		if err != nil {
			return fmt.Errorf("Error piping journalctl %s, %v", shellescape.QuoteCommand(args), err)
		}
		journal.Stderr = os.Stderr
		err = journal.Start()
		if err != nil {
			return fmt.Errorf("Error starting journalctl %s, %v", shellescape.QuoteCommand(args), err)
		}

		go func() {
			scanner := bufio.NewScanner(stdout)
			for scanner.Scan() {
				log.Printf(": %s", scanner.Text())
			}
		}()

		defer func() {
			journal.Process.Signal(syscall.SIGTERM)
			journal.Process.Signal(syscall.SIGKILL)
		}()
	}

	return cmd.Run()
}

func StopExistingUnit(ctx context.Context, timeout time.Duration, unit_name string) error {
	load_state, err := systemctl.Show(ctx, unit_name, properties.LoadState, systemctl.Options{UserMode: !dirs.AsRoot})
	has_unit := load_state == "loaded"
	if err != nil {
		return err
	}

	_, err = StopUnit(ctx, timeout, has_unit, false, unit_name)
	return err
}

func StopUnit(ctx0 context.Context, timeout time.Duration, fatal_error bool, no_block bool, units ...string) (timeouted bool, err error) {
	var cancel context.CancelFunc = func() {}
	var ctx = ctx0
	if timeout != 0 {
		ctx, cancel = context.WithTimeout(ctx0, timeout)
		defer cancel()
	}

	var opts = []string{dirs.SystemdModeFlag(), "stop"}
	if no_block {
		opts = append(opts, "--no-block")
	}

	cmd := LogCommand(ctx, "systemctl", append(opts, units...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if !fatal_error {
		cmd.Stderr = nil
	}
	err = RunWithJournal(ctx, cmd, units...)
	if err != nil && (fatal_error || err == ctx.Err()) {
		return err == ctx.Err(), err
	}

	return false, nil
}

func KillUnit(ctx0 context.Context, timeout time.Duration, fatal_error bool, signal string, reset_failed bool, units ...string) (timeouted bool, err error) {
	var cancel context.CancelFunc = func() {}
	var ctx = ctx0
	if timeout != 0 {
		ctx, cancel = context.WithTimeout(ctx0, timeout)
		defer cancel()
	}

	var opts = []string{dirs.SystemdModeFlag(), "kill"}
	if signal != "" {
		opts = append(opts, "--signal="+signal)
	}

	cmd := LogCommand(ctx, "systemctl", append(opts, units...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if !fatal_error {
		cmd.Stderr = nil
	}
	if signal == "SIGKILL" {
		err = cmd.Run()
	} else {
		err = RunWithJournal(ctx, cmd, units...)
	}
	if err != nil && (fatal_error || err == ctx.Err()) {
		return err == ctx.Err(), err
	}

	if reset_failed {
		cmd = LogCommand(ctx0, "systemctl", append([]string{dirs.SystemdModeFlag(), "reset-failed"}, units...)...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		if err != nil {
			return false, err
		}
	}

	return false, nil
}

func StopExistingUnits(ctx context.Context, timeout, term_timeout time.Duration, units ...string) error {
	var err error
	load_states := MapHash(units, func(u string) (string, string) {
		load_state, e := systemctl.Show(ctx, u, properties.LoadState, systemctl.Options{UserMode: !dirs.AsRoot})
		if e != nil {
			errors.Join(err, e)
		}
		return u, load_state
	})
	if err != nil {
		return err
	}

	found_units := Filter(units, func(u string) bool {
		load_state, _ := load_states[u]
		return load_state != "not-found" && load_state != ""
	})

	socket_units := Filter(found_units, func(u string) bool {
		return strings.HasSuffix(u, ".socket")
	})

	non_socket_units := Filter(found_units, func(u string) bool {
		return !strings.HasSuffix(u, ".socket")
	})

	ordered_units := append(append([]string{}, socket_units...), non_socket_units...)

	//
	// First stop service and socket units all at once
	//

	_, e := StopUnit(ctx, timeout, false, true, found_units...)
	if err != nil {
		err = errors.Join(err, e)
	}

	//
	// Then stop all others units, starting by socket units
	//

	for _, unit := range ordered_units {
		loaded := load_states[unit] == "loaded"
		timeouted, e := StopUnit(ctx, timeout, loaded, false, unit)
		if e != nil {
			err = errors.Join(err, e)
		}

		//
		// Send TERM signal to service unit if it failed to stop
		//

		if strings.HasSuffix(unit, ".service") && timeouted && term_timeout != 0 {
			timeouted, e = KillUnit(ctx, term_timeout, loaded, "", false, unit)
		}

		//
		// Send KILL signal to service unit if it failed to stop and reset-failed
		//

		if strings.HasSuffix(unit, ".service") && timeouted {
			timeouted, e = KillUnit(ctx, 0, loaded, "SIGKILL", true, unit)
		}
	}

	return err
}
