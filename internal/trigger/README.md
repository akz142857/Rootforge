# Trigger

Receives and normalizes low-volume incident signals such as Alertmanager
webhooks, Docker OOM events, deployment changes, process exits, and manual test
events. It produces candidate incident events and performs no diagnosis.

Planned adapters: `alertmanager`, `docker`, `webhook`, and `manual`.
