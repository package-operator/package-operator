package packages

import (
	"time"
)

type Lease interface {
	GetLease(name string) bool
	ReportFinished(name string)
}

var _ Lease = &ConcurrentLease{}

// TODO: Should we read in all existing jobs on startup, or can we assume it is okay (package controller will run an
// create a lease
// TODO: Should we kill jobs that time out?
type ConcurrentLease struct {
	leases map[string]time.Time
	max    int
}

func (c *ConcurrentLease) GetLease(name string) bool {
	if len(c.leases) >= c.max {
		return false
	}
	c.leases[name] = time.Now()
	return true
}

func (c *ConcurrentLease) ReportFinished(name string) {
	delete(c.leases, name)
}
