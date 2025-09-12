package api

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

func RunServer() error {
	log.SetOutput(os.Stderr)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	server := &idlehttp.Server{
		Idle: idlehttp.NewIdleTracker(ctx, 5*time.Second),
		Server: http.Server{
			Handler: http.HandlerFunc(handleRequest),
		},
	}

	return server.ServeIdle(0)
}

func handleRequest(w http.ResponseWriter, r *http.Request) {
	log.Printf("Request handled")
	w.Header().Set("X-Hello", "World")
	fmt.Fprintf(w, "Hello, World! (sdactivated)")
}
