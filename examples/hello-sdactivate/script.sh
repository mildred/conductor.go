#!/bin/bash

cd "$(dirname "$0")"

set -x
go build -o main . >&2
exec ./main "$@"
