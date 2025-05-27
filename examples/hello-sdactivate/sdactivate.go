package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mildred/conductor.go/lib/idlehttp"
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
	idle := idlehttp.NewIdleTracker(5 * time.Second)
	server := &http.Server{
		Handler:   http.HandlerFunc(handleRequest),
		ConnState: idle.ConnState,
	}

	return idle.ServeIdle(server, 0)
}

func handleRequest(w http.ResponseWriter, r *http.Request) {
	log.Printf("Request handled")
	w.Header().Set("X-Hello", "World")
	fmt.Fprintf(w, "Hello, World! (sdactivated)")
}
