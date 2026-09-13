# LLM

Rootforge-side model policy and provider constraints supplied when requesting a
ClayHarness Run and used by Forge. ClayHarness owns provider integration and its
credentials; Rootforge owns which data may leave its boundary, which models are
permitted for a Case, and the applicable cost policy. Incident orchestration
does not belong here.

Generic model interaction and tool-call orchestration belong to ClayHarness.
Rootforge must not duplicate that runtime loop in this package.

No provider is selected by the repository scaffold.
