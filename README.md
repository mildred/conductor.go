Conductor
=========

TODO:
- timeouts on templates

Light orchestrator for Containers and CGI scripts for systemd servers running
Podman and a Caddy reverse-proxy.

Sometimes Kubernetes is overkill, even lightweight versions. When configuration
is static, you just need to manage and run services on a different set of
machines. However, you might need a light orchestrator to handle zero downtime
deployments, load balancer integration or CGI (now known as lambdas or
serverless functions)

Conductor will handle all of this for you with a few core concepts:

- **Application**: this is what it means, an application.

- **Service**: this is a particular instance of an application deployed.
  Example: production, staging, ...

- **Deployment**: within an instance, the application can be deployed a single
  time (as normal) or multiple times (to provide parallelism or handle
  version updates in deployments)

The idea is that the Conductor configuration is static on all machines, and
conditionals (to be implemented) control which services run where. CGI scripts
can be then used to control communication between the machines if needed and a
global replicated static configuration allows every service to know where each
other service resides (no need of etcd when the data store never changes).

Installation
------------

Download the executable, place it in your `PATH` and run `conductor system
install`. You should have systemd and podman on your server. It also expects to
be able to connect to the Caddy via the standard API endpoint on port 2019. The
Caddy server should be described in `caddy.service` (others units depends on
it).

Service anatomy
---------------

To declare a service, just put a JSON file in
`/etc/conductor/services/my-service/conductor-service.json` (replace
`my-service` by your service name):

```json
{
  "inherit" [
    "/path/to/the/base/conductor-service.json"
  ],
  "app_name": "beautiful-app",
  "instance_name": "staging",
  "config": {
    "BEAUTIFUL_APP_ENV": "staging",
    "DOCKER_IMAGE": "example.org/beautiful/app",
    "DOCKER_TAG": "latest",
    "SERVICE_TOGGLE": "true"
  },
  "pods": [
    {
      "name": "",
      "pod_template": "./pod.template",
      "config_map_template": "./config-map.template",
      "service_directives": []
    }
  ],
  "proxy_config_template": "./proxy-config.template",
  "hooks": [
    {
      "id": "my-start-hook",
      "when": "post-start",
      "exec": ["./post-start.hook"]
    },
    {
      "id": "my-stop-hook",
      "when": "pre-stop",
      "exec": ["./pre-stop.hook"]
    }
  ],
  "commands" {
    "migrate": {
      "service": true,
      "service_any_deployment": false,
      "deployment": false,
      "description": "Run data migrations",
      "exec": ["./migrate.cmd"]
    }
  },
  "display_service_config": ["BEAUTIFUL_APP_ENV", "DOCKER_TAG", "SERVICE_TOGGLE"],
  "display_service_deployment_config": ["BEAUTIFUL_APP_ENV", "DOCKER_TAG"],
  "display_deployment_config": ["DOCKER_VERSION"]
}
```

### Hooks

Hooks are scripts executed with the config as environment variables.

Additional variables for services and deployments:

- `CONDUCTOR_APP`
- `CONDUCTOR_INSTANCE`
- `CONDUCTOR_SERVICE_NAME`
- `CONDUCTOR_SERVICE_DIR`
- `CONDUCTOR_SERVICE_UNIT`
- `CONDUCTOR_SERVICE_CONFIG_UNIT`

Additional variables for deployments only:

- `CONDUCTOR_SERVICE_PART` name of the pod in the service
- `CONDUCTOR_DEPLOYMENT` contains the deployment name
- `CONDUCTOR_DEPLOYMENT_UNIT`
- `CONDUCTOR_DEPLOYMENT_CONFIG_UNIT`
- `POD_NAME` contains the pod name
- `POD_IP_ADDRESS` contains the pod IP address

### Templates

Templates are any executable script, variables are passed to them via the
command-line arguments or the environment variables, use what you prefer. It
should return on the standard output the templated document.

Templates executed in the context of the service are executed in the service
directory. If executed in the context of a deployment, the template is executed
in the deployment directory.

The important templates are:

- the pod template: it should return a Kubernetes pod in YAML compatible with
  `podman kube play`. This is what will run in the deployment.

- the config map template: this is an optional template that can generate a
  ConfigMap file suitable for consumption within podman

- the proxy configuration template: this should generate JSON Caddy
  configuration snippets suitable for injection via the Caddy API. It should be
  a JSON array of objects, each with:

      - `id`: where the snipped should be POSTed via the config API
      - `config`: the snipped, containing a unique `@id` key suitable for
        removal when undeploying the service
      - `register_only`: can be true to only register but never deregister the
        snippet, then the `@id` is optional. Can be used to add domain names for
        certificate automation.

  The template is called both with the service and with the deployment, at
  startup and shutdown. The template can detect it is run with the deployment
  because the variable `POD_IP_ADDRESS` is then present.

Variables accessible within templates are visible with the commands `conductor
service env` or `conductor deployment env`. It is possible to simulate a
template execution with `conductor _ service template` or `conductor _
deployment template`.

Example template:

```bash
#!/bin/bash

cat <<YAML
---
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: $CONDUCTOR_APP
  name: $POD_NAME
spec:

  hostAliases:
    - ip: "127.0.0.1"
      hostnames:
        - mongodb

  containers:

    - name: main
      image: ${DOCKER_IMAGE}:${DOCKER_TAG}
      env:
        - name: CONDUCTOR_DEPLOYMENT
          value: "${CONDUCTOR_DEPLOYMENT}"
      resources:
        limits:
          memory: "4Gi"

    - name: mongodb
      image: mongo:7.0
      command: ["--replSet", "rs0"]
      # args: []
YAML
```

### Display config

It is possible to add columns to the `conductor service ls` and `conductor
deployment ls` commands via the display_service_config and
display_deployment_config configuration options. It will show the variables for
those services and deployments

### Inheritance

It is possible to inherit from a base configuration, in which case the path
locations inherited from the base configuration will be adjusted to be relative
to the file they were declared in.

### Commands

It is possible to declare commands that can be execute with `conductor run`.
Each command must have an `exec` array to execute a script relative to the
current file.

The command must declare in what context it can be executed:

- `"service": true` if the command is executed for a service globally
- `"service_any_deployment": true` if the command is executed for a service in
  which case it will be executed in the context of any active deployment, or
  will fail if there is no compatible active deployment
- `"deployment": true` if the command is executed in the context of a deployment

The command can be executed for a service with `conductor run -s SERVICE_NAME`
or for a deployment with `conductor run -d DEPLOYMENT_NAME`.

The configuration is available as environment variables just as hooks, with the
addition of:

- `CONDUCTOR_COMMAND` contains the command name.
- `CONDUCTOR_COMMAND_DIR` working directory of the invocation, the command
itself run relative to the service or deployment directory.

Basic How-To
------------

- declare a service
- execute `conductor reload` to start the systemd units for the services you
  declared


CGI / Serverless
----------------

There is an untested implementation. You can declare functions in your service:

```json
{
  "functions": [
    {
      "name": "",
      "format": "cgi",
      "exec": ["./cgi-script.sh", "--cgi"],
      "stderr_as_stdout": false,
      "response_headers": [
        "Content-Type: text/plain; charset=utf-8"
      ],
      "no_response_headers": false,
      "path_info_strip": 2,
      "service_directives": []
    }
  ]
}
```

When the deployment corresponding to the function is started, a systemd socket
with Accept=yes is started and socket activation is used to start the script
that will handle the request.

The formats supported are:

- `cgi`: a should be compatible CGI interface
- `http-stdio`: the stdin contains a HTTP request and stdout should be replied
  with the http response. This is just passthrough of the accepted socket.

In the proxy config template, you can add this shell snippet to configure your
functions:

```bash
if [[ -n "$CONDUCTOR_FUNCTION_ID" ]]; then
  exec conductor function caddy-config
fi
```

### Future developments ###

If systemd socket activation is not enough for CGI, perhaps Conductor should
take the socket activation in its own hands. This would be necessary in order to
pre-provision functions to get faster response times (the CGI format would not
work in this case).

It should also be possible to declare such functions as daemons that could be
pre-provisioned or scaled down to zero depending on the configuration.

Basically, there are three modes of operations possible:

- single shot (CGI): each request calls a single executable that is started for
  the request and ends with the request. This is standard systemd socket
  activation with `Accept=yes`. Pre-provisioning could be difficult if sticking
  with the CGI interface and systemd cannot handle it.

- multi-shot: each executable can handle multiple requests but not in parallel.
  After a configurable number of requests, the executable can be recycled. This
  could be an HTTP/1.1 connection with keep-alive. Systemd to my knowledge is
  not able to pre-provision such services.

- parallel: a single executable can handle all requests, like a daemon, but it
  can scale down to zero in the absence of requests and be reloaded when a
  connection arrives. Systemd can handle this with standard socket activation
  (Accept=no)

Roadmap:

- [x] A service pod should be optional in which case it does not generate a
  deployment
- [x] A service can declare CGI functions, each function is then started as a
  systemd socket and service that executes the CGI script via cgi-adapter (to be
  included in conductor)
- [x] The proxy config template should be called for the CGI functions too, and
  be given the service unique ID as variable
- [ ] Add socket activation to pod (allow multiple unix sockets for a single
  pod).
- [x] Allow multiple pods in a single service
- [x] Allow raw HTTP CGI scripts with Accept=yes that can handle multiple
  requests on a single keep alive connection
- [ ] Let Conductor handle socket activation for the multiple requests use case,
  and let Conductor handle preloading of the CGI executable.
    - Add conductor-fast-function@.service which will depend on
    - conductor-fast-function-manager.service which will receive start and stop
      signal from individual functions
    - it will create the socket and manage the function execution either via
      systemd-run (so it can pass around the socket) or by other means
    - it can handle the equivalent of Accept=yes sockets and pre-provision the
      function processes. This would improve over systemd socket activation by:
          - pre-provisioning the process, no cold start
          - ~the same process could be used for two separate clients that would
            otherwise use separate connections and separate processes~ (wrong
            because there is a single connection with Caddy, except that perhaps
            Caddy does not maintain an open connection)
      instead of using systemd-run, the conductor fast function manager could
      listen on a socket A (bound to the reverse proxy) and manage a reverse
      proxy to a socket B (managed by conductor-cgi-function@.socket). By
      reverse-proxying, the manager could handle pre-provisioning by opening a
      connection and not writing to it
    - it can handle the equivalent of Accept=no sockets in which case the main
      socket would have to be passed as a file descriptor. This would improve on
      systemd socket activation by:
          - pre-provisioning the process, no cold start
          - because otherwise Conductor does not support socket activation yet
- [ ] It should be possible to have commands accessible as functions with a sane
  protocol and security
- [ ] Handle security policies (see below)

### Security (not yet implemented) ###

Security can be handled using Caddy reverse_proxy rewrite handlers. See:

- https://caddyserver.com/docs/caddyfile/directives/forward_auth
- https://caddyserver.com/docs/json/apps/http/servers/routes/handle/reverse_proxy/#rewrite
- https://caddyserver.com/docs/json/apps/http/servers/routes/handle/reverse_proxy/handle_response/routes/handle/headers/

For CGI/lanbdas, the reverse proxy should be configured to forward a copy of the
request to the authentication service which can then decide if the request is
authorized (is the API token authorized or not).

The key here is to have two handlers: the first a reverse proxy to the auth
service with the rewrite configuration. In this first handler the
handle_response block is configured to use upon auth success the `headers`
handler to copy the headers to the request, and upon failure a handler can
respond with an unauthorized message. Then the second handler follows the first
with the rewrite and forwards to the legitimate upstream if the auth succeeds.

Conductor should setup such an auth service which can detect a list of bearer
tokens to authorize CGI functions. CGI functions can be associated with
policies, and policies can specify a number of rules.

The policy could contain :
- a list of static bearer tokens accepted
- a list of JWT public keys accepted (RS*, ES*)

Services making use of these tokens will have to have the tokens or JWT private
keys configured

Actions
-------

It should be possible to declare actions in the services. For example an action
could perform a data migration, another could enter a console program to debug
the application.

It should be as simple as running `conductor action run SERVICE db:migrate ...`.

It should be easy to declare an action to be accessible remotely via a CGI
script, security should prevent unauthorized access.

Clustering
----------

Conductor machines can work in cluster mode with a gossip protocol

Cluster creation flow:

- foreign node generates an invite key pair with `conductor peer invite`
  the invite key is stored in a separate policy that is not a peer (no
  peer-hostname in meta) and that is authorized to add a peer. The secret key is
  shown on the standard output
- current node adds the foreign node with `conductor peer add foreign INVITE_SECRET`
- current node generates a secret key for itself stored in the metadata of the
  policy matcher. It sets peer-hostname to the current fqdn and sets the public
  key as allowed token
- current node dials the foreign node and authenticate with the secret key
  provided. It executes a CGI function to add a peer. It add itself to the
  foreign peer. The remote:
    - the foreign node add the first node to the peer list with the public key
      it provided
    - the foreign node generates a secret key for itself if needed and sets
      itself in the list of peers providing the current fqdn
    - the foreign node returns to the first node its public key
- upon success, it adds the remote peer to the current list of peers

Commands:

- `conductor peers list`: list all peers in the cluster
- `conductor peers pubkey [HOSTNAME]`: get the public key for the named peer, or
  current. This key allows other peers to add themselves to the cluster
- `conductor peers add HOSTNAME PUBKEY`: add the peer by hostname and public
  key, this will also 

Fleet
-----

Conductor should come with:

- a default Caddy systemd unit (reverse proxy, configured for CGI and services)
- a sidecar daemon to synchronize a configuration directory
    - either using NATS Jetstream object store
      https://github.com/corpix/ndfs
    - a syncthing instance, probably overkill and difficult to configure and
      maintain (no need for 2 way sync)

Conductor should come with a few build-in functions that:

- uploads to the local disk a service directory
- enable/disable/start/stop services uploaded by function above
- connects the file synchronization with another conductor machine

A fleet orchestrator daemon should be provided that would share the common
configuration via the file synchronization tool and would trigger functions on
the orchestrator instances to enable/start services depending on allocation

The fleet orchestrator could be implemented on each conductor instance and
managed using NATS messenging. A conductor instance would take a lock on a
service definition, evaluate where it should be scheduled, then release the
lock.

Orchestration strategies:

- each conductor instance would have a config file describing the machine it is
  running on with a number of resources (abstract). The machine could be
  declared to have 4 VCPU, 100 GBRAM, 10 NETWORKPORTS

- each service would declare the best and minimum numbers for these units

- if a machine can be found that can allocate the best resource for all its
services, it is chosen. Else, each machine is evaluated in turn, machine that
cannot allocate the minimum resources are excluded. Then an overcommit ratio is
computed for each unit, and the maximum overcommit is selected to evaluate the
machine. The chosen machine is the one with the less overcommit.

### Function: add peer

authentication: JWT or bearer token signed by the member

inputs:

  - policy name
  - peer id (mandatory)
  - peer hostname (optional)
  - peer authentication method (list):
      - public key (JWT token)
      - bearer token
  - authorizations (object of boolean values)
      - peer-list-read
      - peer-list-write
      - service-write
  - if the member add should be gossipped to other members
  - the list of member ids that received this message already (for gossip)

Create an Any matcher in the policy with the peer info in the matcher

Add metadata to the matcher:
  - peer id
  - peer hostname
  - authorizations


```json
{
  match: [
    {
      any: [
        {
          "meta": {
            "peer-id": "1",
            "peer-hostname": "peer-1.example.org"
          },
          default_authorizations: {
            "peer-list-read": true,
            "peer-list-write": true,
            "service-write": true
          },
          bearer: [
            { "token": "secret" }
          ]
        }
      ]
    }
  ]
}
```

### Function: remove peer

authentication: JWT or bearer  token signed by the member

inputs: 
  - policy name
  - peer id
  - if the member remove should be gossipped to other members
  - the list of member ids that received this message already (for gossip)

Remove an Any matcher from the policy

### Function: list peers

authentication: JWT or bearer token signed by the member

inputs:
  - policy name

outputs: list of peers

### Function: write service

authentication: JWT or bearer token signed by the member

inputs:
  - a list of:
      - service name
      - service files (archive file)
      - service version string
  - should the service write should be gossipped or not

does nothing if the service is already at the same version

