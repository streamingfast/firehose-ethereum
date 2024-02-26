#!/usr/bin/env bash

set -Eeuo pipefail

ROOT="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

usage=$(cat <<END
usage: generate.sh

Runs the necessary commands to generate the necessary SEI Devnet
configuration files in preparation of a full sync.

The script use 'seid' to init folder structure and 'sd' to update
config.toml file. Trust height and hash are updated based on the
latest block height and hash from the primary endpoint.

At this point the folder structure is compressed and moved to the
root folder of the repository.

Requires binary 'sd', 'wget', 'jq', 'zstd', 'curl', 'tar' and 'seid'
to be installed.

Options
    -h          Display help about this script
END
)

main() {
  pushd "$ROOT" &> /dev/null

  chain=${CHAIN:-"arctic-1"}
  node_binary=${READER_NODE_BINARY_PATH:-"seid"}
  node_home=${READER_NODE_NODE_DATA_DIR}
  if [[ -z "$node_home" ]]; then
    usage_error "READER_NODE_NODE_DATA_DIR must be set"
  fi

  while getopts "h" opt; do
    case $opt in
      h) echo "$usage" && exit 0;;
      \?) usage_error "Invalid option: -$OPTARG";;
    esac
  done
  shift $((OPTIND-1))

  sei="seid --home $node_home"
  config_file="$node_home/config/config.toml"
  genesis_file="$node_home/config/genesis.json"

  primary_endpoint="https://rpc-${chain}.sei-apis.com"
  secondary_endpoint="https://rpc.${chain}.seinetwork.io"
  genesis_url="https://github.com/sei-protocol/testnet/raw/main/${chain}/genesis.json"

  echo "About to bootstrap SEI node"
  echo " Command: $sei"
  echo " Chain: $chain"
  echo " Config: $config_file"
  echo " Genesis: $genesis_file"
  echo ""

  mkdir -p "$node_home"

  $sei init sf-firehose-devnet-node --chain-id ${chain} -o

  echo "Updating config/config.toml ..."
  sd 'persistent-peers = ""' 'persistent-peers = "de8b1df70c7a8817ed121908e7c6e6059f4238f9@3.142.50.176:26656,7a962f3a928ca4e0e58355e6e798aba1ea253272@34.242.85.117:26656,0490865aebc5cd85ecaf8e03878b53b0d1f4dd0b@3.64.60.2:26656,b4dd0a39392118be51f9dd9859c09e91ce1fb6a3@3.17.4.173:26656,d980319d3a77af4ac57e6ff252822e74adf12247@18.193.125.51:26656,0682143c1b580617bc1b64813d6cea02b0373d06@3.73.73.115:26656,956568957540b2b531189b9d9d310e71ad92c098@35.159.50.215:26656,56aff82773ad67bc67b697b07cfacf395bac470c@18.200.244.54:26656,fa525576efbdaaabc558bfdd12a438c7f5ee2480@54.93.165.193:26656,9a6c2741d0fd81b8a1711081edb1639a0e763f43@3.122.61.76:26656,1beab85a23169387f97df1e4f49748390c851286@3.73.158.122:26656,0fe73d7f5cc6f3025f9ab0aecad2b2296b9f9c90@18.156.121.116:26656,8ac5d20de52b3435854c9e30bba4dbf81d153933@3.248.190.196:26656,c9ac1c958f1c00ad663700a584b7088f8cf7888d@34.244.72.65:26656,1f119c92bbfa18eda62f2d62915e46c09240a120@52.15.241.149:26656,32520cab111f0ea4ded0e5724be811548a45e33a@3.255.170.104:26656,b4ba086ebdd44e9de321964141528ecc873c56ca@3.254.136.7:26656,cedd0693ec5d238fc994961eee5002a8d34d5bca@52.215.89.89:26656,d47908feaa06b16112758652cc4f339a638df07f@13.58.252.221:26656,8f9ce5e9db04a103282090691244541962c7a580@3.134.87.147:26656,343909b55865776e7ba0a6e992946461fe6ce9fa@54.75.75.216:26656,2284dd7cb0a126e45aa1b1d14c53f47e0d048aae@52.29.21.234:26656,229207ce855ac521491817a43672cf900e52694a@63.33.61.88:26656,70984d67b8088ae4e26a4ed34abea85bd1662300@13.58.143.114:26656,fb5e128b5fd3340bf5d60c822d7c97b3a0602d5b@3.19.142.242:26656,10586f63fee205b857740e86a66d4d7827e69158@18.197.42.160:26656,85f81a3ba6c3d75584bede0cd71281941b78ad3f@34.254.248.141:26656,af2a87ae0b3b2519a442406d11d5a9a1b965fc87@18.188.207.75:26656,feca131d5da0cfc78e9eb9aec95cf26ce53a0c71@18.225.11.134:26656,86bfa3f8d9968ca1fa205356015041451c99a9b9@3.133.59.72:26656,cce0882a656cd97c6bac7f0f0bc04139a3d390f3@18.222.200.6:26656"' $config_file
  sd '^rpc-servers *=.*' "rpc-servers = \"$primary_endpoint,$secondary_endpoint\"" $config_file
  sd '^enable *=.*' "enable = true" $config_file
  sd '^db-sync-enable *=.*' "db-sync-enable = false" $config_file

  update_trust_checkpoint "$primary_endpoint" "$config_file"

  echo "Fetching genesis file ..."
  wget -q -O "$genesis_file" "$genesis_url" 1> /dev/null

  echo Done
}

function update_trust_checkpoint() {
  endpoint="$1"
  config_file="$2"
  trust_height_delta=10000
  snapshot_sync_rounding_delta=1000

  echo "Fetching sync trust block height ..."
  latest_height=$(curl --fail -sS "$endpoint"/block | jq -r ".block.header.height")

  if [[ "$latest_height" -gt "$trust_height_delta" ]]; then
    sync_block_height=$(($latest_height - $trust_height_delta))
  else
    sync_block_height=$latest_height
  fi

  echo "Fetching sync trust block hash ..."
  sync_block_hash=$(curl --fail -sS "$endpoint/block?height=$sync_block_height" | jq -r ".block_id.hash")

  sd '^trust-height *=.*' "trust-height = $sync_block_height" "$config_file"
  sd '^trust-hash *=.*' "trust-hash = \"$sync_block_hash\"" "$config_file"

  first_streamable_block=$((($sync_block_height + $trust_height_delta) - ($sync_block_height % $snapshot_sync_rounding_delta) + 1))

  echo ""
  echo "**Important**"
  echo "Don't forget to use '--common-first-streamable-block=$first_streamable_block' so Firehose starts from snapshopt height."
  echo "Also important, this value is actually dynamic and is based on the actual block height when the snapshot sync ends"
  echo "so it's not really possible to infer a value that is going to always work."
  echo ""
  echo "If you get an error that received block is after first streamable block, restart using a closer"
  echo "value to the actual chain's block height (the one you see in 'seid' output logs as finalized block)."
  echo ""
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