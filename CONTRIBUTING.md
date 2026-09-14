# Contributing to Rootforge

Rootforge is currently defining its first unattended OOM investigation slice.
Implementation changes must preserve the boundaries in `ARCHITECTURE.md` and
the evidence rules in `docs/`. Implemented code covers incident intake, durable
local Case state, and a leased investigation Outbox; most remaining packages are
still boundary scaffolds.

## Development commands

```bash
make fmt
make test
make vet
```

Never commit production logs, credentials, source archives, heap dumps, or
unredacted Incident Cases. Test fixtures must be synthetic or explicitly
sanitized.

The project license and public contribution process have not yet been selected.
