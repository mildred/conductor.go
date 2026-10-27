package utils

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/alessio/shellescape"
	"github.com/mildred/conductor.go/src/dirs"
)

func RunWithJournal(ctx context.Context, cmd *exec.Cmd, units ...string) error {
	ctx1, cancel := context.WithCancel(ctx)
	defer cancel()

	args := []string{dirs.SystemdModeFlag(), "-f", "--lines=0"}
	for _, unit := range units {
		args = append(args, "-u", unit)
	}
	journal := exec.CommandContext(ctx1, "journalctl", args...)
	journal.Stdout = os.Stderr
	journal.Stderr = os.Stderr
	err := journal.Start()
	if err != nil {
		return fmt.Errorf("Error executing journalctl %s, %v", shellescape.QuoteCommand(args), err)
	}

	return cmd.Run()
}
