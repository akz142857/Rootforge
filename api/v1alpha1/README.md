# API v1alpha1

Reserved for versioned external contracts: incident events, Case summaries,
evidence requests, investigation results, action approvals, and notifications.

Schemas added here are public compatibility surfaces. Internal ClayHarness
adapter state does not belong in this directory.

The first implemented endpoints are:

- `POST /api/v1alpha1/events`: accept a scoped incident event and create or
  update its open Case.
- `GET /api/v1alpha1/cases/{caseID}`: inspect the current Case summary.
- `GET /healthz`: process health probe.

Event intake requires `type`, `source`, `occurred_at`, `environment`, and at
least one of `service`, `node`, `workload`, or `container`. This API is
`v1alpha1`; compatibility is not yet promised across minor development changes.
