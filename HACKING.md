Develop conductor using systemd user session
--------------------------------------------

    ./test-instance.sh [ -p PORT ]

This executes a tmux with:

  - a Caddy instance listening on provided port (8080 by default)
  - systemd configured as user daemon
  - journalctl for monitoring
  - a shell configured for the instance

Beware that systemd user can be stopped using Ctrl-C but it takes some time to
do so

Services in the examples directory are automatically linked to the environment
as available services. The conductor executable is linked from this directory
and can be rebuilt using:

    go build ./cmd/conductor
