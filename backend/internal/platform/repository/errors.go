// Package repository defines errors shared by persistence ports and adapters.
package repository

import "errors"

var (
	ErrNotFound  = errors.New("not found")
	ErrConflict  = errors.New("conflict")
	ErrDuplicate = errors.New("duplicate")
)
