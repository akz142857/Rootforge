# Trigger

Receives and normalizes low-volume incident signals such as Alertmanager
webhooks, Docker OOM events, deployment changes, process exits, and manual test
events. It produces candidate incident events and performs no diagnosis.

The implemented core Event contract validates the minimum incident identity and
scope, normalizes boundary values, and derives an opaque deduplication
fingerprint. Explicit dedupe keys are namespaced by source; otherwise the
fingerprint uses incident type and runtime scope. Transport-specific adapters
remain planned for `alertmanager`, `docker`, `webhook`, and `manual`.
