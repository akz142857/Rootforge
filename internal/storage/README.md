# Storage

Persistence adapters for Case metadata, evidence indexes, artifact references,
workflow state, audit records, and leases.

`internal/storage/local` currently provides atomically replaced JSON snapshots
for Cases and the investigation Outbox. Files are created with owner-only
permissions and validated when reopened. It is a single-process v0.1 adapter,
not a distributed storage decision.

Rootforge storage is not a general log, metrics, or trace database.
