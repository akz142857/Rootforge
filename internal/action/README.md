# Action executor

Executes an action only after receiving an explicit policy decision. It will own
idempotency, preconditions, dry runs, bounded credentials, verification,
rollback hooks, status reporting, and immutable audit events.

ClayHarness and Forge cannot call infrastructure mutation APIs directly.
