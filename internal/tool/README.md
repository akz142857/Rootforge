# Tool gateway

Defines Agent-visible tool schemas, registration, Case-scoped authorization,
guarded execution, result normalization, budgets, timeouts, evidence persistence,
and audit hooks. It implements ClayHarness's `ToolExecutor` boundary.

Every call requested by ClayHarness is untrusted. The gateway checks the run and
Case scope and policy again, executes only the named bounded operation, saves
the resulting Evidence before reporting success, and returns a structured result
with stable artifact references.

Tools expose narrow domain operations such as `logs.search`,
`runtime.inspect_container`, `heapdump.analyze`, or `git.inspect_commit`. An
unrestricted `run_shell` tool is outside the production trust boundary.
