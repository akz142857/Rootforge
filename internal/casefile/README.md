# Casefile

Owns the Incident Case: incident metadata, evidence ledger, derived findings,
hypotheses, investigation runs, RCA versions, proposals, approvals, actions, and
notifications.

Raw facts, deterministic findings, and model inferences remain distinguishable.
Existing records are versioned or appended rather than silently overwritten.

The first slice implements atomic event application, revision increments, and a
concurrency-safe in-memory Store. The memory adapter deliberately makes no
durability claim; a persistent Store must preserve the same atomic
deduplication contract.
