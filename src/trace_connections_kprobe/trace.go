package trace_connections

import (
	_ "embed"

	"context"
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"sync/atomic"
	"time"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/ringbuf"
	"kernel.org/pub/linux/libs/security/libcap/cap"
)

// To install bpf2go: go get -tool github.com/cilium/ebpf/cmd/bpf2go
// You need clang and llvm installed locally as well as libbpf-devel
//
//go:generate go tool bpf2go -tags linux -target $GOARCH trace trace.c

var bpfCode string

type Event struct {
	Timestamp uint64
	PID       uint32
	FD        int32
	Type      byte // 'A' = accepted, 'C' = closed
}

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

var ErrNoPermissions = fmt.Errorf("Requires root or capabilities: cap_bpf,cap_perfmon")

func StartTrace(ctx context.Context, fd int, requests <-chan Request) error {
	if os.Geteuid() != 0 {
		has_caps, err := hasCapBPF()
		if err != nil {
			return err
		} else if !has_caps {
			return ErrNoPermissions
		}
	}

	var coll traceObjects
	err := loadTraceObjects(&coll, nil)
	if err != nil {
		return fmt.Errorf("loading eBPF collection: %v", err)
	}

	// Pass target FD to eBPF global variable
	var targetFD int32 = int32(fd)
	if err := coll.TargetFdMap.Update(uint32(0), targetFD, ebpf.UpdateAny); err != nil {
		return fmt.Errorf("Setting target_fd: %v", err)
	}

	reading_events := false

	// Attach probes
	acceptLink, err := link.Kretprobe("sys_accept", coll.TraceAccept, nil)
	if err != nil {
		return fmt.Errorf("Attaching accept probe: %v", err)
	}
	defer func() {
		if !reading_events {
			acceptLink.Close()
		}
	}()

	accept4Link, err := link.Kretprobe("sys_accept4", coll.TraceAccept, nil)
	if err != nil {
		return fmt.Errorf("Attaching accept4 probe: %v", err)
	}
	defer func() {
		if !reading_events {
			accept4Link.Close()
		}
	}()

	closeLink, err := link.Kprobe("sys_close", coll.TraceClose, nil)
	if err != nil {
		return fmt.Errorf("Attaching close probe: %v", err)
	}
	defer func() {
		if !reading_events {
			closeLink.Close()
		}
	}()

	ringbuf, err := ringbuf.NewReader(coll.Events)
	if err != nil {
		return fmt.Errorf("Creating ringbuf: %v", err)
	}
	defer func() {
		if !reading_events {
			ringbuf.Close()
		}
	}()

	// Metrics state
	var inFlight int32
	var lastCloseTime atomic.Value // stores time.Time

	// Respond to user requests
	go func() {
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

	// Read BPF events
	reading_events = true
	go func() {
		defer acceptLink.Close()
		defer accept4Link.Close()
		defer closeLink.Close()
		defer ringbuf.Close()

		in_flight := map[uint64]bool{}

		for {
			rec, err := ringbuf.Read()
			if err != nil {
				if err == os.ErrDeadlineExceeded {
					continue
				} else if err == os.ErrClosed {
					return
				}
				log.Printf("[eBPF] Read eBPF event failed, %v", err)
				continue
			}

			var evt Event
			evt.Timestamp = binary.LittleEndian.Uint64(rec.RawSample[0:8])
			evt.PID = binary.LittleEndian.Uint32(rec.RawSample[8:12])
			evt.FD = int32(binary.LittleEndian.Uint32(rec.RawSample[12:16]))
			evt.Type = rec.RawSample[16]

			var key uint64 = (uint64(evt.PID) << 32) | uint64(evt.FD)

			now := time.Unix(0, int64(evt.Timestamp))

			switch evt.Type {
			case 'A':
				in_flight[key] = true
				atomic.StoreInt32(&inFlight, int32(len(in_flight)))
				fmt.Printf("[eBPF] event: %-20d %-25s %-8d %s\n", len(in_flight), "N/A", evt.PID, "ACCEPT")
			case 'C':
				if in_flight[key] {
					delete(in_flight, key)
					atomic.StoreInt32(&inFlight, int32(len(in_flight)))
					lastCloseTime.Store(now)
					fmt.Printf("[eBPF] event: %-20d %-25s %-8d %s\n", len(in_flight), now.Format("15:04:05.000"), evt.PID, "CLOSE")
				}
			}
		}
	}()

	return nil
}

func hasCapBPF() (bool, error) {
	cap_bpf, err := cap.GetBound(cap.BPF)
	if err != nil {
		return false, err
	}
	cap_perfmon, err := cap.GetBound(cap.PERFMON)
	if err != nil {
		return false, err
	}
	return cap_bpf && cap_perfmon, nil
}
