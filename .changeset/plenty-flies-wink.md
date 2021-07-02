---
'@eth-optimism/contracts': patch
---

Ensure that within a call to `appendSequencerBatch()` sequencer transactions must have monotonically increasing context relative to previous queue transactions.
