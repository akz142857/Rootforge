# rootforged

Long-running Rootforge control plane. It will receive incident events, own the
Incident Controller, schedule runs through the ClayHarness App Server, persist
Cases, enforce policy, and dispatch notifications.

The current executable exposes incident intake and Case inspection over HTTP:

```bash
go run ./cmd/rootforged \
  -listen 127.0.0.1:8080 \
  -data-dir .rootforge/data
```

It currently persists Case and investigation Outbox snapshots in the selected
directory and reconciles pending Cases into the Outbox at startup and every 30
seconds by default. Use `-reconcile-interval` to change that recovery interval.
The local adapter supports only one process; distributed deployment and
ClayHarness task dispatch are not implemented yet.
