#!/bin/bash

current_version="$(caddy --version 2>/dev/null | cut -f1 -d' ')"
latest_version=latest

if [[ -n "$current_version" ]]; then
  echo "Checking the latest Caddy version against current version $current_version..."
  latest_version="$(curl -s https://api.github.com/repos/caddyserver/caddy/releases/latest | jq -r .tag_name)"
  echo "Latest Caddy version is: $latest_version"
fi

if [[ -n "$current_version" ]] && [[ $current_version = $latest_version ]]; then
  echo "Caddy is up to date"
  exit 0
fi

curl_caddy() {
  curl --no-progress-meter "https://caddyserver.com/api/download?$1" -o /usr/local/bin/.caddy.tmp
}

if [[ -z "$latest_version" ]]; then
  echo >&2 "Cannot detect latest version of Caddy"
  echo >&2 "try: curl -s https://api.github.com/repos/caddyserver/caddy/releases/latest | jq -r .tag_name"
  exit 1
fi

echo "Updating Caddy to $latest_version..."
set -e

case "$(uname -m)" in
  x86_64)
    curl_caddy "os=linux&arch=amd64&p=github.com%2Fmholt%2Fcaddy-l4&idempotency=76417366483309"
    ;;
  aarch64)
    curl_caddy "os=linux&arch=arm64&p=github.com%2Fmholt%2Fcaddy-l4&idempotency=79985460930629"
    ;;
  *)
    echo >&2 "Unknown architecture $(uname -m) to download caddy"
    exit 1
    ;;
esac

chmod +x /usr/local/bin/.caddy.tmp
mv /usr/local/bin/.caddy.tmp /usr/local/bin/caddy

echo "Caddy now installed at version $(caddy --version)"
exit 0
