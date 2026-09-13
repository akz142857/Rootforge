# rootforged

Long-running Rootforge control plane. It will receive incident events, own the
Incident Controller, schedule runs through the ClayHarness App Server, persist
Cases, enforce policy, and dispatch notifications.

The current executable exposes incident intake and Case inspection over HTTP:

```bash
go run ./cmd/rootforged -listen 127.0.0.1:8080
```

It currently uses process-local memory storage. Restarting the process removes
all Cases; this adapter is for development and contract testing only.
