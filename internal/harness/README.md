# Harness Agent

This is Rootforge's own evidence-driven Agent runtime. It receives a scoped
Incident Case, constructs and tests hypotheses, selects registered tools, asks
for additional evidence when useful, and produces a versioned conclusion or
developer escalation.

Planned internal boundaries:

```text
harness/
├── runtime       Investigation loop and termination conditions
├── planner       Next-step and information-gain planning
├── hypothesis    Candidate causes, support, contradiction, confidence
├── context       Bounded model context assembled from the Case
├── budget        Step, token, time, query, and cost limits
├── evaluator     Evidence sufficiency and conclusion checks
└── conclusion    Structured RCA or escalation result
```

The Harness Agent never receives direct infrastructure credentials or arbitrary
shell access. It can call only tools authorized for the current Case. It may
propose actions, but it cannot authorize or execute them.
