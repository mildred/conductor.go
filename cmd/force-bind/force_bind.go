package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/mildred/conductor.go/src/force_bind"
)

func Main(ctx context.Context) error {
	var opts force_bind.ForceBindOpts
	var permissive bool

	opts.Rules = &force_bind.Rules{}

	flag.Var(opts.Rules, "bind", "Bind rule in form of IP[/PREFIX][:PORT]=ADDRESS[:PORT]")
	flag.BoolVar(&permissive, "permissive", false, "Allow all bind attempts outside of the rules")
	flag.Parse()

	opts.PreventUnknownBind = !permissive

	err := force_bind.ForceBindCommand(ctx, log.Default(), flag.Args(), opts)
	if err != nil {
		return err
	}

	return nil
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := Main(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
