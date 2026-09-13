# Contributing to Rootforge

Rootforge is currently defining its first unattended OOM investigation slice.
Implementation changes must preserve the boundaries in `ARCHITECTURE.md` and
the evidence rules in `docs/`. The first executable slice covers incident event
intake and in-memory Case creation; most remaining packages are still boundary
scaffolds.

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
