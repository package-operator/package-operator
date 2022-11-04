package packages

type Lease interface {
	CanGo() bool
	ReportFinished()
}

// TODO: I don't know if this acutally qualifies as a lease
type ConcurrentLease struct {
	count int
	max   int
}

// TODO: No! Map of package name to timestamp
type lease struct {
	name string
	// timestamp
}

func NewLease(max int) Lease {
	// TODO: Should we read in all existing jobs on startup, or can we assume it is okay (package controller will run an
	// create a lease
	return &ConcurrentLease{
		count: 0,
		max:   max,
	}
}

func (l *ConcurrentLease) CanGo() bool {
	if l.count >= l.max {
		return false
	}
	l.count++
	return true
}

func (l *ConcurrentLease) ReportFinished() {
	l.count--
}

func remove(s []int, i int) []int {
	s[i] = s[len(s)-1]
	return s[:len(s)-1]
}
