//! Selection of the accounts an RPC binary sends its requests for.

use substreams_ethereum::pb::eth::v2 as eth;

use crate::decode_hex_address;
use crate::pb::sf::ethereum::substreams::ethmethods::v1::Transfers;

/// Every distinct address the block touches, senders and recipients of both transactions and
/// decoded transfers, plus the fee recipient.
pub fn block_accounts(block: &eth::Block, transfers: &Transfers) -> Vec<Vec<u8>> {
    let mut accounts = AddressSet::new();

    for trx in block.transactions() {
        accounts.insert(&trx.from);
        accounts.insert(&trx.to);
    }

    for transfer in &transfers.erc20_transfers {
        accounts.insert_hex(&transfer.from);
        accounts.insert_hex(&transfer.to);
    }

    for transfer in &transfers.erc721_transfers {
        accounts.insert_hex(&transfer.from);
        accounts.insert_hex(&transfer.to);
    }

    // Last, so that a block holding real activity queries that instead. It is however the one
    // address an empty block still carries, and it is what keeps a chain producing empty blocks,
    // which any testnet does, from reporting that it has nothing to query.
    if let Some(header) = &block.header {
        accounts.insert(&header.coinbase);
    }

    accounts.into_vec()
}

/// Deduplicates addresses while preserving the order they were first seen in, so that the
/// requests a block issues stay identical across runs and across workers.
pub struct AddressSet {
    seen: Vec<Vec<u8>>,
}

impl AddressSet {
    pub fn new() -> Self {
        Self { seen: Vec::new() }
    }

    pub fn insert(&mut self, address: &[u8]) {
        // Contract creations carry an empty `to`, and the zero address is never worth querying.
        if address.len() != 20 || address.iter().all(|byte| *byte == 0) {
            return;
        }

        if !self.seen.iter().any(|candidate| candidate == address) {
            self.seen.push(address.to_vec());
        }
    }

    pub fn insert_hex(&mut self, address: &str) {
        if let Some(address) = decode_hex_address(address) {
            self.insert(&address);
        }
    }

    pub fn is_empty(&self) -> bool {
        self.seen.is_empty()
    }

    pub fn into_vec(self) -> Vec<Vec<u8>> {
        self.seen
    }
}

impl Default for AddressSet {
    fn default() -> Self {
        Self::new()
    }
}

/// Rotates over a list of addresses so that successive invocations of a block query different
/// accounts, wrapping around once the list is exhausted.
pub struct Cursor {
    accounts: Vec<Vec<u8>>,
    next: usize,
}

impl Cursor {
    pub fn new(accounts: Vec<Vec<u8>>) -> Self {
        Self { accounts, next: 0 }
    }

    pub fn take(&mut self, count: usize) -> Vec<Vec<u8>> {
        let mut taken = Vec::with_capacity(count);
        for _ in 0..count {
            taken.push(self.accounts[self.next].clone());
            self.next = (self.next + 1) % self.accounts.len();
        }

        taken
    }
}
