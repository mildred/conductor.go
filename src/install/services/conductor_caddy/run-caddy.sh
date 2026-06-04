#!/bin/sh

exec /usr/local/bin/caddy run --watch --config "$CONDUCTOR_SERVICE_DIR/caddy.json"

