package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/mildred/conductor.go/lib/pipehttp"
)

func main() {
	log.SetOutput(os.Stderr)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	err := runMain(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runMain(ctx context.Context) error {
	server := pipehttp.NewConnServer(&http.Server{
		Handler: http.HandlerFunc(handleRequest),
	})

	return server.ServeStdioConnAndShutdown(ctx)
}

func handleRequest(w http.ResponseWriter, r *http.Request) {
	log.Printf("Request handled")
	w.Header().Set("X-Hello", "World")
	fmt.Fprintf(w, "Hello, World!")
}
