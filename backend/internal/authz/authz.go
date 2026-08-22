// Package authz defines authorization actors and policy checks.
package authz

import "time"

// GlobalRole is a backend domain type.
type GlobalRole string

// SeasonRole is a backend domain type.
type SeasonRole string

// EnrollmentState is a backend domain type.
type EnrollmentState string

// AccountState is a backend domain type.
type AccountState string

const (
	// Director is a public value used by the backend.
	Director GlobalRole = "director"
	// SystemAdmin is a public value used by the backend.
	SystemAdmin GlobalRole = "system_admin"
	// Student is a public value used by the backend.
	Student SeasonRole = "student"
	// Mentor is a public value used by the backend.
	Mentor SeasonRole = "mentor"
	// Coordinator is a public value used by the backend.
	Coordinator SeasonRole = "coordinator"
	// Active is a public value used by the backend.
	Active EnrollmentState = "active"
	// Completed is a public value used by the backend.
	Completed EnrollmentState = "completed"
	// Kicked is a public value used by the backend.
	Kicked EnrollmentState = "kicked"
	// Withdrawn is a public value used by the backend.
	Withdrawn EnrollmentState = "withdrawn"
	// AccountActive is a public value used by the backend.
	AccountActive AccountState = "active"
	// Suspended is a public value used by the backend.
	Suspended AccountState = "suspended"
	// DeletionPending is a public value used by the backend.
	DeletionPending AccountState = "deletion_pending"
	// Deleted is a public value used by the backend.
	Deleted AccountState = "deleted"
)

// Enrollment represents a backend data structure.
type Enrollment struct {
	SeasonID string
	Role     SeasonRole
	State    EnrollmentState
}

// Actor represents a backend data structure.
type Actor struct {
	UserID          string
	EmailVerified   bool
	AccountState    AccountState
	SecurityVersion int64
	GlobalRoles     map[GlobalRole]bool
	Enrollments     []Enrollment
	MFAAt           *time.Time
}

func (a Actor) authenticated() bool {
	return a.UserID != "" && a.EmailVerified && a.AccountState == AccountActive
}

// IsGlobal performs the operation.
func (a Actor) IsGlobal(role GlobalRole) bool { return a.GlobalRoles[role] }

// IsPrivileged performs the operation.
func (a Actor) IsPrivileged() bool { return a.IsGlobal(Director) || a.IsGlobal(SystemAdmin) }

// HasRecentMFA performs the operation.
func (a Actor) HasRecentMFA(now time.Time) bool {
	return a.MFAAt != nil && now.Sub(a.MFAAt.UTC()) >= 0 && now.Sub(a.MFAAt.UTC()) <= 15*time.Minute
}

// Enrollment performs the operation.
func (a Actor) Enrollment(seasonID string) (Enrollment, bool) {
	for _, e := range a.Enrollments {
		if e.SeasonID == seasonID {
			return e, true
		}
	}
	return Enrollment{}, false
}

// Alumni performs the operation.
func (a Actor) Alumni() bool {
	for _, e := range a.Enrollments {
		if e.Role == Student && e.State == Completed {
			return true
		}
	}
	return false
}

// EligibleMember performs the operation.
func (a Actor) EligibleMember() bool {
	if !a.authenticated() {
		return false
	}
	if a.Alumni() {
		return true
	}
	for _, e := range a.Enrollments {
		if e.State == Active {
			return true
		}
	}
	return false
}

// ProgrammeAccess includes historical members whose active enrollment was
// completed by closing a season. It is intentionally broader than
// EligibleMember, which remains the directory/basic-profile policy.
func (a Actor) ProgrammeAccess() bool {
	if !a.authenticated() {
		return false
	}
	for _, e := range a.Enrollments {
		if e.State == Active || e.State == Completed {
			return true
		}
	}
	return false
}

// CanViewBasicProfile performs the operation.
func (a Actor) CanViewBasicProfile(targetEligible bool) bool {
	return a.EligibleMember() && targetEligible
}

// CanViewPrivate performs the operation.
func (a Actor) CanViewPrivate(targetID, seasonID string, assignedMentor bool) bool {
	if !a.authenticated() {
		return false
	}
	if a.UserID == targetID || a.IsPrivileged() {
		return true
	}
	e, ok := a.Enrollment(seasonID)
	return ok && e.State == Active && ((e.Role == Mentor && assignedMentor) || e.Role == Coordinator)
}

// CanManageSeason performs the operation.
func (a Actor) CanManageSeason(seasonID string, open bool) bool {
	if !a.authenticated() || !open {
		return false
	}
	if a.IsPrivileged() {
		return true
	}
	e, ok := a.Enrollment(seasonID)
	return ok && e.State == Active && e.Role == Coordinator
}

// CanCloseSeason performs the operation.
func (a Actor) CanCloseSeason(seasonID string) bool { return a.CanManageSeason(seasonID, true) }

// CanReopenSeason performs the operation.
func (a Actor) CanReopenSeason(now time.Time) bool {
	return a.authenticated() && a.IsPrivileged() && a.HasRecentMFA(now)
}

// CanPromoteOrRemove performs the operation.
func (a Actor) CanPromoteOrRemove(seasonID string, assignedMentee, targetActive bool) bool {
	if !targetActive || !a.authenticated() {
		return false
	}
	if a.IsPrivileged() {
		return true
	}
	e, ok := a.Enrollment(seasonID)
	if !ok || e.State != Active {
		return false
	}
	return e.Role == Coordinator || (e.Role == Mentor && assignedMentee)
}
