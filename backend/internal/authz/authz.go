// Package authz defines authorization actors and policy checks.
package authz

import "time"

type GlobalRole string

type SeasonRole string

type EnrollmentState string

type AccountState string

const (
	Director        GlobalRole      = "director"
	SystemAdmin     GlobalRole      = "system_admin"
	Student         SeasonRole      = "student"
	Mentor          SeasonRole      = "mentor"
	Coordinator     SeasonRole      = "coordinator"
	Active          EnrollmentState = "active"
	Completed       EnrollmentState = "completed"
	Kicked          EnrollmentState = "kicked"
	Withdrawn       EnrollmentState = "withdrawn"
	AccountActive   AccountState    = "active"
	Suspended       AccountState    = "suspended"
	DeletionPending AccountState    = "deletion_pending"
	Deleted         AccountState    = "deleted"
)

type Enrollment struct {
	SeasonID string
	Role     SeasonRole
	State    EnrollmentState
}

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

func (a Actor) IsGlobal(role GlobalRole) bool { return a.GlobalRoles[role] }

func (a Actor) IsPrivileged() bool { return a.IsGlobal(Director) || a.IsGlobal(SystemAdmin) }

func (a Actor) HasRecentMFA(now time.Time) bool {
	return a.MFAAt != nil && now.Sub(a.MFAAt.UTC()) >= 0 && now.Sub(a.MFAAt.UTC()) <= 15*time.Minute
}

func (a Actor) Enrollment(seasonID string) (Enrollment, bool) {
	for _, e := range a.Enrollments {
		if e.SeasonID == seasonID {
			return e, true
		}
	}
	return Enrollment{}, false
}

func (a Actor) Alumni() bool {
	for _, e := range a.Enrollments {
		if e.Role == Student && e.State == Completed {
			return true
		}
	}
	return false
}

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

// CanViewSeason preserves access to completed seasons for active members and alumni.
func (a Actor) CanViewSeason(seasonID string) bool {
	if !a.authenticated() {
		return false
	}
	if a.IsPrivileged() {
		return true
	}
	e, ok := a.Enrollment(seasonID)
	return ok && (e.State == Active || e.State == Completed && a.EligibleMember())
}

// MemberRelationship contains facts loaded for a private-data access check.
type MemberRelationship struct {
	SeasonID       string
	TargetEnrolled bool
	AssignedMentor bool
}

// CanViewPrivate includes historical mentor/coordinator relationships after a
// season closes. TargetEnrolled means an active or completed target enrollment.
func (a Actor) CanViewPrivate(targetID string, relationship MemberRelationship) bool {
	if !a.authenticated() {
		return false
	}
	if a.UserID == targetID || a.IsPrivileged() {
		return true
	}
	e, ok := a.Enrollment(relationship.SeasonID)
	if !ok || (e.State != Active && e.State != Completed) {
		return false
	}
	return (e.Role == Mentor && relationship.AssignedMentor) ||
		(e.Role == Coordinator && relationship.TargetEnrolled)
}

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

func (a Actor) CanCloseSeason(seasonID string) bool { return a.CanManageSeason(seasonID, true) }

func (a Actor) CanReopenSeason(now time.Time) bool {
	return a.authenticated() && a.IsPrivileged() && a.HasRecentMFA(now)
}

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
