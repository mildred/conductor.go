package pipehttp

import (
	"context"
	"net"
	"net/http"
	"os"
)

type ConnServer struct {
	*http.Server
	oldConnState func(net.Conn, http.ConnState)
}

func NewConnServer(server *http.Server) (res *ConnServer) {
	res = &ConnServer{
		Server:       server,
		oldConnState: server.ConnState,
	}

	res.Server.ConnState = res.onConnState
	return
}

func (s *ConnServer) onConnState(c net.Conn, cs http.ConnState) {
	if s.oldConnState != nil {
		s.oldConnState(c, cs)
	}
	if cs == http.StateHijacked || cs == http.StateClosed {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		s.Shutdown(ctx)
	}
}

func (s *ConnServer) ServeConnAndShutdown(ctx context.Context, conn net.Conn) error {
	listener := NewPipeListenerContext(ctx)
	err := listener.ServeConn(conn)
	if err != nil {
		return err
	}

	if ctx != nil {
		s.BaseContext = func(l net.Listener) context.Context { return ctx }
	}

	err = s.Serve(listener)
	if err != nil && err != http.ErrServerClosed {
		return err
	}

	return nil
}

func (s *ConnServer) ServeStdioConnAndShutdown(ctx context.Context) error {
	conn, err := net.FileConn(os.Stdin)
	if err != nil {
		return err
	}

	return s.ServeConnAndShutdown(ctx, conn)
}
