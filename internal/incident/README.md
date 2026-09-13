# Incident

Owns incident identity, deduplication, aggregation, lifecycle, priority, and
workflow scheduling. It converts normalized events into new or updated Cases and
decides when an unattended investigation should start or resume.

It does not interpret evidence or select a root cause.

The implemented lifecycle aggregate starts in `pending_investigation`, tracks
the first and last occurrence, and appends matching normalized Events. Resolved
and closed states are reserved, but their transition policy is not implemented
yet.
