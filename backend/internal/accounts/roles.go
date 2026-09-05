package accounts

// GlobalRoleAssignment records a global account role.
type GlobalRoleAssignment struct {
	ID     string `json:"id"`
	UserID string `json:"userId"`
	Role   string `json:"role"`
	State  string `json:"state"`
}
