#!/usr/bin/env bash

set -Eeuo pipefail

ROOT="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

usage=$(cat <<END
usage: repack.sh

Unpack the node archive, wait for user input that modification(s)
are done and repack the archive

Requires binary 'tar' and 'zstd' to be installed.

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

  archive_file_name="sei-dev-chain.tar"
  archive_file_path="$ROOT/$archive_file_name"

  echo "Unpacking archive ..."
  mkdir -p unpacked
  trap "rm -rf $ROOT/unpacked; rm -rf ${archive_file_path}" EXIT

  pushd unpacked &> /dev/null
    tar --use-compress-program=unzstd -xf "${archive_file_path}.zst"

    echo ""
    echo "The archive has been unpacked to '$ROOT/unpacked' directory."
    read -p "Please make the necessary changes and press [Enter] to repack the archive"

    echo ""
    echo "Re-packing archive ..."
    tar  -cf "${archive_file_path}" *
    zstd -f -q --no-progress "${archive_file_path}"
  popd &> /dev/null

  echo Done
}

usage_error() {
  message="$1"
  exit_code="$2"

  echo "ERROR: $message"
  echo ""
  echo "$usage"
  exit ${exit_code:-1}
}

main "$@"
