# Tool gateway

Defines Agent-visible tool schemas, registration, Case-scoped authorization,
guarded execution, result normalization, budgets, timeouts, and audit hooks.

Tools expose narrow domain operations such as `logs.search`,
`runtime.inspect_container`, `heapdump.analyze`, or `git.inspect_commit`. An
unrestricted `run_shell` tool is outside the production trust boundary.
