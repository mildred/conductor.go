#!/bin/bash

set -e

scriptdir="$(dirname "$0")"

case "$(uname -s)-$(uname -m)" in
  Linux-x86_64)
    os=linux
    arch=amd64
    ;;
  Linux-aarch64)
    os=linux
    arch=arm64
    ;;
  Darwin-x86_64)
    os=darwin
    arch=amd64
    ;;
  *)
    echo "Unknown architecture $(uname -s)-$(uname -m)"
    exit 1
    ;;
esac

URL="https://caddyserver.com/api/download?os=linux&arch=amd64"

if ! [[ -e "$scriptdir/caddy" ]]; then
  curl -L "$URL" -o "$scriptdir/caddy"
  chmod +x "$scriptdir/caddy"
fi

exec "$scriptdir/caddy" "$@"

