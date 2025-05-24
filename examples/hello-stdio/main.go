package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
)

func main() {
	log.SetOutput(os.Stderr)

	err := runMain()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runMain() error {
	server := &http.Server{
		Handler: http.HandlerFunc(handleRequest),
	}

	conn, err := net.FileConn(os.Stdin)
	if err != nil {
		return err
	}

	log.Printf("conn: %+v", conn)

	listener := newPipeListener()
	err = listener.ServeConn(conn)
	if err != nil {
		return err
	}

	log.Printf("listener: %+v", listener)

	err = server.Serve(listener)
	if err != nil {
		return err
	}

	return nil
}

func handleRequest(w http.ResponseWriter, r *http.Request) {
	log.Printf("Request handled")
	// Handle the request and write the response
	r.Header.Set("Hello", "World")
	fmt.Fprintf(w, "Hello, World!")
}

// pipeListener is a hack to workaround the lack of http.Server.ServeConn.
// See: https://github.com/golang/go/issues/36673
type pipeListener struct {
	ch     chan net.Conn
	closed bool
	mu     sync.Mutex
}

func newPipeListener() *pipeListener {
	return &pipeListener{
		ch: make(chan net.Conn, 64),
	}
}

func (ln *pipeListener) Accept() (net.Conn, error) {
	conn, ok := <-ln.ch
	if !ok {
		return nil, net.ErrClosed
	}
	return conn, nil
}

func (ln *pipeListener) Close() error {
	ln.mu.Lock()
	defer ln.mu.Unlock()

	if ln.closed {
		return net.ErrClosed
	}
	ln.closed = true
	close(ln.ch)
	return nil
}

// ServeConn enqueues a new connection. The connection will be returned in the
// next Accept call.
func (ln *pipeListener) ServeConn(conn net.Conn) error {
	ln.mu.Lock()
	defer ln.mu.Unlock()

	if ln.closed {
		return net.ErrClosed
	}
	ln.ch <- conn
	return nil
}

func (ln *pipeListener) Addr() net.Addr {
	return pipeAddr{}
}

type pipeAddr struct{}

func (pipeAddr) Network() string {
	return "pipe"
}

func (pipeAddr) String() string {
	return "pipe"
}
