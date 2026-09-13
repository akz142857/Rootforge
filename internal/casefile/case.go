package casefile

import (
	"time"

	"rootforge/internal/incident"
)

// Case is Rootforge's authoritative record for one Incident investigation.
// Evidence, findings, runs, and RCA versions will be appended in later slices.
type Case struct {
	ID        string
	Revision  uint64
	CreatedAt time.Time
	UpdatedAt time.Time
	Incident  incident.Incident
}

// Clone returns a Case whose mutable fields do not alias the original.
func (c Case) Clone() Case {
	c.Incident = c.Incident.Clone()
	return c
}
