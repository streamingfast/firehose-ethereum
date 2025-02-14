#!/usr/bin/env bash

ROOT="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

ENDPOINT="${ENDPOINT:-arb-one.streamingfast.io:443}"
MERGED_BLOCKS="${MERGED_BLOCKS:-"$ROOT/firehose-data/storage/merged-blocks"}"

main() {
  mkdir "$ROOT/comparisons"

  firecore_tools="firecore tools --output=protojson"

  while IFS= read -r line; do
    if [[ $line == *"Block "* ]]; then
        block_number=$(echo $line | awk '{print $3}')

        $firecore_tools firehose-single-block-client "$ENDPOINT" $block_number  | jq .block > "$ROOT/comparisons/$block_number.current.json"
        $firecore_tools print merged-blocks "$MERGED_BLOCKS" $block_number | jq '.' > "$ROOT/comparisons/$block_number.new.json"
        ${DIFF_EDITOR:-"diff -u"} "$ROOT/comparisons/$block_number.current.json" "$ROOT/comparisons/$block_number.new.json"
    fi
  done
}

main
