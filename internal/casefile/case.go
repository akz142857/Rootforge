package casefile

import (
	"time"

	"rootforge/internal/incident"
)

// Case is Rootforge's authoritative record for one Incident investigation.
// Evidence, findings, runs, and RCA versions will be appended in later slices.
type Case struct {
	ID        string            `json:"id"`
	Revision  uint64            `json:"revision"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
	Incident  incident.Incident `json:"incident"`
}

// Clone returns a Case whose mutable fields do not alias the original.
func (c Case) Clone() Case {
	c.Incident = c.Incident.Clone()
	return c
}
