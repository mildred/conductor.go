#!/bin/bash

self="$0"
selfbase="${0##*/}"
absself="$(readlink -f "$0")"

show_options(){
  # Documentation comments starts by ### followed by an optional integer
  # followed by :. The integer serves as ranking comments from first to last on
  # display.
  #
  # As a special case, if "$0" appears first after the : sign, it is replaced by
  # the script name.
  #
  # When the comment appears on the same line as a case branch statement, the
  # case branch expression is reused on the output. There are different way to
  # specify the comment after the ":" sign:
  #
  #   - When the comment start with "-"
  #
  #     The case branch value is displayed indented once followed by the comment
  #     following the "-" disn indented twice.
  #
  #     Example: -h|--help) ###?: - Show help
  #
  #   - When the comment start with a word optionally followed by "--"
  #
  #     The case branch value is not displayed, instead the part before "--" is
  #     displayed indented once (replacing the case branch value) and the part
  #     after "--" is displayed indented twice to explain the option. When there
  #     is no "--", the first word (non whitespace) us used for option name
  #
  #     Example: --root)   ###?: --root DIRECTORY -- Set the root directory
  #
  #     Example: --root=*) ###?: --root=DIRECTORY Set the root directory
  #
  sed -rn \
    -e 's/\s*(\S*)\)\s\s*###([0-9]*:)\s*-\s\s*(.*)/\2\t\1\n\2\t\t\3\n\2/p' \
    -e 's/.*\)\s\s*###([0-9]*:)\s*(.*)\s*\s--\s\s*(.*)/\1\t\2\n\1\t\t\3\n\1/p' \
    -e 's/.*\)\s\s*###([0-9]*:)\s*(\S*)\s\s*(.*)/\1\t\2\n\1\t\t\3\n\1/p' \
    -e 's/^\s*###([0-9]*:)\s?(\s*)\$0(\s.*)/\1\2'"$(basename "$0")"'\3/p' \
    -e 's/^\s*###([0-9]*:)\s?(.*)/\1\2/p' \
    "$self" \
  | grep -n '' \
  | sed -r 's/^([0-9]*):([0-9]*):/\2.\1:/' \
  | sort -t: -k 1V \
  | cut -d: -f2-
}

main(){
  local test_hostname=root@conductor-tests

  while [[ $# -gt 0 ]]; do
    case "$1" in
      ##    0.........1.........2.........3.........4.........5.........6.........7.........8
      ###0: $0 [OPTIONS...] COMMAND
      ###0:
      ###0: This is the conductor tests harness.
      ###0:
      ###0: OPTIONS:
      ###0:
      -h|--help) ###1: - Show Help
        show_options
        exit 1
        ;;
      --test-hostname=*) ###1: - The user@hostname for the test machine [default: conductor-tests]
        test_hostname="${1#*=}"
        ;;
      *)
        break
        ;;
    esac
    shift
  done

  case "$1" in
    ##    0.........1.........2.........3.........4.........5.........6.........7.........8
    ###2: COMMAND:
    ###2:
    install) ###3: install INSTALL_OPTIONS -- install conductor on the test machine
      shift
      do_install "$@"
      ;;
    upload) ###3: upload USER@MACHINE -- upload development conductor on target machine
      shift
      do_upload "$@"
      ;;
  esac
}

do_install(){
  local install_tests=true

  while [[ $# -gt 0 ]]; do
    case "$1" in
      ##    0.........1.........2.........3.........4.........5.........6.........7.........8
      ###4: $0 install [OPTIONS...]
      ###4:
      --no-tests) ###4: - Do not install the test services
        install_tests=false
        ;;
      *)
        echo >&2 "ERROR: unexpected argument to install: $@"
        exit 1
        break
        ;;
    esac
    shift
  done

  ( set -x; go build -o conductor ./cmd/conductor ) || exit 1
  ( set -x; scp ./conductor $test_hostname:conductor ) || exit 1
  ( set -x; ssh $test_hostname sudo ./conductor system install -unlink ) || exit 1

  if $install_tests; then
    ( set -x; rsync -av --delete --rsh=ssh test_config/ $test_hostname:/etc/conductor/) || exit 1
    ( set -x; ssh $test_hostname sudo conductor reload )
  fi
}

do_upload(){
  local install=false

  while [[ $# -gt 0 ]]; do
    case "$1" in
      ##    0.........1.........2.........3.........4.........5.........6.........7.........8
      ###4: $0 upload [OPTIONS...] USER@HOSTNAME
      ###4:
      --install) ###4: - Perform a conductor install as root
        install=true
        ;;
      *)
        break
        ;;
    esac
    shift
  done

  if [[ $# -ne 1 ]]; then
    echo >&2 "ERROR: expect only one argument to upload, got: $@"
    return 1
  fi

  local hostname="$1"

  ( set -x; go build -o conductor ./cmd/conductor ) || exit 1
  ( set -x; scp ./conductor $hostname:conductor ) || exit 1

  if $install; then
    ( set -x; ssh $hostname 'conductor version; trap "conductor version" EXIT; sudo ./conductor system install -unlink' )
  fi
}

main "$@"
