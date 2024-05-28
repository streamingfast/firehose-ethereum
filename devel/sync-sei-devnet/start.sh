#!/usr/bin/env bash

ROOT="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

clean=
fireeth="$ROOT/../fireeth"

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

  fh_data_dir="$ROOT/firehose-data"

  set -e

  if [[ $clean == "true" ]]; then
    printf "Are you sure you want to clean the data directory? [y/N] "
    read -r answer
    if [[ $answer != "y" ]]; then
      echo "Aborting"
      exit 0
    fi

    rm -rf "$fh_data_dir" &> /dev/null || true
  fi

  $fireeth -c $(basename $ROOT).yaml start "$@"
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
  echo "usage: consensus.sh [-c]"
  echo ""
  echo "Start $(basename $ROOT) environment."
  echo ""
  echo "Options"
  echo "    -c             Clean actual data directory first"
  echo ""
  echo "Examples"
  echo "   Stream blocks    grpcurl -plaintext -d '{\"start_block_num\": -1}' localhost:8089 sf.firehose.v2.Stream/Blocks"
}

main "$@"
