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
export XDG_RUNTIME_DIR="$(mktemp "$XDG_RUNTIME_DIR/c.XXXXXXX")"
export PATH="$PWD/bin:$PATH"
export PS1="[conductor instance :$port] $PS1"

trap "rm -rf $XDG_RUNTIME_DIR" 0

mkdir -p $XDG_CONFIG_HOME/environment.d
cat <<CONF >$XDG_CONFIG_HOME/environment.d/path.conf
PATH=$PWD/bin:\$PATH
CONF

mkdir -p bin
ln -sf "$srcdir/conductor bin/conductor"

mkdir -p etc/conductor/dirs
ln -sf "$srcdir"/examples/* etc/conductor/dirs

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

#echo
#echo "Start Caddy server on port $port using: ./caddy.sh &"
#echo

if which tmux >/dev/null 2>&1; then
  tmux new-session -n "init" sh -c '
    tmux new-window -n caddy-'"$port"' ./caddy.sh;
    tmux new-window -n systemd-user /lib/systemd/systemd --user;
    tmux new-window -n journalctl -e XDG_RUNTIME_DIR=$OLD_XDG_RUNTIME_DIR journalctl --user -f;
    tmux new-window -n '"$name"' "$SHELL";
    conductor system install
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


