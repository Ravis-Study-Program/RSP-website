package problem

import (
	"encoding/json"
	"net/http"
)

type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}
type Details struct {
	Type      string       `json:"type"`
	Title     string       `json:"title"`
	Status    int          `json:"status"`
	Detail    string       `json:"detail,omitempty"`
	Instance  string       `json:"instance,omitempty"`
	Code      string       `json:"code"`
	RequestID string       `json:"requestId,omitempty"`
	Errors    []FieldError `json:"errors,omitempty"`
}

func Write(w http.ResponseWriter, v Details) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(v.Status)
	_ = json.NewEncoder(w).Encode(v)
}
