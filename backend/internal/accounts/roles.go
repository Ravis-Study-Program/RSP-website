package accounts

// GlobalRoleAssignment records a global account role and its revision.
type GlobalRoleAssignment struct {
	ID       string `json:"id"`
	UserID   string `json:"userId"`
	Role     string `json:"role"`
	State    string `json:"state"`
	Revision int64  `json:"revision"`
}
