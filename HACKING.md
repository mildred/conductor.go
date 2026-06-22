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

Update flaggy
-------------

### Set up local replacement

We use a custom version of flaggy. To test against a development version of
flaggy, specify in `go.mod`:

    replace github.com/integrii/flaggy => ../flaggy

### Set up remote replacement

To replace it with a remote version, commit to the forked repository then
specify in `go.mod`:

    replace github.com/integrii/flaggy => github.com/mildred/flaggy latest

Then run:

    go mod download github.com/integrii/flaggy

### Update remote replacement

In `go.mod` replace the version number with `latest` then run:

    go mod download github.com/integrii/flaggy
