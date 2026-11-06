//go:build ignore

#include <linux/ptrace.h>
#include <linux/bpf.h>
#include <bpf/bpf_tracing.h>
#include <bpf/bpf_helpers.h>

char __license[] SEC("license") = "Dual MIT/GPL";

struct event {
    __u64 ts;
    __u32 pid;
    int fd;
    char type;
};

// Config map for userspace to pass target FD
struct {
    __uint(type, BPF_MAP_TYPE_ARRAY);
    __type(key, __u32);
    __type(value, int);
    __uint(max_entries, 1);
} target_fd_map SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 1 << 24);
    __type(value, struct event);
} events SEC(".maps");

static void emit_event(__u32 pid, int fd, char type) {
    struct event *e;
    e = bpf_ringbuf_reserve(&events, sizeof(*e), 0);
    if (!e) return;

    e->ts = bpf_ktime_get_ns();
    e->pid = pid;
    e->fd = fd;
    e->type = type;
    bpf_ringbuf_submit(e, 0);
}

SEC("kretprobe/sys_accept4")
int trace_accept4(struct pt_regs *ctx) {
    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u32 pid = pid_tgid >> 32;

    int listen_fd = PT_REGS_PARM1(ctx);
    int new_fd = PT_REGS_RC(ctx);

    // Read target FD from config map
    int lookup_key = 0;
    int *target_fd;
    target_fd = bpf_map_lookup_elem(&target_fd_map, &lookup_key);

    if (!target_fd || listen_fd != *target_fd || new_fd < 0) return 0;

    emit_event(pid, new_fd, 'A');
    return 0;
}

SEC("kretprobe/sys_accept")
int trace_accept(struct pt_regs *ctx) {
    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u32 pid = pid_tgid >> 32;

    int listen_fd = PT_REGS_PARM1(ctx);
    int new_fd = PT_REGS_RC(ctx);

    // Read target FD from config map
    int lookup_key = 0;
    int *target_fd;
    target_fd = bpf_map_lookup_elem(&target_fd_map, &lookup_key);

    if (!target_fd || listen_fd != *target_fd || new_fd < 0) return 0;

    emit_event(pid, new_fd, 'A');
    return 0;
}

SEC("kprobe/sys_close")
int trace_close(struct pt_regs *ctx) {
    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u32 pid = pid_tgid >> 32;

    int fd = PT_REGS_PARM1(ctx);

    emit_event(pid, fd, 'C');
    return 0;
}

