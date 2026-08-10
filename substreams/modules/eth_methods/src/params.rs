//! Parsing of the `params` input shared by the RPC modules.
//!
//! Parameters are written as a query string, `call_ratio=0.25&call_batch_size=20`. Anything
//! unrecognized or unparseable is an error rather than a silently ignored default: this package
//! exists to diagnose an RPC setup, so a typo that quietly disables the calls would defeat its
//! whole purpose.

/// Block parameter sent along each `eth_getBalance` request.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum BalanceBlock {
    /// The block currently being processed, which requires an archive node when replaying
    /// history but is the only value giving a reproducible answer.
    Number,
    /// The `latest` tag, answered by any node but returning a value unrelated to the block
    /// being processed.
    Latest,
}

#[derive(Debug, Clone, PartialEq)]
pub struct Params {
    /// Number of `eth_call` rounds performed per block, see [`rounds_for`].
    pub call_ratio: f64,
    /// Number of `eth_getBalance` rounds performed per block, see [`rounds_for`].
    pub balance_ratio: f64,
    /// Requests packed in the batched `eth_call` invocation, `0` skips that invocation.
    pub call_batch_size: u32,
    /// Requests packed in the batched `eth_getBalance` invocation, `0` skips that invocation.
    pub balance_batch_size: u32,
    /// Contract `balanceOf(address)` is called on. Left unset, each block resolves the ERC-20
    /// contract that emitted the most `Transfer` events, which keeps the package usable on any
    /// EVM chain instead of only the ones we happened to hardcode an address for.
    pub call_to: Option<Vec<u8>>,
    pub balance_block: BalanceBlock,
}

impl Default for Params {
    fn default() -> Self {
        Self {
            call_ratio: 1.0,
            balance_ratio: 1.0,
            call_batch_size: 10,
            balance_batch_size: 10,
            call_to: None,
            balance_block: BalanceBlock::Number,
        }
    }
}

impl Params {
    pub fn parse(raw: &str) -> Result<Self, String> {
        let mut params = Self::default();

        for pair in raw.split('&') {
            let pair = pair.trim();
            if pair.is_empty() {
                continue;
            }

            let (key, value) = pair
                .split_once('=')
                .ok_or_else(|| format!("parameter {pair:?} is not of the form 'key=value'"))?;
            let (key, value) = (key.trim(), value.trim());

            match key {
                "call_ratio" => params.call_ratio = parse_ratio(key, value)?,
                "balance_ratio" => params.balance_ratio = parse_ratio(key, value)?,
                "call_batch_size" => params.call_batch_size = parse_u32(key, value)?,
                "balance_batch_size" => params.balance_batch_size = parse_u32(key, value)?,
                "call_to" => params.call_to = parse_address(key, value)?,
                "balance_block" => params.balance_block = parse_balance_block(key, value)?,
                _ => return Err(format!("unknown parameter {key:?}")),
            }
        }

        Ok(params)
    }
}

fn parse_ratio(key: &str, value: &str) -> Result<f64, String> {
    let ratio: f64 = value
        .parse()
        .map_err(|_| format!("parameter {key:?} expects a number, got {value:?}"))?;

    if !ratio.is_finite() || ratio < 0.0 {
        return Err(format!(
            "parameter {key:?} expects a finite positive number, got {value:?}"
        ));
    }

    Ok(ratio)
}

fn parse_u32(key: &str, value: &str) -> Result<u32, String> {
    value
        .parse()
        .map_err(|_| format!("parameter {key:?} expects a positive integer, got {value:?}"))
}

fn parse_address(key: &str, value: &str) -> Result<Option<Vec<u8>>, String> {
    if value.is_empty() {
        return Ok(None);
    }

    let bytes = hex::decode(value.strip_prefix("0x").unwrap_or(value))
        .map_err(|_| format!("parameter {key:?} expects a hexadecimal address, got {value:?}"))?;

    if bytes.len() != 20 {
        return Err(format!(
            "parameter {key:?} expects a 20 bytes address, got {} bytes",
            bytes.len()
        ));
    }

    Ok(Some(bytes))
}

fn parse_balance_block(key: &str, value: &str) -> Result<BalanceBlock, String> {
    match value {
        "number" => Ok(BalanceBlock::Number),
        "latest" => Ok(BalanceBlock::Latest),
        _ => Err(format!(
            "parameter {key:?} expects 'number' or 'latest', got {value:?}"
        )),
    }
}

/// How many rounds the given block performs for a ratio.
///
/// A ratio of 1 or more is a count of rounds per block, `2` runs two of them on every block. A
/// ratio below 1 is the inverse of a period instead, `0.25` runs a single round every 4 blocks.
/// The selection is a plain modulo on the block number so that it stays identical no matter how
/// the range gets split across parallel workers.
pub fn rounds_for(ratio: f64, block_number: u64) -> u32 {
    if ratio <= 0.0 {
        return 0;
    }

    if ratio >= 1.0 {
        return ratio.floor() as u32;
    }

    let period = (1.0 / ratio).round().max(1.0) as u64;
    if block_number.is_multiple_of(period) {
        1
    } else {
        0
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parse_empty_yields_defaults() {
        assert_eq!(Params::parse("").unwrap(), Params::default());
        assert_eq!(Params::parse("   ").unwrap(), Params::default());
    }

    #[test]
    fn parse_defaults_hit_one_call_and_one_batch_per_block() {
        let params = Params::default();

        assert_eq!(rounds_for(params.call_ratio, 12345), 1);
        assert_eq!(rounds_for(params.balance_ratio, 12345), 1);
        assert!(params.call_batch_size > 1);
        assert!(params.balance_batch_size > 1);
    }

    #[test]
    fn parse_all_parameters() {
        let params = Params::parse(
            "call_ratio=0.25&balance_ratio=2&call_batch_size=20&balance_batch_size=0\
             &call_to=0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48&balance_block=latest",
        )
        .unwrap();

        assert_eq!(params.call_ratio, 0.25);
        assert_eq!(params.balance_ratio, 2.0);
        assert_eq!(params.call_batch_size, 20);
        assert_eq!(params.balance_batch_size, 0);
        assert_eq!(
            params.call_to,
            Some(hex::decode("a0b86991c6218b36c1d19d4a2e9eb0ce3606eb48").unwrap())
        );
        assert_eq!(params.balance_block, BalanceBlock::Latest);
    }

    #[test]
    fn parse_address_accepts_bare_hex_and_empty() {
        assert_eq!(
            Params::parse("call_to=a0b86991c6218b36c1d19d4a2e9eb0ce3606eb48")
                .unwrap()
                .call_to,
            Some(hex::decode("a0b86991c6218b36c1d19d4a2e9eb0ce3606eb48").unwrap())
        );
        assert_eq!(Params::parse("call_to=").unwrap().call_to, None);
    }

    #[test]
    fn parse_rejects_invalid_input() {
        for raw in [
            "unknown=1",
            "call_ratio",
            "call_ratio=nope",
            "call_ratio=-1",
            "call_batch_size=-1",
            "call_to=0x1234",
            "call_to=zzzz",
            "balance_block=pending",
        ] {
            assert!(
                Params::parse(raw).is_err(),
                "expected {raw:?} to be rejected"
            );
        }
    }

    #[test]
    fn rounds_for_ratio_at_or_above_one_runs_every_block() {
        for block_number in [0, 1, 2, 3, 100] {
            assert_eq!(rounds_for(1.0, block_number), 1);
            assert_eq!(rounds_for(2.0, block_number), 2);
            assert_eq!(rounds_for(2.5, block_number), 2);
        }
    }

    #[test]
    fn rounds_for_ratio_below_one_runs_one_block_out_of_the_period() {
        assert_eq!(rounds_for(0.25, 0), 1);
        assert_eq!(rounds_for(0.25, 1), 0);
        assert_eq!(rounds_for(0.25, 2), 0);
        assert_eq!(rounds_for(0.25, 3), 0);
        assert_eq!(rounds_for(0.25, 4), 1);

        assert_eq!(rounds_for(0.5, 8), 1);
        assert_eq!(rounds_for(0.5, 9), 0);
    }

    #[test]
    fn rounds_for_zero_ratio_disables_the_method() {
        for block_number in [0, 1, 7] {
            assert_eq!(rounds_for(0.0, block_number), 0);
        }
    }
}
