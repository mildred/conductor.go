package utils

import (
	"bufio"
	"context"
	"log"
	"os"
	"os/exec"
	"sync"

	"github.com/alessio/shellescape"
)

func CommandLogOutput(cmd *exec.Cmd) func() {
	var wg sync.WaitGroup
	stdout, e := cmd.StdoutPipe()
	if e != nil {
		log.Printf("Failed to redirect stdout logs, %v", e)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else {
		cmd.Stderr = cmd.Stdout
		wg.Go(func() {
			scanner := bufio.NewScanner(stdout)
			for scanner.Scan() {
				log.Printf(": %s", scanner.Text())
			}
		})
	}
	return wg.Wait
}

func RunCommand(ctx context.Context, cmdline ...string) error {
	log.Printf("+ %s\n", shellescape.QuoteCommand(cmdline))
	cmd := exec.Command(cmdline[0], cmdline[1:]...)

	wait := CommandLogOutput(cmd)
	defer wait()

	return cmd.Run()
}

func LogCmd(format string, cmd *exec.Cmd, args ...any) {
	log.Printf(format, append([]any{shellescape.QuoteCommand(cmd.Args)}, args...)...)
}
