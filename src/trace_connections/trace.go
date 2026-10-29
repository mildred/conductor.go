package trace_connections

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

const bpftrace_program = `
  // Requires bpftrace >= v0.24 (ppid, getopt)

  BEGIN {
		@count = 0;
		printf("started fd=%d ppid=%d\n", getopt("fd"), getopt("ppid"));
	}

	tracepoint:syscalls:sys_enter_accept /ppid = getopt("ppid")/ {
		@accept_fd[tid] = args.fd
	}
	tracepoint:syscalls:sys_exit_accept /ppid = getopt("ppid")/ {
		if (@accept_fd[tid] == getopt("fd") && args.ret >= 0) {
			@count++;
			@established[pid, args.ret] = 1;
			printf("accept pid=%d count=%d comm=%s\n", pid, @count, comm)
		}
		delete(@accept4_fd[tid])
	}

	tracepoint:syscalls:sys_enter_accept4 /ppid = getopt("ppid")/ {
		@accept4_fd[tid] = args.fd
	}
	tracepoint:syscalls:sys_exit_accept4 /ppid = getopt("ppid")/ {
		if (@accept4_fd[tid] == getopt("fd") && args.ret >= 0) {
			@count++;
			@established[pid, args.ret] = 1;
			printf("accept pid=%d count=%d comm=%s\n", pid, @count, comm)
		}
		delete(@accept4_fd[tid])
	}

	tracepoint:syscalls:sys_enter_close /ppid = getopt("ppid")/ {
		if (@established[pid, args.fd]) {
			delete(@established[pid, args.fd]);
			@count--;
			printf("close pid=%d count=%d comm=%s\n", pid, @count, comm)
		}
	}
`

type Request struct {
	Response chan<- Metrics
}

type Metrics struct {
	InFlight      int
	LastCloseTime time.Time
}

func AskMetrics(requests chan<- Request) Metrics {
	resp := make(chan Metrics, 1)
	defer close(resp)
	requests <- Request{Response: resp}
	return <-resp
}

func StartTrace(ctx context.Context, fd int, pid_chan chan int, requests <-chan Request) error {
	cmd := exec.CommandContext(ctx, "bpftrace", "-e", bpftrace_program, "--", fmt.Sprintf("--ppid=%d", os.Getpid()), fmt.Sprintf("--fd=%d", fd))
	cmd.Stderr = os.Stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	err = cmd.Start()
	if err != nil {
		return err
	}

	ready := make(chan error, 0)

	// Metrics state
	var inFlight int32
	var lastCloseTime atomic.Value // stores time.Time

	// Respond to user requests
	go func() {
		<-ready
		for {
			select {
			case <-ctx.Done():
				return
			case req := <-requests:
				lastClose, _ := lastCloseTime.Load().(time.Time)
				req.Response <- Metrics{
					InFlight:      int(atomic.LoadInt32(&inFlight)),
					LastCloseTime: lastClose,
				}
			}
		}
	}()

	// Collect metrics
	go func() {
		var pid int
		defer cmd.Wait()
		defer close(ready)
		started := false

		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() && ctx.Err() == nil {
			line := scanner.Text()
			parts := strings.Split(line, " ")
			if len(parts) < 1 {
				continue
			} else if parts[0] == "started" {
				started = true
				close(ready)
				log.Printf("trace: started...", pid)
				pid = <-pid_chan
				log.Printf("trace: tracing pid %d...", pid)
			} else if parts[0] == "accept" {
				if getIntVar(parts, "pid", -1) == pid {
					count := getIntVar(parts, "count", -1)
					if count >= 0 {
						atomic.StoreInt32(&inFlight, int32(count))
					}
				}
			} else if parts[0] == "close" {
				if getIntVar(parts, "pid", -1) == pid {
					lastCloseTime.Store(time.Now())
					count := getIntVar(parts, "count", -1)
					if count >= 0 {
						atomic.StoreInt32(&inFlight, int32(count))
					}
				}
			}
		}

		errs := scanner.Err()
		if errs == io.EOF {
			errs = nil
		}

		if !started {
			err := cmd.Wait()
			if err != nil {
				errs = errors.Join(errs, err)
			}
			ready <- err
			err = nil
		}

		if err != nil {
			log.Printf("trace: ERROR %v", err)
		}
	}()

	err = <-ready

	return err
}

func getIntVar(vars []string, name string, default_value int) int {
	res, err := strconv.ParseInt(getVar(vars, name), 0, 0)
	if err != nil {
		return default_value
	} else {
		return int(res)
	}
}

func getVar(vars []string, name string) string {
	for _, v := range vars {
		parts := strings.SplitN(v, "=", 2)
		if len(parts) == 2 && parts[0] == name {
			return parts[1]
		}
	}
	return ""
}
