//! Tallying of the results an RPC binary collected into the report it emits.

use crate::hex0x;
use crate::pb::sf::ethereum::substreams::ethmethods::v1::{
    InvocationKind, RpcInvocation, RpcResult, RpcStats,
};

pub fn failed_result(account: &[u8]) -> RpcResult {
    RpcResult {
        address: hex0x(account),
        failed: true,
        value: String::new(),
        raw: String::new(),
    }
}

pub fn invocation_of(kind: InvocationKind, round: u32, results: Vec<RpcResult>) -> RpcInvocation {
    let failure_count = results.iter().filter(|result| result.failed).count() as u32;

    RpcInvocation {
        kind: kind as i32,
        round,
        request_count: results.len() as u32,
        success_count: results.len() as u32 - failure_count,
        failure_count,
        results,
    }
}

pub fn stats_of(invocations: &[RpcInvocation]) -> RpcStats {
    RpcStats {
        invocation_count: invocations.len() as u32,
        request_count: invocations.iter().map(|it| it.request_count).sum(),
        success_count: invocations.iter().map(|it| it.success_count).sum(),
        failure_count: invocations.iter().map(|it| it.failure_count).sum(),
    }
}
