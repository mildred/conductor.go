package policies

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/coreos/go-systemd/v22/activation"
)

func httpCheckPolicies(w http.ResponseWriter, req *http.Request) error {
	policies, err := LoadPolicies()
	if err != nil {
		return err
	}

	for _, policy_spec := range req.Header.Values("Conductor-Policy") {
		policy_parts := strings.SplitN(policy_spec, "/", 2)
		policy_name := policy_parts[0]
		authorization := ""
		if len(policy_parts) >= 2 {
			authorization = policy_parts[1]
		}

		policy := policies.ByName[policy_name]

		if policy == nil {
			return fmt.Errorf("missing policy %s", policy_name)
		}

		res, err, _ := policy.Matching(&MatchContext{
			Policies: policies,
			Request:  req,
		}, authorization, nil)
		if err != nil {
			return err
		}

		if !res {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprintf(w, "Unauthorized")
			return nil
		}
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}

func PolicyServer(w http.ResponseWriter, req *http.Request) {
	err := httpCheckPolicies(w, req)
	if err != nil {
		w.WriteHeader(500)
		log.Printf("INTERNAL ERROR: %v", err)
		fmt.Fprintf(w, "Internal error")
	}
}

func RunServer() error {
	ctx := context.Background()
	signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)

	listeners, err := activation.Listeners()
	if err != nil {
		return err
	}

	if len(listeners) != 1 {
		return fmt.Errorf("socket activation got %d sockets, expected 1", len(listeners))
	}

	server := &http.Server{
		Handler: http.HandlerFunc(PolicyServer),
	}

	//
	// Start server in background and collect the last error
	// (in background because the Serve() function returns early before shutdown
	// is complete)
	//

	http_err := make(chan error, 1)
	go func() {
		err = server.Serve(listeners[0])
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			http_err <- err
		}
		close(http_err)
	}()

	//
	// Wait for signal (then shutdown) or server to naturally stop
	//

	select {
	case <-ctx.Done():
		err = func() error {
			ctx_grace, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			return server.Shutdown(ctx_grace)
		}()
		if err != nil {
			return err
		}

	case err = <-http_err:
		return err
	}

	//
	// Return HTTP server error
	//

	return <-http_err
}
