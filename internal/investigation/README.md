# Investigation scheduling

Owns the durable Outbox that bridges authoritative Rootforge Cases to eventual
ClayHarness dispatch. Intake persists the Case before ensuring one pending task
for its latest revision. Startup reconciliation repairs the narrow crash window
between those two durable writes.

Workers lease tasks with an expiring token. If a newer Case revision arrives
while a task is leased, completing the older revision returns the task to
`pending` instead of dropping the newer work. The Outbox is delivery state, not
the authoritative investigation history; Run results and events belong in the
Case and audit ledger.
