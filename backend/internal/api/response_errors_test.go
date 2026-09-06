package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResponseEncodingFailureDoesNotCommitSuccess(t *testing.T) {
	response := httptest.NewRecorder()
	writeJSONResponse(response, http.StatusOK, make(chan int))
	if response.Code != http.StatusInternalServerError || errorMessage(t, response) != "The request could not be completed." {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestUnexpectedMockErrorDoesNotLeakDetails(t *testing.T) {
	response := httptest.NewRecorder()
	New(Config{}).writeMockErrorResponse(response, errors.New("private internal details"))
	if response.Code != http.StatusInternalServerError || strings.Contains(response.Body.String(), "private internal details") {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}
