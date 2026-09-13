# Rootforge Architecture

Rootforge is an unattended, event-driven incident investigation and remediation
system. An incident event starts the workflow; a human does not need to type an
`ask` command before an investigation can begin.

This document defines the repository boundaries. It intentionally contains no
implementation design beyond what is needed to keep packages separated.

## Runtime shape

```text
Production and external systems
        |
        v
Trigger Intake
        |
        v
Incident Controller -----> Case & Evidence Store
        |                           ^
        v                           |
ClayHarness App Server ----> Tool Gateway
 (independent service)      (Rootforge-owned)
                                  |
                    +-------------+-------------+
                    |             |             |
                 Evidence      Analyzers    Correlation
                    |             |             |
                    +-------------+-------------+
                                  |
                                  v
                              RCA / Finding
                                  |
                                  v
                                Forge
                                  |
                                  v
                         Policy & Action Executor
                           |                 |
                      safe action       notify developer
```

## Process boundaries

The initial repository reserves three executable boundaries:

- `rootforge`: operator-facing CLI for configuration, inspection, replay, and
  explicit administrative actions. It is not the normal incident trigger.
- `rootforged`: long-running control plane that receives events, owns incident
  state, schedules ClayHarness runs, and coordinates Forge and notifications.
- `rootforge-retriever`: optional low-privilege component close to a workload. It
  exposes only predefined, bounded, read-only evidence operations.

These may run in one process during early development. Their permissions and
contracts must remain distinct even when deployed together.

## Package boundaries

| Package area | Owns | Must not own |
| --- | --- | --- |
| `trigger` | Event ingestion and normalization | Root-cause reasoning |
| `incident` | Deduplication, lifecycle, scheduling | Evidence interpretation |
| `casefile` | Case state, evidence lineage, artifacts | Production collection |
| `evidence` | Bounded access to external facts | Long-term telemetry storage |
| `analyzer` | Repeatable deterministic findings | Open-ended agent reasoning |
| `correlation` | Runtime-to-code graph and timeline | Final causal claims |
| `harness` | Rootforge-to-ClayHarness adaptation, schemas, and result mapping | Generic Agent runtime or a second Case state |
| `tool` | Schemas, registry, guarded execution | Business decisions |
| `llm` | Model-provider boundary | Incident workflow |
| `forge` | Candidate change and isolated verification | Unapproved production mutation |
| `policy` | Authorization and automation decisions | Tool implementation |
| `action` | Approved action execution and rollback hooks | Deciding its own permission |
| `notification` | Developer escalation and delivery | Selecting a root cause |
| `audit` | Immutable activity records | Mutable Case state |
| `storage` | Persistence adapters | Domain policy |

## ClayHarness boundary

Rootforge connects to the independently developed and versioned ClayHarness App
Server through its public protocol, normally via the generated Go SDK.
ClayHarness owns the generic Agent loop, budgets, events, and runtime
checkpoints. Rootforge owns incident semantics, Case state, artifacts, tools,
evidence, authorization, actions, and audit history.

Rootforge maps an Incident Case into application-neutral ClayHarness contracts:

```text
Incident Case                    -> RunRequest
Evidence / Finding / ContextGraph -> ArtifactRef
Rootforge Tool Registry           -> ToolExecutor
RunResult + output schema         -> RCA / Escalation
Harness Events                    -> Case history / Audit
```

The delegated loop is conceptually:

```text
observe -> hypothesize -> plan -> call tool -> record evidence -> evaluate
        -> conclude, request more evidence, or escalate
```

Every material conclusion must reference evidence. ClayHarness can request only
tools described for the run. The Rootforge Tool Gateway independently checks
the Case scope and policy, executes a bounded read-only operation, persists its
evidence and audit record, and only then returns a structured result.

ClayHarness never receives Docker, GitHub, production database, cloud-platform,
or arbitrary shell credentials. A requested tool call is not authorization.
ClayHarness can express a proposed action in schema-constrained output, but only
Rootforge Policy can authorize it and only the Action Executor can perform it.

Rootforge's Case is always the source of truth for incident facts and audit
history. ClayHarness checkpoints contain only resumable runtime state and must
not become a parallel investigation record.

ClayHarness v0.1 is a separate Rust repository and independent service.
Rootforge pins a compatible App Server Protocol and Go SDK version, and gives
each investigation a delegated Execution Host profile. Local development may
supervise the App Server as a child process; production deployments keep the
service lifecycle and failure domain separate from `rootforged`.

## Automation levels

Rootforge separates unattended investigation from unattended mutation:

1. `observe`: detect, investigate, and notify.
2. `recommend`: add RCA and remediation guidance.
3. `prepare`: create and verify a candidate patch or pull request.
4. `remediate`: execute explicitly allowlisted, reversible actions.
5. `autonomous`: repair, verify, and roll back within a declared policy.

The default is deny for all write actions. Deployments choose their allowed
automation level per environment and action type.

## Repository map

```text
rootforge/
├── api/v1alpha1/               External API and event schemas
├── cmd/
│   ├── rootforge/              Administrative CLI
│   ├── rootforged/             Control-plane daemon
│   └── rootforge-retriever/    Optional read-only node retriever
├── configs/examples/           Sanitized configuration examples
├── deployments/                Deployment packaging
├── docs/                       Product and architecture decisions
├── heapdump/hprofx/            Existing standalone HPROF analyzer module
├── internal/
│   ├── action/                 Approved action execution
│   ├── analyzer/               Deterministic analyzers
│   ├── audit/                  Audit trail
│   ├── casefile/               Incident Case and evidence ledger
│   ├── config/                 Configuration loading and validation
│   ├── correlation/            Runtime-to-code graph and timeline
│   ├── evidence/               Bounded evidence acquisition
│   ├── forge/                  Patch generation and verification orchestration
│   ├── harness/                ClayHarness adapter and domain schema mapping
│   ├── incident/               Incident lifecycle controller
│   ├── llm/                    Model-provider adapters
│   ├── notification/           Human escalation
│   ├── policy/                 Permission and automation gates
│   ├── storage/                Persistence adapters
│   ├── tool/                   Agent tool contracts and guarded runner
│   ├── transport/httpapi/      Versioned control-plane HTTP transport
│   └── trigger/                Incident event adapters
└── testdata/cases/             Sanitized, reproducible incident fixtures
```

The detailed product and layer contracts remain in `docs/`.

## Implemented first slice

The current control plane implements one narrow path:

```text
POST /api/v1alpha1/events
  -> normalize and validate Event
  -> derive an opaque incident fingerprint
  -> atomically create or update an open in-memory Case
  -> GET /api/v1alpha1/cases/{caseID}
```

The in-memory Store is a development adapter, not a production persistence
decision. ClayHarness dispatch, evidence acquisition, lifecycle transitions,
and durable storage remain outside this slice.
