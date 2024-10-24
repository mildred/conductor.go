Conductor
=========

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
    "DOCKER_TAG": "latest"
  },
  "pod_template": "./pod.template",
  "config_map_template": "./config-map.template",
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
  "display_service_config": ["BEAUTIFUL_APP_ENV", "DOCKER_TAG"],
  "display_deployment_config": ["DOCKER_VERSION"]
}
```

### Hooks

Hooks are scripts executed with the config as environment variables.

Additional variables for services and deployments:

- `CONDUCTOR_APP`
- `CONDUCTOR_INSTANCE`
- `CONDUCTOR_SERVICE_DIR`
- `CONDUCTOR_SERVICE_UNIT`
- `CONDUCTOR_SERVICE_CONFIG_UNIT`

Additional variables for deployments only:

- `CONDUCTOR_DEPLOYMENT` contains the deployment name
- `CONDUCTOR_DEPLOYMENT_UNIT`
- `CONDUCTOR_DEPLOYMENT_CONFIG_UNIT`
- `POD_NAME` contains the pod name
- `POD_IP_ADDRESS` contains the pod IP address

### Templates

Templates are any executable script, variables are passed to them via the
command-line arguments or the environment variables, use what you prefer. It
should return on the standard output the templated document.

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

It is not yet implemented, but the general ideas are there. A CGI script or
serverless lambda would be implemented using systemd socket activation, with the
socket tied to a route in the reverse proxy. Upon activation, something like
[`cgi-adapter`](https://github.com/mildred/cgi-adapter) could be used to
translate the HTTP request to CGI and back to HTTP.

If systemd socket activation is not enough for CGI, perhaps Conductor should
take the socket activation in its own hands. This would be necessary in order to
pre-provision CGI scripts to get faster response times.

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

- [ ] A service pod should be optional in which case it does not generate a
  deployment
- [ ] A service can declare CGI functions, each function is then started as a
  systemd socket and service that executes the CGI script via cgi-adapter (to be
  included in conductor)
- [ ] The proxy config template should be called for the CGI functions too, and
  be given the service unique ID as variable
- [ ] Add socket activation to pod (allow multiple unix sockets for a single
  pod).
- [ ] Allow multiple pods in a single service
- [ ] Allow raw HTTP CGI scripts with Accept=yes that can handle multiple
  requests on a single keep alive connection
- [ ] Let Conductor handle socket activation for the multiple requests use case,
  and let Conductor handle preloading of the CGI executable.

### Security

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
