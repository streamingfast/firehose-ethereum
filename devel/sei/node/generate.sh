#!/usr/bin/env bash

set -Eeuo pipefail

ROOT="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

usage=$(cat <<END
usage: generate.sh <sei-chain-path>

Runs the necessary commands to generate the a local Sei node and pack
it.

The script uses the scripts/initialize_local_chain.sh
to generate the node and then it packs it into a tar.zst file.

Requires binary 'sd', 'wget', 'jq', 'curl', and 'seid'
to be installed.

Options
    -h          Display help about this script
END
)

main() {
  pushd "$ROOT" &> /dev/null

  while getopts "h" opt; do
    case $opt in
      h) echo "$usage" && exit 0;;
      \?) usage_error "Invalid option: -$OPTARG";;
    esac
  done
  shift $((OPTIND-1))

  if [[ ${1:-} == "" ]]; then
    usage_error "Missing <sei-chain-path> argument"
  fi

  if [[ ! -d "${1:-}" ]]; then
    usage_error "The path '$1' does not exist or is not a directory"
  fi

  sei_chain_path="$1"
  initialize_script_relative_path=scripts/initialize_local_chain.sh
  initialize_script="$sei_chain_path/$initialize_script_relative_path"

  sei_home="$HOME/.sei"
  app_file="$sei_home/config/app.toml"
  archive_file_name="sei-dev-chain.tar"
  archive_file_path="$ROOT/sei-dev-chain.tar"

  echo "About to bootstrap Sei development node"
  echo " Home: $sei_home"
  echo " Chain: $sei_chain_path"
  echo " App Config: $app_file"
  echo ""

  echo "Running $initialize_script ..."
  pushd "$sei_chain_path" &> /dev/null
    NO_RUN=1 "$initialize_script_relative_path"
  popd &> /dev/null

  pushd "$sei_home" &> /dev/null
    echo "Adjusting config/app.toml to use FirehoseTracer"
    sd '\[evm\]' "[evm]\nlive_evm_tracer = \"firehose\"" "$app_file"

    echo "Packing archive ${archive_file_path}.zst..."
    tar -cf "${archive_file_path}" *
    zstd -f -q --no-progress "${archive_file_path}"

    rm -rf "${archive_file_path}"
  popd &> /dev/null

  # We print Done in the revert scripts changes which is a trap on EXIT
}

usage_error() {
  message="$1"
  exit_code="${2:--1}"

  echo "ERROR: $message"
  echo ""
  echo "$usage"
  exit ${exit_code}
}

main "$@"