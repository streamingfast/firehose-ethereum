#!/bin/bash
echo "Here we go!"

function process_block_range() {
  local version="$1"
  local start_block="$2"
  local stop_block="$3"

  local output_file="$version-$start_block-$stop_block.jsonl"
  local block_range="$start_block:$stop_block"

  fireeth tools print merged-blocks "gs://dfuseio-global-blocks-uscentral/arb-one/$version?project=dfuseio-global" "$block_range" -o jsonl | \
    jq 'del(
    .detail_level,
    .header.total_difficulty,
    .header.gas_used,
    .transaction_traces[]?.calls,
    .transaction_traces[]?.status,
    .transaction_traces[]?.type,
    .transaction_traces[]?.max_fee_per_gas,
    .transaction_traces[]?.max_priority_fee_per_gas,
    .transaction_traces[]?.gas_used,
    .transaction_traces[]?.begin_ordinal,
    .transaction_traces[]?.end_ordinal,
    .transaction_traces[]?.receipt.logs_bloom,
    .transaction_traces[]?.receipt.cumulative_gas_used,
#    .transaction_traces[]?.receipt.logs[]?.index,
    .transaction_traces[]?.receipt.logs[]?.ordinal)' > "/tmp/merged-blocks-compare/$output_file"

    echo "$output_file"
}

mkdir -p /tmp/merged-blocks-compare

v1File=$(process_block_range v1 22208500 22208600)
vPollerFile=$(process_block_range vPoller 22208500 22208600)

echo "Diffing $v1File and $vPollerFile"

diff -C0 "/tmp/merged-blocks-compare/$v1File" "/tmp/merged-blocks-compare/$vPollerFile"
