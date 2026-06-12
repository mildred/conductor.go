Changelog
=========

v0.0.84 (unreleased)
-------

### New features

- Add subcommand `conductor addr` with up, down and update. This create local only
  addresses (127.0.0.X for IPv4, and a ULA prefix for IPv6). The `up` subcommand
  associates a hostname with a newly allocated address, the `down` subcommand
  removes this hostname and address. IPv4 addresses are added to the loopback
  device and IPv6 addresses are added to a dummy `addr0` interface. The ULA prefix
  is generated upon installation and can be configured via a `conductor.tolm`
  configuration file.

- Add `conductor api-server` to serve the API server from CLI. The API server is
  currently empty.

- Install default conductor services: when installed, conductor will install
  default conductor services (caddy reverse proxy for instance). The install
  subcommand now gains `--enable`, `--now` and `--enable-now` flags.

- Add a new part type in addition to pods and functions: daemons. Daemons are
  long running processed that are not containerized. It shares most behaviours
  with pods except it does not use podman. Daemons bring up an addr with the
  service name when started.

- Parts should not be declared using the `parts` key in the configuration, each
  part must define its type (pod, function or daemon)

### Bug fixes

- service enable and disable, when services are configured with auto_restart =
  false or static_deployments = true, will not stop or start the deployments in
  the foreground.

v0.0.83
-------

### Changes

- `conductor service config set` gains a new option: `-s` / `--no-inherit`

v0.0.82
-------

### Changes

- The setting `auto_restart` has been deprecated in favor of `static_deployments`
  which means the opposite. Whereas the default value for `auto_restart` is
  `true`, the default value for `static_deplotments` is `false`.

v0.0.81 or before
-----------------

### Breaking changes

- Policies are now directories that must contain a single file `policy.json`.
  You must manually migrate all your policies.

