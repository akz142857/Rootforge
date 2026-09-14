# Rootforge

> **Find the root. Forge the fix.**

Rootforge is an open-source, unattended incident investigation and remediation
system. It detects production failures, integrates with the independently
developed ClayHarness Agent runtime to trace likely root causes back to source
code, prepares or applies safe remediation according to policy, and notifies
developers when human judgment is required.

An incident event starts the workflow; a developer does not need to ask Rootforge
a question first. Rootforge connects the parts of incident response that are
usually separated:

```text
Production Runtime -> Observability -> Deployment -> Git -> Source Code -> Fix
```

For the product scope, target users, non-goals, and initial success criteria, see
the [Chinese product blueprint](docs/product-blueprint.zh-CN.md) and the
[system layers and boundaries](docs/system-layers.zh-CN.md). The detailed L1
boundary is defined in the [Evidence Acquisition design](docs/evidence-acquisition.zh-CN.md).
The concrete repository map and package dependency rules are in
[ARCHITECTURE.md](ARCHITECTURE.md).

## Why Rootforge?

Monitoring tools tell you that something is wrong. Coding agents can change code.
The difficult part in between is establishing what happened in production, which
version was running, and which code change most likely caused the incident.

Rootforge is designed to bridge that gap.

Given an incident, it should autonomously determine:

- What happened, when, and where?
- Which service, container, and node were affected?
- Which image, build, repository, and Git commit were deployed?
- What runtime evidence supports the diagnosis?
- Which code paths or recent changes are likely responsible?
- What is the safest recommended fix?

## Project status

Rootforge is in an early design and prototyping stage. The first milestone is
intentionally narrow: detect out-of-memory incidents in Docker-based production
environments, investigate them without waiting for a human prompt, generate an
evidence-backed root cause analysis, and notify the responsible developer.

The control plane now accepts normalized incident events over HTTP, validates
their identity and scope, deduplicates repeated signals into an open Incident,
and durably creates or updates the authoritative Case. A persistent Outbox
retains pending ClayHarness investigation work across process restarts.
Evidence acquisition and ClayHarness dispatch are the next slices.

Unattended investigation and unattended mutation are separate capabilities.
Write actions default to denied. Later milestones may prepare a pull request or
execute explicitly allowlisted, reversible remediation when deployment policy
permits it.

Run the development control plane:

```bash
go run ./cmd/rootforged \
  -listen 127.0.0.1:8080 \
  -data-dir .rootforge/data
```

Submit a synthetic OOM event:

```bash
curl -X POST http://127.0.0.1:8080/api/v1alpha1/events \
  -H 'Content-Type: application/json' \
  -d '{
    "event_id": "docker-event-123",
    "type": "container.oom",
    "source": "docker",
    "occurred_at": "2026-09-12T03:00:00Z",
    "environment": "development",
    "service": "example-api",
    "container": "container-123",
    "severity": "critical"
  }'
```

The response contains the Case ID. Cases and pending investigation tasks survive
restart in the selected data directory. The local JSON storage adapter supports
one `rootforged` process and is not a distributed production database.

## v0.1: OOM Investigator

The initial workflow focuses on one concrete incident type:

```text
OOM event received
    |
    v
Create or update Incident Case
    |
    v
Identify node, service, and container
    |
    v
Acquire bounded kernel, runtime, log, and memory evidence
    |
    v
Resolve image and deployment metadata to a Git commit
    |
    v
Correlate memory growth with deployments and recent changes
    |
    v
Inspect relevant source code
    |
    v
Generate an RCA report with confidence and supporting evidence
    |
    v
Prepare an allowed remediation or notify a developer
```

An investigation report should include:

- Incident summary and impact
- Timeline of relevant events
- OOM, container, and host evidence
- Memory behavior before the failure
- Deployment and Git commit correlation
- Ranked root-cause candidates with confidence levels
- Relevant repositories, files, symbols, and code changes
- Recommended fixes, risks, and verification steps

## Architecture direction

Rootforge separates event intake, bounded evidence acquisition, an independent
ClayHarness Agent service, and policy-governed actions. It queries existing
observability systems when available and uses restricted, on-demand node
retrievers otherwise.

```text
+---------------- Production ----------------+
|                                             |
|  Host A       Host B       Docker / Swarm   |
|     \            |              /           |
|   Existing Sources / On-demand Retrievers   |
+---------------------+-----------------------+
                      |
             logs / metrics / events
                      |
                      v
+---------------- Rootforge ------------------+
|  Trigger -> Incident Case -> Harness Adapter|----> ClayHarness App Server
|                 ^                  |         |       (independent service)
|                 |                  |         |
|                 +----- Tool Gateway+<--------|---- delegated tool requests
|                         -> Forge             |
|                         -> Policy / Action   |
+---------------------+-----------------------+
                      |
                      v
          Safe remediation or notification
```

Evidence sources provide facts such as:

- Kernel OOM events and process information
- Docker events, task exits, and container logs
- CPU, memory, disk, and network signals
- Service, image, build, and deployment metadata

ClayHarness performs generic Agent execution and reasoning through Rootforge's
guarded tools. Rootforge remains authoritative for Case state, evidence,
permissions, audit, and actions. Large-model agents do not run with shell access
on production hosts.

## Runtime-to-code mapping

Reliable diagnosis depends on knowing exactly which code produced a running
artifact. Rootforge expects build and deployment metadata similar to:

```text
service=payments-api
repository=github.com/example/payments
commit=7f31a21
build=2026-09-10.12
environment=production
```

This creates a traceable chain:

```text
Incident -> Service -> Container -> Image -> Build -> Git Commit -> Source Code
```

## Roadmap

### Phase 1: Detect, investigate, and notify

- Detect Docker and host OOM incidents
- Acquire a bounded incident evidence package
- Map workloads to repositories and commits
- Correlate incidents with deployments and code changes
- Produce a structured RCA report
- Notify a developer with evidence, confidence, and required next action

### Phase 2: Prepare and verify

- Generate a candidate patch
- Run targeted tests and static checks
- Explain the change, confidence, and risk
- Open a pull request when repository policy permits it

### Phase 3: Policy-governed remediation

- Execute allowlisted, reversible operational actions
- Verify whether the incident is resolved
- Roll back automatically when verification fails
- Escalate with the complete Case when safe automation cannot continue

### Phase 4: Expand incident coverage

- Crash and restart loops
- Sustained CPU saturation
- Disk exhaustion
- HTTP 5xx spikes and timeouts
- Database connection exhaustion
- Memory leaks and performance regressions

## Design principles

- **Evidence before inference.** Every conclusion should link back to observable
  runtime, deployment, or code evidence.
- **Policy-controlled remediation.** Investigation is unattended. Mutations are
  separately authorized by environment, action type, confidence, reversibility,
  and approval policy.
- **Least privilege.** Evidence sources receive only the access needed for a case.
- **Bounded context.** Incident data is scoped by service, host, and time window.
- **Explainable confidence.** Root-cause candidates state both confidence and the
  evidence that raises or lowers it.
- **Open integrations.** Rootforge should complement existing observability and
  coding tools rather than replace them.
- **Agent behind a tool boundary.** ClayHarness has no direct production or
  repository credentials and cannot bypass Rootforge's Tool Gateway, Policy, or
  Action layers.

## Contributing

Rootforge is at the beginning of its journey. Architecture discussions, incident
samples, collector ideas, and implementation contributions are welcome once the
initial project structure and contribution guide are published.

## License

A license has not been selected yet. Until a license file is added, the project is
not licensed for redistribution or modification.

---

**Rootforge — Find the root. Forge the fix.**
