package addr

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/mildred/conductor.go/src/config"
)

type lockType int16

func (lt lockType) String() string {
	switch lt {
	case readLock:
		return "RLock"
	case writeLock:
		return "Lock"
	default:
		return "Unlock"
	}
}

const (
	readLock  lockType = syscall.LOCK_SH
	writeLock lockType = syscall.LOCK_EX
)

func lock(f *os.File, lt lockType) (err error) {
	for {
		err = syscall.Flock(int(f.Fd()), int(lt))
		if err != syscall.EINTR {
			break
		}
	}
	if err != nil {
		return &fs.PathError{
			Op:   lt.String(),
			Path: f.Name(),
			Err:  err,
		}
	}
	return nil
}

func unlock(f *os.File) error {
	return lock(f, syscall.LOCK_UN)
}

type Opts struct {
	ULAPrefix string
	Namespace string
}

func NewAddr(cfg *config.Config) *Addr {
	return &Addr{
		Opts: Opts{
			ULAPrefix: cfg.ULAPrefix,
		},
	}
}

var defaultAddr *Addr

func newDefaultAddr() (*Addr, error) {
	if defaultAddr == nil {
		cfg, err := config.ReadConfig()
		if err != nil {
			return nil, err
		}

		defaultAddr = NewAddr(cfg)
	}
	return defaultAddr, nil
}

func OpenDefaultAddr() (*Addr, error) {
	_, err := newDefaultAddr()
	if err != nil {
		return nil, err
	}

	err = defaultAddr.Init()
	if err != nil {
		return nil, err
	}
	return defaultAddr, nil
}

func DefaultAddrReadOnly() (*Addr, error) {
	return newDefaultAddr()
}

//  #!/bin/bash
//
//  ULA_PREFIX='${local.ula_prefix}'
//  UNIT_NAME='${var.unit_name}'
//
//  LF=$'\n'
//
//  set -e

type Addr struct {
	Opts
	hosts *os.File
}

func (o *Opts) UnitName() string {
	if o.Namespace == "" {
		return "addr"
	} else {
		return o.Namespace
	}
}

func (o *Opts) UnitTitle() string {
	name := strings.ReplaceAll(strings.ReplaceAll(o.UnitName(), "_", " "), "-", " ")
	return strings.ReplaceAll(strings.Title(name), " ", "")
}

func (o *Opts) InterfaceName() string {
	return "addr0"
}

func (o *Opts) ULAPrefixNoCidr() string {
	parts := strings.SplitN(o.ULAPrefix, "/", 2)
	parts = strings.SplitN(parts[0], "::", 2)
	return parts[0] + "::"
}

func (a *Addr) Init() error {
	//  mkdir -p /run/$UNIT_NAME

	//  touch /run/$UNIT_NAME/hosts
	//  exec 4</run/$UNIT_NAME/hosts
	//  flock --timeout 5 4 || exit 1

	err := os.MkdirAll(fmt.Sprintf("/run/%s", a.UnitName()), 0755)
	if err != nil {
		return fmt.Errorf("mkdir failed, %v", err)
	}

	hosts_file := fmt.Sprintf("/run/%s/hosts", a.UnitName())
	f, err := os.OpenFile(hosts_file, os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("create file failed, %v", err)
	}
	f.Close()

	currentTime := time.Now().Local()
	err = os.Chtimes(fmt.Sprintf("/run/%s/hosts", a.UnitName()), currentTime, currentTime)
	if err != nil {
		return fmt.Errorf("chtimes failed, %v", err)
	}

	a.hosts, err = os.Open(fmt.Sprintf("/run/%s/hosts", a.UnitName()))
	if err != nil {
		return fmt.Errorf("open failed, %v", err)
	}

	err = lock(a.hosts, writeLock)
	if err != nil {
		err = fmt.Errorf("lock failed, %v", err)
		e := a.hosts.Close()
		if e != nil {
			err = errors.Join(err, e)
		}
		return err
	}

	return nil
}

func (a *Addr) Close() error {
	err1 := unlock(a.hosts)
	err2 := a.hosts.Close()

	return errors.Join(err1, err2)
}

func (a *Addr) Update() error {
	//  if [ -w /etc/hosts ]; then
	//    sed -i "/# BEGIN \/run\/$UNIT_NAME\/hosts/,/# END \/run\/$UNIT_NAME\/hosts/ d" /etc/hosts
	//    cat /etc/hosts >/etc/.tmp$$-hosts
	//    echo "# BEGIN /run/$UNIT_NAME/hosts" >>/etc/.tmp$$-hosts
	//    cat /run/$UNIT_NAME/hosts >>/etc/.tmp$$-hosts
	//    echo "# END /run/$UNIT_NAME/hosts" >>/etc/.tmp$$-hosts
	//    mv /etc/.tmp$$-hosts /etc/hosts
	//  else
	//    echo >&2 "/etc/hosts: read-only"
	//  fi

	st, err := os.Stat("/etc/hosts")
	if err != nil {
		return err
	}

	hosts, err := os.ReadFile("/etc/hosts")
	if err != nil {
		return err
	}

	lines := strings.Split(string(hosts), "\n")
	small_hosts_file := fmt.Sprintf("/run/%s/hosts", a.UnitName())

	begin := fmt.Sprintf("# BEGIN %s", small_hosts_file)
	end := fmt.Sprintf("# END %s", small_hosts_file)

	var result []string
	var skip bool
	var inserted bool
	for _, line := range lines {
		if line == begin {
			h, err := os.ReadFile(small_hosts_file)
			if err != nil {
				return err
			}
			result = append(result, line, strings.TrimRight(string(h), "\n"))
			inserted = true
			skip = true
		}
		if line == end {
			skip = false
		}
		if !skip {
			result = append(result, line)
		}
	}
	if !inserted {
		h, err := os.ReadFile(small_hosts_file)
		if err != nil {
			return err
		}
		result = append(result, begin, strings.TrimRight(string(h), "\n"), end)
		inserted = true
	}

	new_hosts := joinLines(result)
	if new_hosts != string(hosts) {
		fmt.Fprintf(os.Stderr, ": update %q\n", "/etc/hosts")
		err := os.WriteFile("/etc/hosts", []byte(new_hosts), st.Mode())
		if err != nil {
			return err
		}
	}

	return nil
}

func (a *Addr) EnvVars(new_host string) (res []string, err error) {
	env_filename := fmt.Sprintf("/run/%s/%s.env", a.UnitName(), new_host)

	env, err := readEnvFile(env_filename, true)
	if err != nil {
		return nil, err
	}
	for k, v := range env {
		res = append(res, k+"="+v)
	}
	return
}

var ErrNoAddr = errors.New("no address")

func (a *Addr) GetAddr(new_host string) (ip4, ip6, varname string, err error) {
	hosts_filename := fmt.Sprintf("/run/%s/hosts", a.UnitName())
	hosts, err := readHostsFile(hosts_filename, true)
	if err != nil {
		return "", "", envname(new_host), err
	}

	for _, h := range hosts {
		if slices.Contains(h.Hostnames, new_host+"4") {
			ip4 = h.Address
		}
		if slices.Contains(h.Hostnames, new_host+"6") {
			ip6 = h.Address
		}
	}

	if ip4 == "" || ip6 == "" {
		err = ErrNoAddr
	}

	return ip4, ip6, envname(new_host), err
}

func (a *Addr) Up(ctx context.Context, new_host string) error {
	//  local NEW_HOST="$1"
	//  local i prefix ip4 ip6

	hosts_filename := fmt.Sprintf("/run/%s/hosts", a.UnitName())
	env_filename := fmt.Sprintf("/run/%s/%s.env", a.UnitName(), new_host)

	//  # Check if the address is already allocated, then print and exit
	//  if [[ -e /run/$UNIT_NAME/$NEW_HOST.env ]]; then
	//    echo "/run/$UNIT_NAME/$NEW_HOST.env: already exists" >&2
	//    echo
	//    egrep "^\\S* $NEW_HOST ${NEW_HOST}[46]$" /run/$UNIT_NAME/hosts
	//    echo
	//    cat "/run/$UNIT_NAME/$NEW_HOST.env"
	//    return 0
	//  fi

	hosts, err := readHostsFile(hosts_filename, true)
	if err != nil {
		return err
	}

	data, err := os.ReadFile(env_filename)
	if err == nil {
		fmt.Fprintf(os.Stderr, "%s: already exists\n\n", env_filename)

		for _, h := range hosts {
			if h.IsHost(new_host, 0) {
				fmt.Fprintf(os.Stdout, "%s\n", *h.Line)
			}
		}
		fmt.Fprintf(os.Stdout, "\n%s\n", string(data))
		return nil
	}

	//  # Generate and address by iterating until a free address is found
	//  prefix=$(echo $ULA_PREFIX | sed 's:/.*::')
	//  # If the address is already in hosts file, take it
	//  i="$(sed -rn "/^127\\.0\\.0\\.(\\S*) ${NEW_HOST} ${NEW_HOST}4$/ { s//\\1/p; q }" /run/$UNIT_NAME/hosts)"
	//  if [ -z "$i" ]; then # No address found, generate it
	//    update
	//    i=2
	//    while egrep -q "^127.0.0.$i " /etc/hosts /run/$UNIT_NAME/hosts; do
	//      i=$((i+1))
	//    done
	//  fi

	prefix := a.ULAPrefixNoCidr()
	var ip4, ip6 string

	i := -1
	for _, h := range hosts {
		if h.IsHost(new_host, 4) {
			parts := strings.Split(h.Address, ".")
			if len(parts) == 4 {
				num, err := strconv.ParseInt(parts[3], 10, 0)
				if err != nil {
					err = nil
					i = -1
				} else {
					i = int(num)
					ip4 = h.Address
				}
			}
		}
	}

	if i == -1 {
		err := a.Update()
		if err != nil {
			return err
		}
		i = 2
		system_hosts, err := readHostsFile("/etc/hosts", true)
		if err != nil {
			return err
		}
		for {
			found := false
			ip4 = fmt.Sprintf("127.0.0.%d", i)
			for _, h := range hosts {
				if h.Address == ip4 {
					found = true
					break
				}
			}
			for _, h := range system_hosts {
				if h.Address == ip4 {
					found = true
					break
				}
			}
			if found {
				i += 1
			} else {
				break
			}
		}
	}

	//  # We have allocated the IP addresses
	//  ip4="127.0.0.$i"
	//  ip6="$prefix$(printf %""x $i)"
	//  /usr/bin/sed -ri -e "/^\\S* ${NEW_HOST} ${NEW_HOST}[46]$/d" /run/$UNIT_NAME/hosts
	//  echo "$ip4 $NEW_HOST $NEW_HOST""4" >>/run/$UNIT_NAME/hosts
	//  echo "$ip6 $NEW_HOST $NEW_HOST""6" >>/run/$UNIT_NAME/hosts
	//  mkdir -p /run/$UNIT_NAME
	//  echo "HOST_$(echo $NEW_HOST | sed 's:-:_:g')6=$ip6" >/run/${UNIT_NAME}/$NEW_HOST.env
	//  echo "HOST_$(echo $NEW_HOST | sed 's:-:_:g')4=$ip4" >>/run/${UNIT_NAME}/$NEW_HOST.env
	//  echo "HOSTADDR6=$ip6" >>/run/$UNIT_NAME/$NEW_HOST.env
	//  echo "HOSTADDR4=$ip4" >>/run/$UNIT_NAME/$NEW_HOST.env
	//  ip link add addr0 type dummy 2>/dev/null || true
	//  ip link set dev addr0 up
	//  ip addr del "$ip6" dev lo 2>/dev/null || true
	//  ip addr add "$ip6" dev addr0
	//  update

	ip6 = fmt.Sprintf("%s%x", prefix, i)

	var new_hosts []Host
	for _, h := range hosts {
		if !h.IsHost(new_host, 0) {
			new_hosts = append(new_hosts, h)
		}
	}
	new_hosts = append(new_hosts, Host{Address: ip4, Hostnames: []string{new_host, new_host + "4"}})
	new_hosts = append(new_hosts, Host{Address: ip6, Hostnames: []string{new_host, new_host + "6"}})
	fmt.Fprintf(os.Stderr, ": write %q\n", hosts_filename)
	err = os.WriteFile(hosts_filename, []byte(assembleHostsFile(new_hosts)), 0644)
	if err != nil {
		return err
	}

	err = os.MkdirAll(fmt.Sprintf("/run/%s", a.UnitName()), 0755)
	if err != nil {
		return err
	}

	envs := joinLines([]string{
		fmt.Sprintf("%s4=%s", envname(new_host), ip4),
		fmt.Sprintf("%s6=%s", envname(new_host), ip6),
		fmt.Sprintf("HOSTADDR4=%s", ip4),
		fmt.Sprintf("HOSTADDR6=%s", ip6),
	})
	fmt.Fprintf(os.Stderr, ": write %q\n", env_filename)
	err = os.WriteFile(env_filename, []byte(envs), 0644)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "+ ip link add %s type dummy 2>/dev/null || true\n", a.InterfaceName())
	cmd := exec.CommandContext(ctx, "ip", "link", "add", a.InterfaceName(), "type", "dummy")
	cmd.Stdout = os.Stdout
	cmd.Stderr = nil
	_ = cmd.Run()

	fmt.Fprintf(os.Stderr, "+ ip link set dev %s up\n", a.InterfaceName())
	cmd = exec.CommandContext(ctx, "ip", "link", "set", "dev", a.InterfaceName(), "up")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return err
	}

	if ip6 != "" {
		fmt.Fprintf(os.Stderr, "+ ip addr del %s dev lo 2>/dev/null || true\n", ip6)
		cmd = exec.CommandContext(ctx, "ip", "addr", "del", ip6, "dev", "lo")
		cmd.Stdout = os.Stdout
		cmd.Stderr = nil
		_ = cmd.Run()

		fmt.Fprintf(os.Stderr, "+ ip addr add %s dev %s\n", ip6, a.InterfaceName())
		cmd = exec.CommandContext(ctx, "ip", "addr", "add", ip6, "dev", a.InterfaceName())
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		if err != nil {
			return err
		}
	}

	err = a.Update()
	if err != nil {
		return err
	}

	//  echo
	//  egrep "^\\S* $NEW_HOST ${NEW_HOST}[46]$" /run/$UNIT_NAME/hosts
	//  echo
	//  cat "/run/$UNIT_NAME/$NEW_HOST.env"

	fmt.Fprintf(os.Stdout, "%s %s %s4\n%s %s %s6\n\n%s\n", ip4, new_host, new_host, ip6, new_host, new_host, envs)
	return nil
}

func (a *Addr) Gen(ctx context.Context, new_host string, genfile string) error {
	//  local NEW_HOST="$1"
	//  local generated=false
	//  if [ -e "/run/$UNIT_NAME/$NEW_HOST.generated" ]; then
	//    while read f; do
	//      if [ "$f" = "$2" ]; then
	//        generated=true
	//        break
	//      fi
	//    done <"/run/$UNIT_NAME/$NEW_HOST.generated"
	//  fi
	//  if ! $generated; then
	//    echo "$2" >> "/run/$UNIT_NAME/$NEW_HOST.generated"
	//  fi
	//  up "$1"

	generated := false
	generated_filename := fmt.Sprintf("/run/%s/%s.generated", a.UnitName(), new_host)

	var lines []string
	gen, err := os.ReadFile(generated_filename)
	if err != nil && !os.IsNotExist(err) {
		return err
	} else if err == nil {
		lines = strings.Split(string(gen), "\n")
		for _, line := range lines {
			if line == genfile {
				generated = true
				break
			}
		}
	}
	if !generated {
		lines = append(lines, genfile)
		err := os.WriteFile(generated_filename, []byte(joinLines(lines)), 0644)
		if err != nil {
			return err
		}
	}

	return a.Up(ctx, new_host)
}

func (a *Addr) Down(ctx context.Context, new_host string) error {
	//  local NEW_HOST="$1"
	//  local i prefix ip4 ip6

	//  if [ -e /run/$UNIT_NAME/$NEW_HOST.generated ]; then
	//    local generated=()
	//    while read f; do
	//      if [ -e "$f" ] && grep "$UNIT_NAME[46]\?@$NEW_HOST" "$f" >/dev/null; then
	//        generated+=("$f")
	//      fi
	//    done <"/run/$UNIT_NAME/$NEW_HOST.generated"
	//    if [ -n "${generated}" ]; then
	//      echo >&2 "Cannot stop $UNIT_NAME@$NEW_HOST because generated units depends on it: ${generated[*]}"
	//      return 1
	//    else
	//      rm -f "/run/$UNIT_NAME/$NEW_HOST.generated"
	//    fi
	//  fi

	generated_filename := fmt.Sprintf("/run/%s/%s.generated", a.UnitName(), new_host)
	lookup := []string{
		fmt.Sprintf("%s@%s", a.UnitName(), new_host),
		fmt.Sprintf("%s4@%s", a.UnitName(), new_host),
		fmt.Sprintf("%s6@%s", a.UnitName(), new_host),
	}

	var lines []string
	gen, err := os.ReadFile(generated_filename)
	if err != nil && !os.IsNotExist(err) {
		return err
	} else if err == nil {
		generated := []string{}
		lines = strings.Split(string(gen), "\n")
		for _, f := range lines {
			contains, err := fileContainsLine(f, lookup...)
			if err != nil && !os.IsNotExist(err) {
				return err
			}
			if contains {
				generated = append(generated, f)
			}
		}
		if len(generated) > 0 {
			return fmt.Errorf("Cannot stop %s@%s because generated units depends on it: %v", a.UnitName(), new_host, strings.Join(generated, ", "))
		} else {
			err := os.RemoveAll(generated_filename)
			if err != nil {
				return err
			}
		}
	}

	//  ip6=$(sed -rn -e "s/.*6=(.*)/\\1/p" -e '1 q' /run/$UNIT_NAME/$NEW_HOST.env)
	//  /usr/bin/sed -ri -e "/^\\S* ${NEW_HOST} ${NEW_HOST}[46]$/d" /run/$UNIT_NAME/hosts
	//  /bin/rm -f "/run/$UNIT_NAME/${NEW_HOST}.env"
	//  ip addr del "$ip6" dev lo 2>/dev/null || true
	//  ip addr del "$ip6" dev addr0
	//  update

	hosts_filename := fmt.Sprintf("/run/%s/hosts", a.UnitName())
	env_filename := fmt.Sprintf("/run/%s/%s.env", a.UnitName(), new_host)

	var ip6 string
	env, err := readEnvFile(env_filename, true)
	if err != nil {
		return err
	}
	for k, v := range env {
		if strings.HasSuffix(k, "6") {
			ip6 = v
			// } else if strings.HasSuffix(k, "4") {
			//   ip4 = v
		}
	}

	hosts, err := readHostsFile(hosts_filename, true)
	if err != nil {
		return err
	}
	var new_hosts []Host
	for _, h := range hosts {
		if !h.IsHost(new_host, 0) {
			new_hosts = append(new_hosts, h)
		}
	}
	fmt.Fprintf(os.Stderr, ": write %q\n", hosts_filename)
	err = os.WriteFile(hosts_filename, []byte(assembleHostsFile(new_hosts)), 0644)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "+ rm -f %q\n", env_filename)
	err = os.RemoveAll(env_filename)
	if err != nil {
		return err
	}

	if ip6 != "" {
		fmt.Fprintf(os.Stderr, "+ ip addr del %s dev lo\n", ip6)
		cmd := exec.CommandContext(ctx, "ip", "addr", "del", ip6, "dev", "lo")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		_ = cmd.Run()

		fmt.Fprintf(os.Stderr, "+ ip addr del %s dev %s\n", ip6, a.InterfaceName())
		cmd = exec.CommandContext(ctx, "ip", "addr", "del", ip6, "dev", a.InterfaceName())
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		if err != nil {
			return err
		}
	}

	return a.Update()
}

func fileContainsLine(fname string, line ...string) (bool, error) {
	data, err := os.ReadFile(fname)
	if err != nil {
		return false, err
	}

	lines := strings.Split(string(data), "\n")
	for _, l := range lines {
		if slices.Contains(lines, l) {
			return true, nil
		}
	}
	return false, nil
}

func readEnvFile(fname string, ignoreMissing bool) (map[string]string, error) {
	data, err := os.ReadFile(fname)
	if err != nil && os.IsNotExist(err) && ignoreMissing {
		return map[string]string{}, nil
	} else if err != nil {
		return nil, err
	}

	res := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 1 && parts[0] != "" {
			delete(res, parts[0])
		} else if len(parts) == 2 && parts[0] != "" {
			res[parts[0]] = parts[1]
		}
	}
	return res, nil
}

var Spaces = regexp.MustCompile("\\s+")

type Host struct {
	Line      *string
	Address   string
	Hostnames []string
}

func (h *Host) IsHost(hostname string, ipver int) bool {
	hostnames := strings.Join(h.Hostnames, " ")
	match_v4 := ipver != 6
	match_v6 := ipver != 4
	return (match_v4 && hostnames == fmt.Sprintf("%s %s4", hostname, hostname)) ||
		(match_v6 && hostnames == fmt.Sprintf("%s %s6", hostname, hostname))
}

func readHostsFile(fname string, ignoreMissing bool) ([]Host, error) {
	data, err := os.ReadFile(fname)
	if err != nil && os.IsNotExist(err) && ignoreMissing {
		return []Host{}, nil
	} else if err != nil {
		return nil, err
	}

	res := []Host{}
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" {
			continue
		}

		parts := Spaces.Split(line, -1)
		var h = Host{
			Line: &line,
		}
		for _, p := range parts {
			if p != "" {
				if h.Address == "" {
					h.Address = p
				} else {
					h.Hostnames = append(h.Hostnames, p)
				}
			}
		}
		res = append(res, h)
	}
	return res, nil
}

func assembleHostsFile(hosts []Host) string {
	var lines []string
	for _, h := range hosts {
		if h.Line == nil {
			lines = append(lines, fmt.Sprintf("%s %s", h.Address, strings.Join(h.Hostnames, " ")))
		} else {
			lines = append(lines, *h.Line)
		}
	}
	return joinLines(lines)
}

func joinLines(lines []string) string {
	res := strings.Join(lines, "\n")
	if res != "" && !strings.HasSuffix(res, "\n") {
		res = res + "\n"
	}
	return res
}

func envname(host string) string {
	return strings.ReplaceAll(host, "-", "_")
}

//  case "$1" in
//    up|down|gen|update)
//      "$@"
//      ;;
//    *)
//      echo "$0 gen HOSTNAME GENERATED_FILE"
//      echo "$0 up|down HOSTNAME"
//      echo "$0 update"
//      exit 1
//      ;;
//  esac
