package idlehttp

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/coreos/go-systemd/v22/activation"
)

type IdleTracker struct {
	Context context.Context
	mu      sync.Mutex
	active  map[net.Conn]bool
	idle    time.Duration
	timer   *time.Timer
}

type Server struct {
	http.Server
	Idle *IdleTracker
}

func (s *Server) ServeIdle(fdnum int) error {
	if s.ConnState != nil {
		return fmt.Errorf("ConnState should be nil")
	}
	s.ConnState = s.Idle.ConnState

	if s.BaseContext == nil {
		s.BaseContext = func(l net.Listener) context.Context { return s.Idle.Context }
	}

	return s.Idle.ServeIdle(&s.Server, fdnum)
}

func NewIdleTracker(ctx context.Context, idle time.Duration) *IdleTracker {
	return &IdleTracker{
		Context: ctx,
		active:  make(map[net.Conn]bool),
		idle:    idle,
		timer:   time.NewTimer(idle),
	}
}

func (t *IdleTracker) ConnState(conn net.Conn, state http.ConnState) {
	t.mu.Lock()
	defer t.mu.Unlock()

	oldActive := len(t.active)
	switch state {
	case http.StateNew, http.StateActive, http.StateHijacked:
		t.active[conn] = true
		// stop the timer if we transitioned to idle
		if oldActive == 0 {
			t.timer.Stop()
		}
	case http.StateIdle, http.StateClosed:
		delete(t.active, conn)
		// Restart the timer if we've become idle
		if oldActive > 0 && len(t.active) == 0 {
			t.timer.Reset(t.idle)
		}
	}
}

func (t *IdleTracker) Done() <-chan time.Time {
	return t.timer.C
}

func (t *IdleTracker) Shutdown(server *http.Server, ctx context.Context) error {
	select {
	case <-t.Done():
	case <-t.Context.Done():
	}
	return server.Shutdown(ctx)
}

func (t *IdleTracker) GoShutdown(server *http.Server) {
	go func() {
		ctx := t.Context
		if ctx.Err() != nil {
			ctx = context.Background()
		}
		err := t.Shutdown(server, ctx)
		if err != nil {
			log.Fatalf("error shutting down: %v\n", err)
		}
	}()
}

func (t *IdleTracker) ServeIdle(server *http.Server, listenernum int) error {
	listeners, err := activation.Listeners()
	if err != nil {
		return err
	}

	if len(listeners) < listenernum+1 {
		return fmt.Errorf("unexpected number of socket activation fds: %d < %d", len(listeners), listenernum+1)
	}

	t.GoShutdown(server)

	return server.Serve(listeners[listenernum])
}
