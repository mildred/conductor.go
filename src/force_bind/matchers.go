package force_bind

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"

	"log"

	"github.com/coreos/go-systemd/v22/activation"
	"golang.org/x/sys/unix"
)

var _ log.Logger

type Matcher struct {
	Prefix netip.Prefix
	Port   int
}

type Rule struct {
	Matcher
	ReplaceBind string
	ReplaceFd   *uintptr
	FdListening bool
}

type Rules struct {
	Rules []*Rule
}

func (r *Rules) String() string {
	if r == nil {
		return ""
	}

	var s []string
	for _, rule := range r.Rules {
		s = append(s, rule.String())
	}
	return strings.Join(s, " ")
}

func (r *Rules) Set(val string) error {
	rule, err := ParseRule(val)
	if err != nil {
		return err
	}

	r.Rules = append(r.Rules, rule...)
	return nil
}

func (r *Rules) Lookup(addr netip.Addr, port uint16) *Rule {
	for _, rule := range r.Rules {
		// log.Printf("lookup %v:%d against %v", addr, port, rule)
		if rule.Port >= 0 && rule.Port != int(port) {
			continue
		}
		if !rule.Prefix.Contains(addr) {
			continue
		}
		// log.Printf("Lookup found %v", rule)
		return rule
	}
	// log.Printf("Lookup failed")
	return nil
}

func (r *Rules) ListeningFds() (fds []uintptr) {
	for _, listener := range activation.Files(false) {
		fds = append(fds, listener.Fd())
	}
	return
}

func (m *Matcher) String() string {
	if m.Port >= 0 {
		if m.Prefix.Addr().Is4() {
			return fmt.Sprintf("%s:%d", m.Prefix.String(), m.Port)
		} else {
			return fmt.Sprintf("[%s]:%d", m.Prefix.String(), m.Port)
		}
	} else {
		if m.Prefix.Addr().Is4() {
			return fmt.Sprintf("%s", m.Prefix.String())
		} else {
			return fmt.Sprintf("[%s]", m.Prefix.String())
		}
	}
}

func (r *Rule) String() string {
	return fmt.Sprintf("%s=%s", r.Matcher.String(), r.RuleString())
}

func (r *Rule) RuleString() string {
	if r.ReplaceBind != "" {
		return r.ReplaceBind
	} else if r.ReplaceFd != nil {
		if r.FdListening {
			return fmt.Sprintf("fd=%d,listening", *&r.ReplaceFd)
		} else {
			return fmt.Sprintf("fd=%d", *&r.ReplaceFd)
		}
	} else {
		return "block"
	}
}

func ParseRule(rulestr string) ([]*Rule, error) {
	parts := strings.SplitN(rulestr, "=", 2)
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty rule %+v", rulestr)
	}

	part_matcher := parts[0]
	part_rule := ""
	if len(parts) >= 2 {
		part_rule = parts[1]
	}

	addr, port, err := splitAddrPort(part_matcher)
	if err != nil {
		return nil, err
	}
	// log.Printf("parse addr=%+v", addr)
	// log.Printf("parse port=%+v", port)

	var prefixes []netip.Prefix
	if addr == "" {
		prefixes = []netip.Prefix{
			netip.PrefixFrom(netip.MustParseAddr("0.0.0.0"), 0),
			netip.PrefixFrom(netip.MustParseAddr("::"), 0),
		}
	} else {
		if strings.Contains(addr, "/") {
			prefix, err := netip.ParsePrefix(addr)
			if err != nil {
				return nil, err
			}
			prefixes = append(prefixes, prefix)
		} else {
			a, err := netip.ParseAddr(addr)
			if err != nil {
				return nil, err
			}
			prefixes = append(prefixes, netip.PrefixFrom(a, a.BitLen()))
		}
	}

	if len(prefixes) == 0 {
		return nil, nil
	}

	// log.Printf("parse part_rule=%+v", part_rule)
	rule, err := ParseRuleOnly(Matcher{prefixes[0], port}, part_rule)
	if err != nil {
		return nil, err
	}

	// log.Printf("parse rule prefixes=%+v", prefixes)
	var rules []*Rule = []*Rule{rule}
	for i, prefix := range prefixes {
		if i != 0 {
			var r Rule = *rule
			r.Matcher.Prefix = prefix
			rules = append(rules, &r)
		}
	}

	return rules, nil
}

func ParseRuleOnly(m Matcher, rule string) (*Rule, error) {
	if rule == "block" {
		return &Rule{m, "", nil, false}, nil
	} else if strings.HasPrefix(rule, "fd=") || strings.HasPrefix(rule, "sd=") {
		parts := strings.SplitN(rule, "=", 2)
		prefix := parts[0]
		parts = strings.SplitN(parts[1], ",", 2)
		listening := len(parts) > 1 && parts[1] == "listening"
		var fd uintptr
		if prefix == "fd" {
			num, err := strconv.ParseInt(parts[0], 0, 0)
			if err != nil {
				return nil, err
			}
			fd = uintptr(num)
		} else if prefix == "sd" {
			num, err := strconv.ParseInt(parts[0], 0, 0)
			if err != nil {
				return nil, err
			}
			fd = uintptr(num) + 3
		}
		return &Rule{m, rule, &fd, listening}, nil
	} else {
		return &Rule{m, rule, nil, false}, nil
	}
}

var ErrBlock = errors.New("bind call blocked")

func (r *Rule) Fd(ctx context.Context, initial_fd int, initial_port uint16) (fd uintptr, listened bool, err error) {
	if r.ReplaceFd == nil && r.ReplaceBind == "" {
		// log.Printf("=block")
		return 0, false, ErrBlock
	}
	if r.ReplaceFd != nil {
		// log.Printf("=fd=%d", *r.ReplaceFd)
		return *r.ReplaceFd, r.FdListening, nil
	}

	if r.ReplaceBind != "" {
		// log.Printf("=%s", r.ReplaceBind)
		fd, err := unix.Dup(initial_fd)
		if err != nil {
			return 0, false, err
		}

		addr, port, err := splitAddrPort(r.ReplaceBind)
		if err != nil {
			return 0, false, err
		}

		// log.Printf("addr=%v, port=%v", addr, port)
		// log.Printf("lookup %v", addr)

		addrs, err := net.DefaultResolver.LookupIPAddr(ctx, addr)
		if err != nil {
			return 0, false, err
		}

		// log.Printf("addrs=%v", addrs)

		if port < 0 {
			port = int(initial_port)
		}

		for _, addr := range addrs {
			ip := addr.IP

			// Can only replace with an address of the same family
			if r.Matcher.Prefix.Addr().BitLen()/8 != len(ip) {
				continue
			}

			var sockaddr unix.Sockaddr
			switch len(ip) {
			case 4:
				sockaddr4 := unix.SockaddrInet4{
					Port: int(port),
					Addr: [4]byte(ip),
				}
				sockaddr = &sockaddr4
			case 16:
				sockaddr6 := unix.SockaddrInet6{
					Port: int(port),
					Addr: [16]byte(ip),
				}
				sockaddr = &sockaddr6
			}

			// log.Printf("bind(%d, %v)", fd, sockaddr)
			err = unix.Bind(fd, sockaddr)
			if err != nil {
				return 0, false, err
			}

			break // bind can only be called once
		}

		var replace_fd = uintptr(fd)
		r.ReplaceFd = &replace_fd
		return replace_fd, false, nil
	}

	return 0, false, ErrBlock
}

func splitAddrPort(addrport string) (addr string, port int, err error) {
	i := strings.LastIndex(addrport, ":")
	var portstr string
	if i >= 0 && !strings.HasSuffix(addrport, "]") {
		addr, portstr = addrport[:i], addrport[i+1:]
	} else {
		addr = addrport
	}

	if portstr == "" {
		return addr, -1, nil
	}

	num, err := strconv.ParseInt(portstr, 10, 0)
	if err != nil {
		return "", 0, err
	}

	return addr, int(num), nil
}
