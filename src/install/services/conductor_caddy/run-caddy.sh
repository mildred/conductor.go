#!/bin/sh

FQDN="$(hostname -f)"

cat <<JSON >./caddy.json
{
  "admin": {
    "listen": "$CONDUCTOR_SERVICE_ADDR_NAME:2019"
  },
  "apps": {
    "tls": {
      "automation": {
        "policies": [
          {
            "issuers": [
              {
                "challenges": {
                  "http": {
                    "disabled": true
                  },
                  "tls-alpn": {
                    "disabled": false
                  }
                },
                "module": "acme"
              }
            ]
          }
        ]
      },
      "certificates": {
        "@id": "tls.certificates",
        "automate": [
          "$FQDN"
        ]
      }
    },
    "http": {
      "servers": {
        "master": {
          "@id": "http.server",
          "listen": [
            "$CONDUCTOR_SERVICE_ADDR_NAME:443"
          ],
          "routes": [
            {
              "match": [
                {
                  "host": [ "$FQDN" ]
                }
              ],
              "handle": [
                {
                  "handler": "static_response",
                  "status_code": "200",
                  "body": "OK"
                }
              ]
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

date
/usr/local/bin/caddy run --watch --config "./caddy.json"
date
res=$?
sleep 1
exit $res

