//! Handler exercising the `rpc::eth_get_balance` extension, which `fireeth` registers on
//! `--substreams-rpc-get-balance-endpoints`.
//!
//! It is alone in this binary so that `rpc::eth_get_balance` is the only extension imported here.
//! That extension is also the more recently added of the two, so an instance predating it is the
//! likeliest one to be missing it while answering `eth_call` just fine.

// See `src/bin/eth_call.rs`.
#![allow(clippy::not_unsafe_ptr_arg_deref)]

use substreams::errors::Error;
use substreams::scalar::BigInt;
use substreams_ethereum::pb::eth::rpc::{RpcGetBalanceRequest, RpcGetBalanceRequests};
use substreams_ethereum::pb::eth::v2 as eth;

use eth_methods::accounts::{block_accounts, Cursor};
use eth_methods::hex0x;
use eth_methods::params::{self, BalanceBlock, Params};
use eth_methods::pb::sf::ethereum::substreams::ethmethods::v1::{
    EthGetBalanceReport, InvocationKind, ReportStatus, RpcInvocation, RpcResult, Transfers,
};
use eth_methods::report::{failed_result, invocation_of, stats_of};

// See `src/bin/no_rpc.rs`.
fn main() {}

#[substreams::handlers::map]
fn map_eth_get_balance(
    raw_params: String,
    block: eth::Block,
    transfers: Transfers,
) -> Result<EthGetBalanceReport, Error> {
    let params = Params::parse(&raw_params).map_err(Error::msg)?;

    let rounds = params::rounds_for(params.balance_ratio, block.number);
    if rounds == 0 {
        return Ok(EthGetBalanceReport {
            block_number: block.number,
            status: ReportStatus::SkippedByRatio as i32,
            ..Default::default()
        });
    }

    let accounts = block_accounts(&block, &transfers);
    if accounts.is_empty() {
        return Ok(EthGetBalanceReport {
            block_number: block.number,
            status: ReportStatus::InsufficientBlockData as i32,
            hints: vec![
                "this block carries no address at all, not even a fee recipient, so there is no \
                 account to query a balance for. Expected on a quiet chain, a testnet in \
                 particular: widen the range."
                    .to_string(),
            ],
            ..Default::default()
        });
    }

    // Querying the block being processed is the only choice giving a reproducible answer, at the
    // cost of requiring an archive node when replaying history. `balance_block=latest` trades
    // that reproducibility for reaching any node.
    let block_param = match params.balance_block {
        BalanceBlock::Number => format!("0x{:x}", block.number),
        BalanceBlock::Latest => "latest".to_string(),
    };

    let mut cursor = Cursor::new(accounts);
    let mut invocations = Vec::new();

    for round in 0..rounds {
        invocations.push(eth_get_balance_invocation(
            InvocationKind::Single,
            round,
            &block_param,
            &cursor.take(1),
        ));

        if params.balance_batch_size > 0 {
            invocations.push(eth_get_balance_invocation(
                InvocationKind::Batch,
                round,
                &block_param,
                &cursor.take(params.balance_batch_size as usize),
            ));
        }
    }

    let mut hints = Vec::new();
    let stats = stats_of(&invocations);
    if stats.request_count > 0 && stats.request_count == stats.failure_count {
        hints.push(match params.balance_block {
            // An archive node is what querying an old block needs, and a pruned one answering
            // nothing else is by far the likeliest reason for a wholesale failure here.
            BalanceBlock::Number => format!(
                "every request failed while querying block {block_param}. A node pruning its \
                 state cannot answer a balance for anything but recent blocks: retry with \
                 'balance_block=latest', and check that the serving instance was started with \
                 --substreams-rpc-get-balance-endpoints."
            ),
            BalanceBlock::Latest => "every request failed. Check that the serving instance was \
                 started with --substreams-rpc-get-balance-endpoints and that the endpoint \
                 answers 'eth_getBalance'."
                .to_string(),
        });
    }

    Ok(EthGetBalanceReport {
        block_number: block.number,
        status: ReportStatus::Performed as i32,
        hints,
        block_param,
        stats: Some(stats),
        invocations,
    })
}

fn eth_get_balance_invocation(
    kind: InvocationKind,
    round: u32,
    block_param: &str,
    accounts: &[Vec<u8>],
) -> RpcInvocation {
    let requests = RpcGetBalanceRequests {
        requests: accounts
            .iter()
            .map(|account| RpcGetBalanceRequest {
                address: account.clone(),
                block: block_param.to_string(),
            })
            .collect(),
    };

    let responses = substreams_ethereum::rpc::eth_get_balance(&requests).responses;

    let results = accounts
        .iter()
        .enumerate()
        .map(|(index, account)| match responses.get(index) {
            None => failed_result(account),
            Some(response) => RpcResult {
                address: hex0x(account),
                failed: response.failed,
                value: if response.failed {
                    String::new()
                } else {
                    BigInt::from_unsigned_bytes_be(&response.balance).to_string()
                },
                raw: hex0x(&response.balance),
            },
        })
        .collect();

    invocation_of(kind, round, results)
}
