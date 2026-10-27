package utils

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"

	"github.com/alessio/shellescape"
)

func AddPrefix(logger *log.Logger, format string, args ...any) func() {
	old_prefix := log.Prefix()
	log.SetPrefix(old_prefix + fmt.Sprintf(format, args...))
	return func() {
		log.SetPrefix(old_prefix)
	}
}

func ReplacePrefix(logger *log.Logger, format string, args ...any) func() {
	old_prefix := log.Prefix()
	log.SetPrefix(fmt.Sprintf(format, args...))
	return func() {
		log.SetPrefix(old_prefix)
	}
}

var LogCommandsToLogger bool = true

func LogCommand(ctx context.Context, executable string, args ...string) *exec.Cmd {
	if LogCommandsToLogger {
		log.Printf("+ %s", shellescape.QuoteCommand(append([]string{executable}, args...)))
	} else {
		fmt.Fprintf(os.Stderr, "+ %s", shellescape.QuoteCommand(append([]string{executable}, args...)))
	}
	return exec.CommandContext(ctx, executable, args...)
}
