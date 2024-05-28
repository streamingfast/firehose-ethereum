#!/usr/bin/env bash

ROOT="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
BLOCK_STORE_PATH="$ROOT/data/one-block-store"

main() {
    if [[ -z "$1" ]]; then
        echo "Missing RPC endpoint"
        usage
        exit 1
    fi

    pushd "$ROOT" &> /dev/null
    pushd "$BLOCK_STORE_PATH" &> /dev/null
    limit=

    while getopts ':l:-:' OPTION; do
        case "$OPTION" in
            l) limit="$OPTARG" ;;  # Set the limit if -l is passed
            -)
            case "${OPTARG}" in
                limit=*)
                limit="${OPTARG#*=}" ;;  # Set the limit if --limit= is passed
                *)
            esac
            ;;
            ?)
            usage ;;  # Display usage for invalid short options
        esac
    done

    # Remaining arguments after options
    shift $((OPTIND - 1))
    set -e
    
    j=0
    for i in `ls $BLOCK_STORE_PATH`; do
        j=$(($j+1))
        if [[ -n "$limit" && $j -gt $limit ]]; then
            break
        fi

        echo "Comparing $i"
        
        # Setting DLOG to error to avoid spamming the console
        DLOG=.*=error fireeth tools compare-oneblock-rpc $BLOCK_STORE_PATH/$i "$1"
    done
}

# TODO: add the gsutil directory link on the fireeth tool  compare-oneblock-rpc

usage() {
  echo "usage: one-block-compare.sh <rpc-endpoint>"
  echo ""
  echo "Compare locally produced one block against RPC endpoint."
  echo "Prerequiste: have fireeth installed: https://github.com/streamingfast/firehose-ethereum"
  echo "Options"
  echo "    -l, --limit        Limit the number of blocks to compare, eg.: -limit 10 (will compare 10 blocks)"
  echo ""
}

main "$@"