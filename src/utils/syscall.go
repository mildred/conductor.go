//go:build dragonfly || freebsd || linux || netbsd

package utils

import (
	"syscall"
)

func fcntl(fd int, cmd int, arg int) (val int, err error) {
	r0, _, e1 := syscall.Syscall(syscall.SYS_FCNTL, uintptr(fd), uintptr(cmd), uintptr(arg))
	val = int(r0)
	if e1 != 0 {
		err = e1
	}
	return
}

func SetCloseOnExec(fd int, close bool) (err error) {
	flag, err := fcntl(fd, syscall.F_GETFD, 0)
	if err != nil {
		return err
	}
	if close {
		flag |= syscall.FD_CLOEXEC
	} else {
		flag &= ^syscall.FD_CLOEXEC
	}
	_, err = fcntl(fd, syscall.F_SETFD, flag)
	return err
}
