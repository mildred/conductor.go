package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/mildred/conductor.go/lib/pipehttp"
)

func main() {
	log.SetOutput(os.Stderr)

	err := runMain(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runMain(ctx context.Context) error {
	server := pipehttp.NewConnServer(&http.Server{
		Handler: http.HandlerFunc(handleRequest),
	})

	return server.ServeStdioConnAndShutdown()
}

func handleRequest(w http.ResponseWriter, r *http.Request) {
	log.Printf("Request handled")
	// Handle the request and write the response
	r.Header.Set("Hello", "World")
	fmt.Fprintf(w, "Hello, World!")
}
