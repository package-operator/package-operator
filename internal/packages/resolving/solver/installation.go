package solver

import (
	"errors"
)

// ErrDatastructure indicates that the overall data structure of an [Installation] is invalid.
var ErrDatastructure = errors.New("installation data structure is invalid")

type InstallationData any

type MockInstallationData struct{}

// InstallationConstrainer is used to generate constraints for an [Installation].
type InstallationConstrainer[IM InstallationData, SM ScopeData, CM CandidateData] func(i InstallationAccessor[IM, SM, CM]) []Constraint

// InstallationAccessor is used to access a [Installation] read only.
type InstallationAccessor[IM InstallationData, SM ScopeData, CM CandidateData] interface {
	// InstallationScopes returns all [Scope] of the [Installation].
	// Returned slice may be modified.
	InstallationScopes() []ScopeAccessor[IM, SM, CM]
	// InstallationMetadata returns the meta data of the [Installation].
	InstallationData() IM
}

// Installation defines an installation problem that is to be resolved.
type Installation[IM InstallationData, SM ScopeData, CM CandidateData] struct {
	variable

	// Scopes defines all scopes of this installation.
	Scopes []Scope[IM, SM, CM]
	// Data contains arbitrary meta data.
	Data IM

	// Constrainers are called to generate solver constraints.
	Constrainers []InstallationConstrainer[IM, SM, CM]
}

func (i Installation[IM, _, _]) InstallationData() IM { return i.Data }

func (i Installation[IM, SM, CM]) InstallationScopes() []ScopeAccessor[IM, SM, CM] {
	res := []ScopeAccessor[IM, SM, CM]{}
	for _, s := range i.Scopes {
		res = append(res, s)
	}

	return res
}
