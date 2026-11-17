package force_bind

/*
#include <linux/seccomp.h>
#include <linux/filter.h>
#include <linux/audit.h>
#include <linux/signal.h>
#include <sys/ptrace.h>
#include <sys/syscall.h>
#include <sys/ioctl.h>
#include <unistd.h>
#include <errno.h>

typedef struct seccomp_notif seccomp_notif_t;
typedef struct seccomp_notif_resp seccomp_notif_resp_t;
typedef struct seccomp_notif_addfd seccomp_notif_addfd_t;

static int call_ioctl_seccomp_notif_recv(int fd, seccomp_notif_t *notif) {
	return ioctl(fd, SECCOMP_IOCTL_NOTIF_RECV, notif);
}

static int call_ioctl_seccomp_notif_send(int fd, seccomp_notif_resp_t *resp) {
	return ioctl(fd, SECCOMP_IOCTL_NOTIF_SEND, resp);
}

static int call_ioctl_seccomp_notif_id_valid(int fd, __u64 id) {
	return ioctl(fd, SECCOMP_IOCTL_NOTIF_ID_VALID, &id);
}

static int call_ioctl_seccomp_notif_addfd(int fd, seccomp_notif_addfd_t *addfd) {
	return ioctl(fd, SECCOMP_IOCTL_NOTIF_ADDFD, addfd);
}

static int get_errno() {
	return errno;
}

static int get_flag_continue() {
	return SECCOMP_USER_NOTIF_FLAG_CONTINUE;
}
*/
import "C"

import (
	"golang.org/x/sys/unix"
)

type SeccompNotif struct {
	ID    uint64
	Pid   uint32
	Flags uint32
	Data  SeccompNotifData
}
type SeccompNotifData struct {
	Syscall      int32
	Arch         uint
	InstrPointer uint64
	Args         [6]uint64
}
type SeccompNotifResp struct {
	ID    uint64
	Error int32
	Val   uint64
	Flags uint32
}

func IoctlSeccompNotifRecv(listener_fd uintptr) (*SeccompNotif, error) {
	var req C.seccomp_notif_t
	var res = C.call_ioctl_seccomp_notif_recv(C.int(listener_fd), &req)
	if res < 0 {
		return nil, unix.Errno(C.get_errno())
	}
	return &SeccompNotif{
		ID:    uint64(req.id),
		Pid:   uint32(req.pid),
		Flags: uint32(req.flags),
		Data: SeccompNotifData{
			Syscall:      int32(req.data.nr),
			Arch:         uint(req.data.arch),
			InstrPointer: uint64(req.data.instruction_pointer),
			Args: [6]uint64{
				uint64(req.data.args[0]),
				uint64(req.data.args[1]),
				uint64(req.data.args[2]),
				uint64(req.data.args[3]),
				uint64(req.data.args[4]),
				uint64(req.data.args[5]),
			},
		},
	}, nil
}

func IoctlSeccompNotifSend(listener_fd uintptr, resp *SeccompNotifResp) error {
	var res = C.call_ioctl_seccomp_notif_send(C.int(listener_fd), &C.seccomp_notif_resp_t{
		id:    C.__u64(resp.ID),
		val:   C.__s64(resp.Val),
		error: C.__s32(resp.Error),
		flags: C.__u32(resp.Flags),
	})
	if res < 0 {
		return unix.Errno(C.get_errno())
	}
	return nil
}

func IoctlSeccompNotifIdValid(listener_fd uintptr, notif_id uint64) error {
	res := C.call_ioctl_seccomp_notif_id_valid(C.int(listener_fd), C.__u64(notif_id))

	if res < 0 {
		return unix.Errno(C.get_errno())
	}
	return nil
}
func IoctlSeccompNotifAddfd(listener_fd uintptr, notif_id uint64, flags uint32, src_fd uint32, new_fd uint32, new_fd_flags uint32) error {
	res := C.call_ioctl_seccomp_notif_addfd(C.int(listener_fd), &C.seccomp_notif_addfd_t{
		id:          C.__u64(notif_id),
		flags:       C.__u32(flags),
		srcfd:       C.__u32(src_fd),
		newfd:       C.__u32(new_fd),
		newfd_flags: C.__u32(new_fd_flags),
	})

	if res < 0 {
		return unix.Errno(C.get_errno())
	}
	return nil
}

var SeccompNotifRespFlagContinue uint32 = uint32(C.get_flag_continue())
