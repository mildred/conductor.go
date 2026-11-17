package force_bind

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/netip"
	"os"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"unsafe"

	"github.com/oraoto/go-pidfd"
	"github.com/seccomp/libseccomp-golang"
	"golang.org/x/sys/unix"
)

type ForceBindOpts struct {
	Rules              *Rules
	PreventUnknownBind bool
}

type Filter struct {
	*seccomp.ScmpFilter
	SyscallBind   seccomp.ScmpSyscall
	SyscallListen seccomp.ScmpSyscall
}

var UseProcMem = true

func NotifyFilter() (*Filter, error) {

	filter, err := seccomp.NewFilter(seccomp.ActAllow)
	if err != nil {
		return nil, err
	}

	syscall_bind, err := seccomp.GetSyscallFromName("bind")
	if err != nil {
		return nil, err
	}

	syscall_listen, err := seccomp.GetSyscallFromName("listen")
	if err != nil {
		return nil, err
	}

	err = filter.AddRule(syscall_bind, seccomp.ActNotify)
	if err != nil {
		return nil, err
	}

	err = filter.AddRule(syscall_listen, seccomp.ActNotify)
	if err != nil {
		return nil, err
	}

	return &Filter{filter, syscall_bind, syscall_listen}, nil
}

func ForceBindCommand(ctx context.Context, l *log.Logger, cmdline []string, opts ForceBindOpts) error {
	lprefix := l.Prefix()
	l.SetPrefix(lprefix + "[force-bind] ")
	defer l.SetPrefix(lprefix)

	for _, rule := range opts.Rules.Rules {
		l.Printf("Redirect bind(%s) to bind(%s)", rule.Matcher.String(), rule.RuleString())
	}

	fb := &forceBind{opts, nil}

	filter, err := NotifyFilter()
	if err != nil {
		return err
	}

	err = filter.Load()
	if err != nil {
		return err
	}

	notif_fd, err := filter.GetNotifFd()
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, cmdline[0], cmdline[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Start()
	if err != nil {
		return err
	}

	l.Printf("Started %s", strings.Join(cmdline, " "))

	var wg sync.WaitGroup
	ctx_handler, cancel_handler := context.WithCancel(ctx)
	wg.Go(func() {
		fb.handleNotifs(ctx_handler, &wg, l, notif_fd, filter)
	})

	err = cmd.Wait()
	if err != nil {
		cancel_handler()
		return err
	}

	log.Printf("Process terminated, shutting down...")

	cancel_handler()
	wg.Wait()

	return nil
}

type forceBind struct {
	ForceBindOpts
	ListenedFds []uintptr
}

func (fb *forceBind) handleNotifs(ctx context.Context, wg *sync.WaitGroup, l *log.Logger, notif_fd seccomp.ScmpFd, filter *Filter) {
	for ctx.Err() == nil {
		select {
		case <-ctx.Done():
			return
		case err := <-goPollFd(ctx, int(notif_fd)):
			if err != nil {
				l.Printf("FATAL: %v", err)
				continue
			}
		}

		notif, err := seccomp.NotifReceive(notif_fd)
		if err != nil {
			l.Printf("FATAL: %v", err)
		}

		wg.Go(func() {
			self_pid := os.Getpid()
			pid := notif.Pid
			resp := seccomp.ScmpNotifResp{
				ID:    notif.ID,
				Error: 0,
				Val:   0,
				Flags: seccomp.NotifRespFlagContinue,
			}

			tgid, err := PIDFromTID(int(pid))
			// l.Printf("[%v] notif received from pid=%v tgid=%v", self_pid, pid, tgid)
			if err != nil {
				l.Printf("[%d] FATAL: %v", pid, err)
				tgid = int(pid)
			}

			if pid == uint32(self_pid) || tgid == self_pid {
				// Allowing operation for outself
			} else if notif.Data.Syscall == filter.SyscallBind {
				err = fb.handleBind(ctx, l, notif_fd, notif, &resp)
				if err != nil {
					l.Printf("[%d] FATAL: %v", pid, err)
					resp.Error = int32(unix.ENOENT)
					resp.Flags = 0
				}

			} else if notif.Data.Syscall == filter.SyscallListen {
				err = fb.handleListen(l, notif_fd, notif, &resp)
				if err != nil {
					l.Printf("[%d] FATAL: %v", pid, err)
				}

			} else {
				l.Printf("[%d] syscall_%d(%v) detected", pid, notif.Data.Syscall, notif.Data.Args)
			}

			err = seccomp.NotifRespond(notif_fd, &resp)
			if err != nil {
				l.Printf("[%d] FATAL: %v", pid, err)
			}
		})
	}
}

func (fb *forceBind) handleListen(l *log.Logger, notif_fd seccomp.ScmpFd, notif *seccomp.ScmpNotifReq, resp *seccomp.ScmpNotifResp) error {
	pid := notif.Pid
	arg_sockfd := notif.Data.Args[0]
	arg_backlog := notif.Data.Args[1]

	var err error = nil
	contains := slices.ContainsFunc(fb.Rules.ListeningFds(), func(local_fd uintptr) bool {
		same, e := sameFile(local_fd, uintptr(pid), uintptr(arg_sockfd))
		if e != nil {
			err = errors.Join(err, e)
		}
		return same
	})
	if err != nil {
		l.Printf("[%d] listen(%d, %d) detected", pid, arg_sockfd, arg_backlog)
		return fmt.Errorf("could not find if fd %d is already listened", arg_sockfd)
	}

	if contains {
		l.Printf("[%d] listen(%d, %d) detected, prevent listening as socket is already listening", pid, arg_sockfd, arg_backlog)
		resp.Flags = 0
	} else {
		l.Printf("[%d] listen(%d, %d) detected, allowed to continue", pid, arg_sockfd, arg_backlog)
	}

	return nil
}

func (fb *forceBind) handleBind(ctx context.Context, l *log.Logger, notif_fd seccomp.ScmpFd, notif *seccomp.ScmpNotifReq, resp *seccomp.ScmpNotifResp) error {
	pid := notif.Pid
	arg_sockfd := notif.Data.Args[0]
	arg_sockaddr := notif.Data.Args[1]
	arg_socklen := notif.Data.Args[2]

	if arg_socklen <= 0 {
		l.Printf("[%d] bind(%d, 0x%x, %d) detected", pid, arg_sockfd, arg_sockaddr, arg_socklen)
		return fmt.Errorf("sockaddr length must be positive")
	}

	remote_sockaddr := unix.RemoteIovec{
		Base: uintptr(arg_sockaddr),
		Len:  int(arg_socklen),
	}

	var local_sockaddr unix.Iovec

	pidfd, err := pidfd.Open(int(pid), 0)
	if err != nil {
		l.Printf("[%d] bind(%d, 0x%x, %d) detected", pid, arg_sockfd, arg_sockaddr, arg_socklen)
		return fmt.Errorf("could not pidfd_open(%d), %v", pid, err)
	}

	var buf = make([]byte, arg_socklen)
	local_sockaddr = unix.Iovec{
		Base: (*byte)(unsafe.Pointer(&buf[0])),
		Len:  uint64(arg_socklen),
	}

	if UseProcMem {
		// TODO: better use /proc/PID/mem and use ioctl(memfd, SECCOMP_IOCTL_NOTIF_ID_VALID, taskid)
		// this avoids race conditions if the target process PID is reused
		// but in the meantime, it is possible to check the validity after the memory
		// read and send the socket file descriptor as notif response

		mempath := fmt.Sprintf("/proc/%d/mem", pid)
		memfd, err := unix.Open(mempath, unix.O_RDWR, 0)
		if err != nil {
			return fmt.Errorf("could not open target process memory, %v", err)
		}

		defer unix.Close(memfd)
		memf := os.NewFile(uintptr(memfd), mempath)

		_, err = unix.Seek(memfd, int64(arg_sockaddr), unix.SEEK_SET)
		if err != nil {
			return fmt.Errorf("could not seek target process memory, %v", err)
		}

		_, err = io.ReadFull(memf, buf)
		if err != nil {
			return fmt.Errorf("could not read target process memory, %v", err)
		}

	} else {
		_, err = unix.ProcessVMReadv(int(pid), []unix.Iovec{local_sockaddr}, []unix.RemoteIovec{remote_sockaddr}, 0)
		if err != nil {
			l.Printf("[%d] bind(%d, 0x%x, %d) detected", pid, arg_sockfd, arg_sockaddr, arg_socklen)
			return fmt.Errorf("could not read remote socket address, %v", err)
		}
	}

	err = seccomp.NotifIDValid(notif_fd, notif.ID)
	if err != nil {
		l.Printf("[%d] bind(%d, 0x%x, %d) detected", pid, arg_sockfd, arg_sockaddr, arg_socklen)
		return fmt.Errorf("remote process notification is invalid, %v", err)
	}

	sockfd, err := pidfd.GetFd(int(arg_sockfd), 0)
	if err != nil {
		l.Printf("[%d] bind(%d, 0x%x, %d) detected", pid, arg_sockfd, arg_sockaddr, arg_socklen)
		return fmt.Errorf("could not pidfd_getfd(pid=%d, %d, 0), %v", pid, arg_sockfd, err)
	}
	defer unix.Close(sockfd)

	var addr netip.Addr
	var port uint16

	var sockaddr = (*unix.RawSockaddr)(unsafe.Pointer(local_sockaddr.Base))

	if sockaddr.Family == unix.AF_INET {
		var sockaddr4 = (*unix.RawSockaddrInet4)(unsafe.Pointer(local_sockaddr.Base))
		addr = netip.AddrFrom4(sockaddr4.Addr)
		port = NetToHostShort(sockaddr4.Port)
		// l.Printf("sockaddr4 = %+v", sockaddr4)
	} else if sockaddr.Family == unix.AF_INET6 {
		var sockaddr6 = (*unix.RawSockaddrInet6)(unsafe.Pointer(local_sockaddr.Base))
		addr = netip.AddrFrom16(sockaddr6.Addr)
		port = NetToHostShort(sockaddr6.Port)
		// l.Printf("sockaddr6 = %+v", sockaddr6)
	} else {
		l.Printf("[%d] bind(%d, {.sa_family = 0x%x}), continuing as family is not INET or INET6", pid, arg_sockfd, sockaddr.Family)
		return nil
	}

	addrport := netip.AddrPortFrom(addr, port)
	// l.Printf("[%d] bind(%d, %s)", pid, arg_sockfd, addrport.String())
	match := fb.Rules.Lookup(addr, port)

	if match == nil {
		if fb.PreventUnknownBind {
			l.Printf("[%d] bind(%d, %s) errno=EACCESS, unknown bind", pid, arg_sockfd, addrport.String())
			resp.Error = int32(unix.EACCES)
			resp.Flags = 0
			return nil
		} else {
			l.Printf("[%d] bind(%d, %s), continuing (permissive)", pid, arg_sockfd, addrport.String())
			return nil
		}
	}

	l.Printf("[%d] bind(%d, %s), detected, redirecting to %s", pid, arg_sockfd, addrport.String(), match.RuleString())

	fd, _, err := match.Fd(ctx, sockfd, port)
	if err == ErrBlock {
		l.Printf("[%d] bind(%d, %s) errno=EACCESS, bind blocked", pid, arg_sockfd, addrport.String())
		resp.Error = int32(unix.EACCES)
		resp.Flags = 0
		return nil
	} else if err != nil {
		return fmt.Errorf("bind(%d, %s), cannot obtain file descriptor for %s, %v", arg_sockfd, addrport.String(), match.String(), err)
	}

	_, err = IoctlSeccompAddFD(uintptr(notif_fd), notif.ID, unix.SECCOMP_ADDFD_FLAG_SETFD, uint32(fd), uint32(arg_sockfd), 0)
	if err != nil {
		return fmt.Errorf("bind(%d, %s), cannot set file descriptor for %s, %v", arg_sockfd, addrport.String(), match.String(), err)
	}

	l.Printf("[%d] bind(%d, %s), redirected to %s", pid, arg_sockfd, addrport.String(), match.RuleString())
	resp.Flags = 0
	resp.Error = 0
	resp.Val = 0
	return nil
}

type KcmpType int

const (
	KCMP_FILE KcmpType = iota
	KCMP_VM
	KCMP_FILES
	KCMP_FS
	KCMP_SIGHAND
	KCMP_IO
	KCMP_SYSVSEM
	KCMP_EPOLL_TFD
	KCMP_TYPES
)

func sameFile(local_fd uintptr, remote_pid uintptr, remote_fd uintptr) (bool, error) {
	res, _, err := unix.Syscall6(unix.SYS_KCMP, uintptr(os.Getpid()), remote_pid, uintptr(KCMP_FILE), local_fd, remote_fd, 0)
	return res == 0, err
}

func IoctlSeccompAddFD(listener_fd uintptr, notif_id uint64, flags uint32, src_fd uint32, new_fd uint32, new_fd_flags uint32) (uintptr, error) {
	req := struct {
		ID         uint64
		Flags      uint32
		SrcFD      uint32
		NewFD      uint32
		NewFDFlags uint32
	}{
		ID:         notif_id,
		Flags:      flags,
		SrcFD:      src_fd,
		NewFD:      new_fd,
		NewFDFlags: new_fd_flags,
	}
	r1, _, errno := unix.Syscall(
		unix.SYS_IOCTL,
		listener_fd,
		unix.SECCOMP_IOCTL_NOTIF_ADDFD,
		uintptr(unsafe.Pointer(&req)),
	)
	if errno == 0 {
		return r1, nil
	} else {
		return r1, errno
	}
}
