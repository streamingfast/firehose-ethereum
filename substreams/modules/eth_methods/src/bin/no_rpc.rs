//! Handlers performing no RPC at all, and therefore importing neither of the two extensions.
//!
//! This binary instantiates against any `fireeth` instance, one started without a single RPC
//! endpoint flag included, which is what makes `map_transfers` usable to tell a decoding problem
//! apart from an RPC one.

use substreams::errors::Error;
use substreams_ethereum::pb::eth::v2 as eth;
use substreams_ethereum::Event;

use eth_methods::abi;
use eth_methods::hex0x;
use eth_methods::pb::sf::ethereum::substreams::ethmethods::v1::{
    Erc20Transfer, Erc721Transfer, EthCallReport, EthGetBalanceReport, EthMethodsReport, Transfers,
};

// Never runs: `fireeth` instantiates the binary and calls the handler export it needs by name.
// Cargo builds `src/bin` targets as executables, which is the only shape producing one wasm file
// per source file, and an executable is required to have an entry point.
fn main() {}

#[substreams::handlers::map]
fn map_transfers(block: eth::Block) -> Result<Transfers, Error> {
    let mut erc20_transfers = Vec::new();
    let mut erc721_transfers = Vec::new();

    for trx in block.transactions() {
        let tx_hash = hex0x(&trx.hash);

        for (log, _call) in trx.logs_with_calls() {
            // The two standards share the same `Transfer` topic and are told apart by their
            // indexed parameter count, which is exactly what the generated matchers check: the
            // ERC-20 one wants 3 topics and a 32 bytes payload, the ERC-721 one 4 topics and an
            // empty payload. The two are mutually exclusive, so the order below is irrelevant.
            if let Some(transfer) = abi::erc20::events::Transfer::match_and_decode(log) {
                erc20_transfers.push(Erc20Transfer {
                    block_number: block.number,
                    tx_hash: tx_hash.clone(),
                    log_index: log.index,
                    contract: hex0x(&log.address),
                    from: hex0x(&transfer.from),
                    to: hex0x(&transfer.to),
                    value: transfer.value.to_string(),
                });
                continue;
            }

            if let Some(transfer) = abi::erc721::events::Transfer::match_and_decode(log) {
                erc721_transfers.push(Erc721Transfer {
                    block_number: block.number,
                    tx_hash: tx_hash.clone(),
                    log_index: log.index,
                    contract: hex0x(&log.address),
                    from: hex0x(&transfer.from),
                    to: hex0x(&transfer.to),
                    token_id: transfer.token_id.to_string(),
                });
            }
        }
    }

    Ok(Transfers {
        block_number: block.number,
        erc20_transfers,
        erc721_transfers,
    })
}

// Merging the two reports is plain protobuf work, so it belongs here rather than in either RPC
// binary. That does not make the module runnable against an instance missing an endpoint: its
// inputs are produced by the binaries that do need the extensions, and those still fail.
#[substreams::handlers::map]
fn map_eth_methods(
    eth_call: EthCallReport,
    eth_get_balance: EthGetBalanceReport,
) -> Result<EthMethodsReport, Error> {
    Ok(EthMethodsReport {
        block_number: eth_call.block_number.max(eth_get_balance.block_number),
        eth_call: Some(eth_call),
        eth_get_balance: Some(eth_get_balance),
    })
}
