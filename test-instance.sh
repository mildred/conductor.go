#!/bin/bash

srcdir="$(cd "$(dirname "$0")"; pwd)"

port=8080

while true; do
  case "$1" in
    --port|-p)
      port="$2"
      shift
      ;;
    *)
      break
      ;;
  esac
  shift
done

name="${1:-instance-$port}"

if ! [[ -x conductor ]]; then
  go build ./cmd/conductor
fi

mkdir -p "$name"
cd "$name"

export OLD_XDG_RUNTIME_DIR=$XDG_RUNTIME_DIR
export XDG_CONFIG_HOME="$PWD/etc"
export XDG_DATA_HOME="$PWD/share"
export XDG_STATE_HOME="$PWD/lib"
export XDG_RUNTIME_DIR="$(mktemp -d "$XDG_RUNTIME_DIR/c.XXXXXXX")"
export PATH="$PWD/bin:$PATH"
export PS1="[conductor instance :$port] $PS1"

trap "rm -rf $XDG_RUNTIME_DIR" 0

mkdir -p $XDG_CONFIG_HOME/environment.d
cat <<CONF >$XDG_CONFIG_HOME/environment.d/path.conf
PATH=$PWD/bin:\$PATH
CONF

mkdir -p bin
ln -sf "$srcdir/conductor bin/conductor"

mkdir -p etc/conductor/services
ln -sf "$srcdir"/examples/* etc/conductor/services

cat <<SHELL >caddy.sh
#!/bin/bash

exec "$srcdir/caddy.sh" run -c "$PWD/caddy-config.json"

SHELL
chmod +x caddy.sh

cat <<JSON >caddy-config.json
{
  "apps": {
    "http": {
      "servers": {
        "srv0": {
          "@id": "conductor-server",
          "automatic_https": { "disable": true },
          "listen": [ ":$port" ],
          "routes": [
          ]
        }
      }
    }
  }
}
JSON

# Fake caddy service
mkdir -p "$XDG_CONFIG_HOME/systemd/user"
cat <<SERVICE >"$XDG_CONFIG_HOME/systemd/user/caddy.service"
[Service]
ExecStart=/bin/true
RemainAfterExit=yes
SERVICE

#echo
#echo "Start Caddy server on port $port using: ./caddy.sh &"
#echo

if which tmux >/dev/null 2>&1; then
  tmux new-session -n "init" sh -xc '
    tmux new-window -d -n caddy-'"$port"' sh -xc "./caddy.sh; cat";
    tmux new-window -d -n systemd-user sh -xc "/lib/systemd/systemd --user --log-level=debug --log-target=console";
    tmux new-window -d -n journalctl -e XDG_RUNTIME_DIR=$OLD_XDG_RUNTIME_DIR journalctl --user -f;
    tmux new-window -d -n '"$name"' "$SHELL";
    conductor system install
    env | grep -E "^XDG_(CONFIG_HOME|DATA_HOME|STATE_HOME|RUNTIME_DIR)"
    exec $SHELL
  '
else
  ./caddy.sh &
  caddy=$!

  "$SHELL"
  res=$?

  kill $caddy &
  wait $caddy

  exit $res
fi


