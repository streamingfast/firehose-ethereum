//! Handler exercising the `rpc::eth_call` extension, which `fireeth` registers on
//! `--substreams-rpc-endpoints`.
//!
//! It is alone in this binary so that `rpc::eth_call` is the only extension imported here, leaving
//! the module runnable against an instance configured with that flag and nothing else.

// The handler macro expands a module taking a `params` input into an `extern "C"` wrapper reading
// the string out of a raw pointer. Clippy reports that on the handler we wrote rather than on the
// generated wrapper, and the macro drops any attribute we would put on the handler to silence it.
#![allow(clippy::not_unsafe_ptr_arg_deref)]

use substreams::errors::Error;
use substreams::scalar::BigInt;
use substreams_ethereum::pb::eth::v2 as eth;
use substreams_ethereum::rpc::RpcBatch;

use eth_methods::accounts::{block_accounts, AddressSet, Cursor};
use eth_methods::params::{self, Params};
use eth_methods::pb::sf::ethereum::substreams::ethmethods::v1::{
    CallTargetSource, EthCallReport, InvocationKind, ReportStatus, RpcInvocation, RpcResult,
    Transfers,
};
use eth_methods::report::{failed_result, invocation_of, stats_of};
use eth_methods::{abi, decode_hex_address, hex0x};

// See `src/bin/no_rpc.rs`.
fn main() {}

#[substreams::handlers::map]
fn map_eth_call(
    raw_params: String,
    block: eth::Block,
    transfers: Transfers,
) -> Result<EthCallReport, Error> {
    let params = Params::parse(&raw_params).map_err(Error::msg)?;

    let rounds = params::rounds_for(params.call_ratio, block.number);
    if rounds == 0 {
        return Ok(EthCallReport {
            block_number: block.number,
            status: ReportStatus::SkippedByRatio as i32,
            ..Default::default()
        });
    }

    let Some((contract, target_source)) = resolve_call_target(&params, &block, &transfers) else {
        return Ok(EthCallReport {
            block_number: block.number,
            status: ReportStatus::InsufficientBlockData as i32,
            hints: vec![
                "this block carries no ERC-20 transfer and no transaction calling a contract, \
                 so there is nothing to run 'balanceOf' against. Expected on a quiet chain, a \
                 testnet in particular: widen the range, or set the 'call_to' parameter to a \
                 contract you know is deployed on this network."
                    .to_string(),
            ],
            ..Default::default()
        });
    };

    let mut hints = Vec::new();
    let accounts = match call_accounts(&contract, &transfers) {
        Some(accounts) => accounts,
        None => {
            // Reached when the target holds no transfer in this block, which is the norm as soon
            // as `call_to` pins a contract quieter than the chain it sits on. Querying unrelated
            // accounts still exercises the extension, it just answers zero.
            hints.push(format!(
                "{} emitted no transfer in this block, so the accounts queried below are not \
                 holders and are expected to answer a zero balance. That is a property of the \
                 block, not a broken endpoint.",
                hex0x(&contract)
            ));
            block_accounts(&block, &transfers)
        }
    };

    if accounts.is_empty() {
        return Ok(EthCallReport {
            block_number: block.number,
            status: ReportStatus::InsufficientBlockData as i32,
            hints: vec![
                "this block carries no address at all to call 'balanceOf' with. Expected on a \
                 quiet chain, a testnet in particular: widen the range."
                    .to_string(),
            ],
            target_contract: hex0x(&contract),
            target_source: target_source as i32,
            ..Default::default()
        });
    }

    if target_source == CallTargetSource::CalledContract {
        hints.push(format!(
            "this block carries no ERC-20 transfer, so 'balanceOf' is run against {}, an address \
             it called with a payload and which therefore holds code. An empty response is the \
             expected answer when that contract is not a token, and does not indicate a broken \
             endpoint.",
            hex0x(&contract)
        ));
    }

    let mut cursor = Cursor::new(accounts);
    let mut invocations = Vec::new();

    for round in 0..rounds {
        // One single-request invocation and one batched invocation per round: they travel
        // through the same host function but only the second one exercises the batching of the
        // underlying JSON-RPC transport.
        invocations.push(eth_call_invocation(
            InvocationKind::Single,
            round,
            &contract,
            &cursor.take(1),
        ));

        if params.call_batch_size > 0 {
            invocations.push(eth_call_invocation(
                InvocationKind::Batch,
                round,
                &contract,
                &cursor.take(params.call_batch_size as usize),
            ));
        }
    }

    let stats = stats_of(&invocations);
    if stats.request_count > 0 && stats.request_count == stats.failure_count {
        hints.push(format!(
            "every request failed. Check that the serving instance was started with \
             --substreams-rpc-endpoints and that the endpoint answers 'eth_call' at {}.",
            hex0x(&contract)
        ));
    } else if invocations
        .iter()
        .flat_map(|invocation| &invocation.results)
        .all(|result| !result.failed && result.value.is_empty())
    {
        hints.push(format!(
            "every request succeeded but answered an empty payload, which is what an address \
             holding no code returns. {} is most likely not deployed on this network: set \
             'call_to' to a token that exists here.",
            hex0x(&contract)
        ));
    }

    Ok(EthCallReport {
        block_number: block.number,
        status: ReportStatus::Performed as i32,
        hints,
        target_contract: hex0x(&contract),
        target_source: target_source as i32,
        stats: Some(stats),
        invocations,
    })
}

fn eth_call_invocation(
    kind: InvocationKind,
    round: u32,
    contract: &[u8],
    accounts: &[Vec<u8>],
) -> RpcInvocation {
    let mut batch = RpcBatch::new();
    for account in accounts {
        batch = batch.add(
            abi::erc20::functions::BalanceOf {
                account: account.clone(),
            },
            contract.to_vec(),
        );
    }

    let responses = match batch.execute() {
        Ok(responses) => responses.responses,
        // The host function itself failing is a setup problem rather than a per-request one,
        // reporting every request as failed keeps the counters honest.
        Err(err) => {
            substreams::log::info!("eth_call invocation failed: {}", err);
            Vec::new()
        }
    };

    let results = accounts
        .iter()
        .enumerate()
        .map(|(index, account)| match responses.get(index) {
            None => failed_result(account),
            Some(response) => RpcResult {
                address: hex0x(account),
                failed: response.failed,
                value: RpcBatch::decode::<_, abi::erc20::functions::BalanceOf>(response)
                    .map(|balance: BigInt| balance.to_string())
                    .unwrap_or_default(),
                raw: hex0x(&response.raw),
            },
        })
        .collect();

    invocation_of(kind, round, results)
}

/// Contract `balanceOf(address)` is called on, and where it was found.
///
/// Resolving out of the block, rather than out of a hardcoded token address, is what lets the
/// package run unchanged against any EVM chain: a pinned mainnet address resolves to an account
/// holding no code elsewhere, where every call comes back empty and the package reports success
/// while testing nothing.
///
/// The resolution degrades in three steps so that a chain too quiet to hold a token transfer in
/// every block, which any testnet is, still gets its extension exercised. The step that was
/// reached is reported so a caller can tell a call into a real token apart from a call into some
/// contract that merely happened to be there.
fn resolve_call_target(
    params: &Params,
    block: &eth::Block,
    transfers: &Transfers,
) -> Option<(Vec<u8>, CallTargetSource)> {
    if let Some(call_to) = &params.call_to {
        return Some((call_to.clone(), CallTargetSource::Parameter));
    }

    if let Some(contract) = busiest_erc20(transfers) {
        return Some((contract, CallTargetSource::BusiestErc20));
    }

    // An address a transaction carried a payload to holds code, which is all `balanceOf` needs to
    // reach the endpoint. It answers empty unless the contract happens to be a token, and that is
    // still a round trip through the extension, which is what is being tested.
    block
        .transactions()
        .find(|trx| !trx.input.is_empty() && trx.to.len() == 20)
        .map(|trx| (trx.to.clone(), CallTargetSource::CalledContract))
}

/// ERC-20 contract that emitted the most `Transfer` events in the block, the accounts seen
/// transferring it being holders whose balance comes back non-zero.
fn busiest_erc20(transfers: &Transfers) -> Option<Vec<u8>> {
    let mut tally: Vec<(&str, usize)> = Vec::new();
    for transfer in &transfers.erc20_transfers {
        match tally
            .iter_mut()
            .find(|(contract, _)| *contract == transfer.contract)
        {
            Some((_, count)) => *count += 1,
            None => tally.push((transfer.contract.as_str(), 1)),
        }
    }

    // Ties are broken on the address itself, leaving the choice independent from the order the
    // block happens to be iterated in.
    tally
        .into_iter()
        .max_by(|(left_address, left), (right_address, right)| {
            left.cmp(right).then(right_address.cmp(left_address))
        })
        .and_then(|(address, _)| decode_hex_address(address))
}

/// Holders of the target token seen in this block, whose balance comes back non-zero and so
/// tells a working call apart from one answering an all-zeroes payload.
///
/// `None` when the target moved nothing in this block, leaving the caller to fall back on the
/// block's own addresses and to say so in its report.
fn call_accounts(contract: &[u8], transfers: &Transfers) -> Option<Vec<Vec<u8>>> {
    let contract = hex0x(contract);

    let mut accounts = AddressSet::new();
    for transfer in &transfers.erc20_transfers {
        if transfer.contract == contract {
            accounts.insert_hex(&transfer.from);
            accounts.insert_hex(&transfer.to);
        }
    }

    if accounts.is_empty() {
        return None;
    }

    Some(accounts.into_vec())
}
