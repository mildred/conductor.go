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

Installation
------------

Download the executable, place it in your `PATH` and run `conductor system
install`. you should have systemd and podman on your server. It also expects to
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
  "display_service_config": ["BEAUTIFUL_APP_ENV", "DOCKER_TAG"],
  "display_deployment_config": ["DOCKER_VERSION"]
}
```

### Hooks

Hooks are scripts executed with the config as environment variables.

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

Basic How-To
------------

- declare a service
- execute `conductor reload` to start the systemd units for the services you
  declared


