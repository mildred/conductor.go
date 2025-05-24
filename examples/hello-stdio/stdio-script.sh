#!/bin/bash

cd "$(dirname "$0")"

go build -o stdio . >&2
set -x
exec ./stdio "$@"
