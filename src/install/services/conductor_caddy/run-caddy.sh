#!/bin/sh

cat <<JSON >./caddy.json
{
  "admin": {
    "listen": "$CONDUCTOR_ADDR_NAME:2019"
  },
  "apps": {
    "http": {
      "servers": {
        "master": {
          "listen": [
            "$CONDUCTOR_ADDR_NAME:443"
          ],
          "routes": [
            {
              "handle": {
                "handler": "static_response",
                "status_code": "200",
                "body": "OK"
              }
            }
          ]
        }
      }
    }
  }
}
JSON

set -x

env
jq . ./caddy.json

/usr/local/bin/caddy run --watch --config "./caddy.json"
res=$?
sleep 0.1
exit $res

