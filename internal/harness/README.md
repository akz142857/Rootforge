# ClayHarness adapter

This package adapts Rootforge domain state to the independently versioned
ClayHarness App Server Protocol, normally through its generated Go SDK. It does
not embed or implement a second Agent runtime.

Its responsibilities are deliberately narrow:

```text
Incident Case                     -> Run request
Evidence / Finding / ContextGraph -> Artifact references
Rootforge Tool Gateway            -> Delegated Tool Bridge
ClayHarness Run result            -> RCA / Escalation
ClayHarness Event                 -> Case history / Audit
```

Rootforge supplies the output schema that gives the generic run its RCA and
escalation semantics. No Rootforge domain type belongs in ClayHarness's public
API.

The adapter never treats a ClayHarness checkpoint as authoritative Case state.
It persists runtime events and checkpoints through Rootforge, while the Case
remains the source of truth. Tool calls always return through the Rootforge Tool
Gateway for Case-scope checks, policy checks, bounded execution, evidence
persistence, and audit.
