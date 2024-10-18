#!/bin/bash

zero="$0"
real_zero="$(realpath "$zero")"

help(){
  cat <<EOF
conductor service declare DIR [VAR=VAL...]
------------------------------------------

Declare a new service using the data in the provided DIR.
The files in DIR are templated using the provided variables
This configuration declares:

- a service name (unique)
- possibly other services (a list of DIR + key/value pairs that will need templating)
- the service concurrency (number of instances it should be running in parallel)
- a pod template
- reverse proxy configuration
- systemd socket activation data

If the service with the given name already exists, then the service is updated. Upon update, the service configuration hash or generation id changes, and earlier deployments for this service are stopped and new deployments are started.

The service configuration is stored in a directory named after the service

Implementation:
- initial service declaration creates a service with `systemd-run --no-block --unit=NAME.service -pExecStop="conductor service _stop DIR" -p... conductor service start DIR`
- service update reloads the service with `systemctl reload NAME.service`



conductor service _start|_stop|_reload|_cleanup SERVICEDIR
----------------------------------------------------------

ExecStart=|ExecStop=|ExecReload=|ExecStopPost= for the service. It ensures that:

- the deployments for the services are started (unless the service is stopped) according to its concurrency
- the reverse proxy is configured (a unit is created with systemd-run that will continue to be active even if the service itself is stopped)
- old deployments with stale configuration is stopped

During a reload, the list of changes is generated to see if the service it self changed (perform a rolling restart) or if the reverse proxy config changed (reload the reverse proxy config), or both


conductor deployment _start|_stop|_cleanup DEPLOYMENTDIR
--------------------------------------------------------

ExecStart=|ExecStop=|ExecStopPost= for a service deployment

A service deployment links to a SERVICEDIR but it also contains:
- the configuration hash or generation id of the service
- anything from SERVICEDIR that is needed. It must be possible to have all the needed configuration even if SERVICEDIR has been updated with a new configuration
- the pod file after templating
- runtime data such as the pod ip address

Procedure for startup:
- start the processes
- write down runtime configuration
- configure reverse proxy
- check customizable health checks

Procedure for shutdown:
- custom hook to notify processes that shutting down is in progress
- remove from load balancer
- custom hook to wait for processes to stop doing anything important
- stopping of the processes


conductor cgi declare MANIFEST
------------------------------

Declares a CGI function.
Digest of the manifest is computed and is used to access the function.
The function can declare aliases

What this does is:
- it creates a unit via systemd-run to run the function from a socket
- it declares a unix domain socket for this function
- it register with the load balancer endpoints to hook on the socket

The manifest contains:
- a name, purely for description
- a list of aliases it wants
- the interface it talks to withthe socket (CGI, ...)
- the execution engine (pure exec, podman pod, ...)

This could be an alias of "conductor deployment declare"

- CGI function = socket


conductor cgi identify MANIFEST
-------------------------------

Shows the manifest digest on the standard output


conductor cgi list
------------------

List all the declared functions


conductor cgi rm ID
-------------------

Remove function by id
EOF
}

warn(){
  echo "$@" >&2
}

identify(){
  cksum -a blake2b -l 128 --untagged | awk '{print $1}'
}

cgi_install(){
  cat >"/run/systemd/system/conductor-cgi-function-route@.service" <<"UNIT"
[Unit]
Requires=caddy.service
After=caddy.service

# Ensures that the unit is restarted when the Caddy server restarts (required to
# refresh the caddy config)
PartOf=caddy.service

[Service]
WorkingDirectory=/run/conductor/cgi/%i
TimeoutStartSec=0
RemainAfterExit=yes

ExecStartPre=-/bin/bash -xc ' \
  curl -s -X DELETE http://127.0.0.1:2019/id/conductor-cgi-function-route-%i/ 2>/dev/null; \
  true; \
  '

ExecStart=/bin/bash -xec ' \
  curl -s \
    -X POST \
    -H "content-type: application/json" \
    --data @$PWD/proxy-config.json \
    http://127.0.0.1:2019/id/conductor-cgi-routes/routes; \
  '

ExecStopPost=/bin/bash -xc ' \
  curl -s -X DELETE http://127.0.0.1:2019/id/conductor-cgi-function-route-%i/; \
  true; \
  '
UNIT
}

cgi_declare(){
  if [[ -d "$1" ]]; then
    warn "Directory manifest not yet implemented"
    return 1
  fi

  local manifest
  if [[ $1 == - ]]; then
    manifest=/tmp/$$.json
    trap "rm -f /tmp/$$.json" 0
    cat >"$manifest"
  elif ! [[ -f "$1" ]]; then
    warn "$1: file not found"
    return 1
  else
    manifest="$1"
  fi

  local id="$(identify <"$manifest")"
  local dir="/run/conductor/cgi/$id"

  mkdir -p "$dir"
  cp "$manifest" "$dir/manifest.json"
  manifest="$dir/manifest.json"
  rm -f /tmp/$$.json

  cgi_install

  cat <<JSON >"$dir/proxy-config.json"
{
  "@id": "conductor-cgi-function-route-$id",
  "match": [
    {
      "path": ["/cgi/$id/*"]
    }
  ],
  "handle": [
    {
      "handler": "reverse_proxy",
      "transport": {"protocol": "http"},
      "upstreams": [
        { "dial": "unix/$dir/stream.socket" }
      ]
    }
  ]
}
JSON

  local name="$(jq -r .name <"$manifest")"
  local format="$(jq -r .format <"$manifest")"

  #
  # Remove old cgi functions with the same name
  #

  systemctl stop "$name.socket" "$name.service" # Stop previous units we replace
  rm -f "/run/systemd/system/$name.service" "/run/systemd/system/$name@.service""/run/systemd/system/$name.socket"
  for f in /run/conductor/cgi/*; do
    if [[ $f == $dir ]]; then
      continue
    fi
    local n="$(jq -r .name <"$f/manifest.json")"
    if [[ $name = $n ]]; then
      local i="$(basename "$f")"
      systemctl stop conductor-cgi-f-$i.service conductor-cgi-f-$i.socket
      rm -f "/run/systemd/system/conductor-cgi-f-$i.service" "/run/systemd/system/conductor-cgi-f-$i@.service" "/run/systemd/system/conductor-cgi-f-$i.socket"
      rm -rf "$f"
    fi
  done

  #
  # Install new function
  #

  case "$format" in
    cgi|sttp-stdio)

      cat <<UNIT >"$dir/unit@.service"
[Unit]

[Service]
Type=oneshot
WorkingDirectory=$dir
StandardInput=socket
StandardOutput=socket
StandardError=journal
ExecStart=$real_zero cgi _execute $id
CollectMode=inactive-or-failed
UNIT
      cat <<UNIT >"$dir/unit.socket"
[Unit]
Requires=conductor-cgi-function-route@$id.service

[Socket]
ListenStream=$dir/stream.socket
Accept=yes

[Install]
WantedBy=sockets.target
UNIT

      ln -sf "$dir/unit@.service" "/run/systemd/system/$name@.service"
      ln -sf "$dir/unit.socket" "/run/systemd/system/$name.socket"
      systemctl daemon-reload
      systemctl enable --now "$name.socket"
      # ln -sf "$dir/unit@.service" "/run/systemd/system/conductor-cgi-f-$id@.service"
      # ln -sf "$dir/unit.socket" "/run/systemd/system/conductor-cgi-f-$id.socket"
      # systemctl daemon-reload
      # systemctl enable --now "conductor-cgi-f-$id.socket"
      ;;
    *)
      warn "Unknown format: $format"
      return 1
      ;;
  esac

}

cgi_execute_decoded(){
  jq -rj <manifest.json '(.cgi.response_headers // {}) | to_entries | map(.key+": "+.value+"\n") | .[]'
  jq -rj <manifest.json 'if .cgi.no_response_headers then "\n" else "" end'

  code="
    ( set -x;
      $(jq -r <manifest.json '.exec.command | @sh') \
      $(jq -r <manifest.json 'if .exec.stderr_as_stdout then "2>&1" else "" end')
    )"

  echo "$code" >&2
  eval "$code"
}

cgi_execute(){
  local format="$(jq -r .format <manifest.json)"
  case "$format" in
    cgi)
      export AUTH_TYPE=
      cgi-adapter -path-info-strip 2 "$real_zero" cgi _execute-decoded "$@"
      ;;
    http-stdio)
      cgi_execute_decoded
      ;;
    *)
      awk '
        /^\r$/ {exit}
        END {
          printf "HTTP/1.1 500 Internal Server Error\nContent-Type: text/plain; charset=utf-8\nConnection: close\n\nUnknown format '"$format"'\n"
        }'
      ;;
  esac
}

cgi_identify(){
  if [[ -d "$1" ]]; then
    warn "Not yet implemented"
    return 1
  fi

  local manifest
  if [[ $1 == - ]]; then
    manifest=/dev/stdin
  elif ! [[ -f "$1" ]]; then
    warn "$1: file not found"
    return 1
  else
    manifest="$1"
  fi

  identify <"$manifest"
}

service_declare(){
  warn "Not implemented"
  return 1

  # TODO: template the provided directory
  # TODO: systemd-run a unit like lcas-instance2
  # TODO: manage load balancer
}

service_hook(){
  case "$1" in
    start|stop|reload|cleanup)
      warn "Not implemented"
      return 1 ;;
    *)
      warn "Invalid hook $1"
      return 1 ;;
  esac
}

deployment_declare(){
  warn "Not implemented"
  return 1

  # TODO: systemd-run a unit like lcas-pod
  # TODO: manage load balancer
}

deployment_hook(){
  case "$1" in
    start|stop|reload|cleanup)
      warn "Not implemented"
      return 1 ;;
    *)
      warn "Invalid hook $1"
      return 1 ;;
  esac
}

case "$1" in
  service)
    case "$2" in
      declare)
        shift 2
        service_declare "$@"
        ;;
      start|stop|reload|cleanup)
        shift 1
        service_hook "$@"
        ;;
      *)
        warn "Unknown command: $@"
        help
        ;;
    esac
    ;;
  deployment)
    case "$2" in
      declare)
        shift 2
        deployment_declare "$@"
        ;;
      start|stop|reload|cleanup)
        shift 1
        deployment_hook "$@"
        ;;
      *)
        warn "Unknown command: $@"
        help
        ;;
    esac
    ;;
  cgi)
    case "$2" in
      declare)
        shift 2
        cgi_declare "$@"
        ;;
      identify)
        shift 2
        cgi_identify "$@"
        ;;
      _execute)
        shift 2
        cgi_execute "$@"
        ;;
      _execute-decoded)
        shift 2
        cgi_execute_decoded "$@"
        ;;
      *)
        warn "Unknown command: $@"
        help
        ;;
    esac
    ;;
  *)
    warn "Unknown command: $@"
    help
    ;;
esac
