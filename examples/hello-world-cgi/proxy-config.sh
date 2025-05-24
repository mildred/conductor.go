#!/bin/bash -xe

set -o pipefail

cd "$(dirname "$0")"

if [[ -n "$CONDUCTOR_FUNCTION_SOCKET" ]]; then

  (
    jq -n '{
      id: "'"conductor.$CONDUCTOR_APP.$CONDUCTOR_INSTANCE/handle/0/upstreams"'",
      config: {
        "@id": "'"conductor-function.$CONDUCTOR_FUNCTION_ID"'",
        dial: ("unix/" + $ENV.CONDUCTOR_FUNCTION_SOCKET)
      }
    }'
  ) | jq -s .

else

  (
    conductor f caddy-config
    jq -n '{
      "id": "conductor-server/routes",
      "config": {
        "@id": "'"conductor.$CONDUCTOR_APP.$CONDUCTOR_INSTANCE"'",
        "match": [{
          "path": "/hello"
        }],
        "handle": [{
          "handler": "reverse_proxy",
          "upstreams": [],
          "transport": { "protocol": "http" }
        }]
      }
    }'
  ) | jq -s .

fi
