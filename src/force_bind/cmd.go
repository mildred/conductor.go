package force_bind

import (
	"context"
	"flag"
	"log"
)

func Command(ctx context.Context) error {
	var opts ForceBindOpts
	var permissive bool

	opts.Rules = &Rules{}

	flag.Var(opts.Rules, "bind", "Bind rule in form of IP[/PREFIX][:PORT]=RULE, RULE can be ADDRESS[:PORT] or fd=N or sd=N")
	flag.BoolVar(&permissive, "permissive", false, "Allow all bind attempts outside of the rules")
	flag.Parse()

	opts.PreventUnknownBind = !permissive

	err := ForceBindCommand(ctx, log.Default(), flag.Args(), opts)
	if err != nil {
		return err
	}

	return nil
}
