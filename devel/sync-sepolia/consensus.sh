#!/usr/bin/env bash

ROOT="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

clean=
lighthouse="${LIGHTHOUSE_BIN:-lighthouse}"

main() {
  pushd "$ROOT" &> /dev/null

  while getopts "hc" opt; do
    case $opt in
      h) usage && exit 0;;
      c) clean=true;;
      \?) usage_error "Invalid option: -$OPTARG";;
    esac
  done
  shift $((OPTIND-1))
  [[ $1 = "--" ]] && shift

  set -e

  if [[ $clean == "true" ]]; then
    rm -rf cs-data &> /dev/null || true
  fi

  exec "$lighthouse"\
    beacon\
    --datadir="$ROOT/cs-data"\
    --debug-level=info\
    --network=sepolia\
    --listen-address=0.0.0.0\
    --port=9090\
    --http\
    --http-address=0.0.0.0\
    --http-port=5052\
    --metrics-address=0.0.0.0\
    --metrics-port=9102\
    --execution-jwt-id=\
    --checkpoint-sync-url="${CHECKPOINT_SYNC_URL}"\
    --execution-endpoint=http://localhost:9551\
    --execution-jwt="$ROOT/jwt.txt" "$@"
}

usage_error() {
  message="$1"
  exit_code="$2"

  echo "ERROR: $message"
  echo ""
  usage
  exit ${exit_code:-1}
}

usage() {
  echo "usage: start.sh [-c]"
  echo ""
  echo "Start Consensus Client (lighthouse) for $(basename $ROOT) environment."
  echo ""
  echo "Two environment variables can tweak it:"
  echo ""
  echo " 'LIGHTHOUSE_BIN' | Absolute path where to find 'lighthouse' binary, defaults to 'lighthouse' which will be picked in your 'PATH'"
  echo " 'CHECKPOINT_SYNC_URL' | URL where 'lighthouse' can reach to bootstrap the consensus client database from a pre-made checkpoint, sets '--checkpoint-sync-url' flag on 'lighthouse' startup. A community maintained list of public checkpoint sync URLs can be seen https://eth-clients.github.io/checkpoint-sync-endpoints/. **Important** if you already have some data for your consensus client, '--checkpoint-sync-url' has no effect, delete 'cs-data' folder first and then start it back with the environment variable"
  echo ""
  echo "Options"
  echo "    -c             Clean actual data directory '$ROOT/cs-data' first"
 }

main "$@"
