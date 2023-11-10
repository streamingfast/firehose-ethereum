# ERRATA

* This file lists past data consistency issues and how it affects users of firehose and substreams

## Firehose Polygon and Mumbai blocks were missing some StateSync transactions.

### Overview

* A bug in the firehose-ethereum implementation caused some "StateSync" transactions (polygon bridge deposits) to be missing. Their logs were, instead, shown under different generated transaction hashes, with the wrong log number.
* The bug was fixed in [firehose-ethereum release v1.4.16](https://github.com/streamingfast/firehose-ethereum/releases/tag/v1.4.16) (released on September 29th, 2023)
* The Polygon blocks were fully reprocessed on our StreamingFast's `polygon.streamingfast.io:443` endpoint on Nov. 8th, 2023

### How does it impact you ?

* If you are a firehose operator, you will need to reprocess the whole chain to replace your merged-blocks-files.
* If you are consuming a Substreams:
  1. **determine if that Substreams is affected by the existence or the order of these StateSync transactions** (ex: `0x8f3e05c5af7d601a6015c4fdbb68a04cbf0305fe2740920ce70f81ed1943a194`)
  2. **evaluate if the discrepancies in past blocks are impactful to your business need**
* If both previous statements apply to you:
  1. Make sure that the Substreams provider that you query has already upgraded his Polygon blocks
  2. Make a small change to your Substreams code (anything will do), recompile and repackage your .spkg: this will cause the module hashes to change and effectively invalidate the cache.
* We will not delete previous Substreams "caches" that were generated with the faulty blocks. If you believe some widely-used Substreams is affected and should have its cache pruned, you can [contact us on Discord](https://discord.gg/jZwqxJAvRs).

### More context...

* Explanation of Polygon state-sync events: https://wiki.polygon.technology/docs/pos/design/bridge/state-sync/how-state-sync-works/

* How Firehose handle these:
  * a) Polygon transactions are done in parallel in the `bor` client, so they must be reordered after the fact (using special code in the `firehose reader` that mimics the `bor` logic)
  * b) The polygon StateSync events are not handled like normal transactions: A "virtual" transaction is created at the end of the block, with its hash being computed as `Keccak256("matic-bor-receipt-" + BlockNumber + BlockHash)`.
  * c) There are other "system events" that are emitted from the firehose-instrumented `bor` client, which are NOT bundled as part of the state-sync transaction. These are the ones that happen every 6400 block sand involve the polygon Validator Contract `0x0...1000` They are not shown in rpc get_block or get_transaction but they do affect the chain state, so they are in the firehose blocks.

* There were three issues in our polygon-specific implementation:
  1. The reordering of transactions (a) was done BEFORE the polygon system transactions were bundled together, but we only checked the very last transaction in the block to see if it was a StateSync event. When there was an event to the Polygon Validator Contract `0x0...1000`, we the transaction "bundling" was not triggered.
  2. When generating the receipt logs for the "virtual system transaction", we were using the Ordinals as a reference, like we do for every other transaction. However, the way that polygon reconstructs this header in the getLogs() rpc call is different, so we must fullow the original "BlockIndex" from the reader instead of using the Ordinal.
  3. The "other system events" (c) would skew the other transactions index number, since the RPC endpoint does not show them. Moving that special system transaction to the end solved this.

* See the following issue for more details: https://github.com/streamingfast/firehose/issues/25

### Example impacted transactions:

* https://polygonscan.com/tx/0x8f3e05c5af7d601a6015c4fdbb68a04cbf0305fe2740920ce70f81ed1943a194
* https://polygonscan.com/tx/0x7ed01e15e7282696cf3dc73268ef2c84a46203bb54fed6f0451d4a0eb5e5cbd5
* https://polygonscan.com/tx/0x33315f4c921b9c1321448bcc23806742b7aee9081b5c7c96bdd8a097082a61de
