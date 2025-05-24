package pipehttp

import (
	"context"
	"net"
)

// PipeListener is a hack to workaround the lack of http.Server.ServeConn.
// See: https://github.com/golang/go/issues/36673
type PipeListener struct {
	ctx    context.Context
	cancel context.CancelFunc
	ch     chan net.Conn
}

func NewPipeListener() *PipeListener {
	ctx, cancel := context.WithCancel(context.Background())
	return &PipeListener{
		ctx:    ctx,
		cancel: cancel,
		ch:     make(chan net.Conn, 64),
	}
}

func NewPipeListenerContext(ctx0 context.Context) *PipeListener {
	ctx, cancel := context.WithCancel(ctx0)
	return &PipeListener{
		ctx:    ctx,
		cancel: cancel,
		ch:     make(chan net.Conn, 64),
	}
}

func (ln *PipeListener) Accept() (net.Conn, error) {
	select {
	case conn, ok := <-ln.ch:
		if !ok {
			return nil, net.ErrClosed
		}
		return conn, nil
	case _ = <-ln.ctx.Done():
		return nil, net.ErrClosed
	}
}

func (ln *PipeListener) Close() error {
	if ln.ctx.Err() != nil {
		return net.ErrClosed
	}
	ln.cancel()
	close(ln.ch)
	return nil
}

// ServeConn enqueues a new connection. The connection will be returned in the
// next Accept call.
func (ln *PipeListener) ServeConn(conn net.Conn) error {
	if ln.ctx.Err() != nil {
		return net.ErrClosed
	}
	ln.ch <- conn
	return nil
}

func (ln *PipeListener) Addr() net.Addr {
	return pipeAddr{}
}

type pipeAddr struct{}

func (pipeAddr) Network() string {
	return "pipe"
}

func (pipeAddr) String() string {
	return "pipe"
}
