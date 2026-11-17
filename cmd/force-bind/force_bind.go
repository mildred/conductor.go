package main

import (
	"context"
	"fmt"
	"os"

	"github.com/mildred/conductor.go/src/force_bind"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := force_bind.Command(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
