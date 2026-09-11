# Evidence

Provides bounded, Case-scoped access to logs, metrics, runtime events, dumps,
deployment metadata, and code metadata. It queries existing source systems or an
optional restricted retriever; it does not continuously ingest or index all
telemetry.

Planned cross-cutting components include query models, budgets, redaction,
provenance, stable references, truncation reporting, and connector capability
discovery.

Planned sources:

- `source/file`: offline fixtures and imported artifacts
- `source/docker`: Engine inspection, events, and bounded container logs
- `source/journald`: bounded host and kernel journal queries
- `source/loki`: LogQL query and short-lived Case follow
- `source/elasticsearch`: bounded Query DSL search
