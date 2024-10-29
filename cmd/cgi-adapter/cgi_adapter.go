package main

import (
	"flag"
	"fmt"
	"os"

	. "github.com/mildred/conductor.go/src/cgi"
)

func main() {
	var cfg Config
	flag.IntVar(&cfg.PathInfoStrip, "path-info-strip", -1, "How many segments to strip to get the PATH_INFO")
	flag.Parse()
	var args = flag.Args()

	err := ExecCGI(&cfg, args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %e", err)
		os.Exit(1)
	}
}
