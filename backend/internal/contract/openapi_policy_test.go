package contract_test

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"sort"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func loadContract(t *testing.T) *openapi3.T {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate contract test")
	}
	path := filepath.Join(filepath.Dir(filename), "..", "..", "..", "api", "openapi.yaml")
	document, err := openapi3.NewLoader().LoadFromFile(path)
	if err != nil {
		t.Fatalf("load OpenAPI contract: %v", err)
	}
	if err := document.Validate(context.Background()); err != nil {
		t.Fatalf("validate OpenAPI contract: %v", err)
	}
	return document
}

func TestProtectedOperationsDeclareAuthenticationAndRateLimitFailures(t *testing.T) {
	document := loadContract(t)
	public := map[string]bool{
		"GET /health/live":  true,
		"GET /health/ready": true,
		"GET /metrics":      true,
		"GET /openapi.json": true,
	}
	seenOperationIDs := map[string]string{}
	var missing []string
	for path, item := range document.Paths.Map() {
		for method, operation := range item.Operations() {
			key := method + " " + path
			if previous := seenOperationIDs[operation.OperationID]; previous != "" {
				t.Errorf("operationId %q is shared by %s and %s", operation.OperationID, previous, key)
			}
			seenOperationIDs[operation.OperationID] = key
			if public[key] {
				continue
			}
			for _, status := range []string{"401", "403", "429"} {
				if operation.Responses.Value(status) == nil {
					missing = append(missing, fmt.Sprintf("%s (%s)", key, status))
				}
			}
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("protected operations must explicitly document 401, 403 and 429 responses: %v", missing)
	}
	rateLimited := document.Components.Responses["RateLimited"]
	if rateLimited == nil || rateLimited.Value == nil || rateLimited.Value.Headers["Retry-After"] == nil {
		t.Fatal("RateLimited response must require Retry-After")
	}
}

func TestGrowingCollectionsAreCursorPagedAndFiltered(t *testing.T) {
	document := loadContract(t)
	required := map[string][]string{
		"listUsers":                {"limit", "cursor", "direction", "sort", "query", "seasonRole", "globalRole"},
		"listSeasons":              {"limit", "cursor", "direction", "sort", "status"},
		"listEnrollmentCandidates": {"limit", "cursor", "direction", "sort", "query"},
		"listSeasonWeeks":          {"limit", "cursor", "direction", "sort"},
		"listSeasonMembers":        {"limit", "cursor", "direction", "sort", "role", "state"},
		"listSeasonMentorships":    {"limit", "cursor", "direction", "sort", "mentorUserId", "studentUserId"},
		"listLeetcodeProblems":     {"limit", "cursor", "direction", "sort", "difficulty", "category", "premium"},
		"listProblemAttempts":      {"limit", "cursor", "direction", "sort", "userId", "outcome", "difficulty"},
		"listMockInterviews":       {"limit", "cursor", "direction", "sort", "mode"},
	}
	for _, item := range document.Paths.Map() {
		for _, operation := range item.Operations() {
			names, tracked := required[operation.OperationID]
			if !tracked {
				continue
			}
			present := map[string]bool{}
			for _, parameter := range operation.Parameters {
				if parameter.Value != nil && parameter.Value.In == openapi3.ParameterInQuery {
					present[parameter.Value.Name] = true
				}
			}
			for _, name := range names {
				if !present[name] {
					t.Errorf("%s is missing query parameter %s", operation.OperationID, name)
				}
			}
		}
	}
}

func TestMutableOperationsRequireARevision(t *testing.T) {
	document := loadContract(t)
	required := map[string]bool{
		"updateMe": true, "updatePracticeSettings": true, "enableUserPracticeGoals": true,
		"updateSeason": true, "closeSeason": true, "reopenSeason": true,
		"updateSeasonWeek": true, "deleteSeasonWeek": true,
		"updateSeasonMember": true, "promoteSeasonMember": true, "removeSeasonMember": true,
		"updateMentorship": true, "deleteMentorship": true,
		"updateProblemAttempt": true, "deleteProblemAttempt": true,
		"dismissCurrentRecommendation": true,
		"updateMockInterview":          true, "deleteMockInterview": true,
		"correctMockInterviewIdentities": true, "reviewMockInterviewRound": true,
		"setAdminUserAccountState": true, "revokeAdminUserGlobalRole": true,
	}
	seen := map[string]bool{}
	for _, item := range document.Paths.Map() {
		for _, operation := range item.Operations() {
			if !required[operation.OperationID] {
				continue
			}
			seen[operation.OperationID] = true
			if operationRequiresRevision(operation) {
				continue
			}
			t.Errorf("%s must require revision in its body or query", operation.OperationID)
		}
	}
	for operationID := range required {
		if !seen[operationID] {
			t.Errorf("required mutable operation %s is absent", operationID)
		}
	}
}

func operationRequiresRevision(operation *openapi3.Operation) bool {
	for _, parameter := range operation.Parameters {
		if parameter.Value != nil && parameter.Value.In == openapi3.ParameterInQuery && parameter.Value.Name == "revision" && parameter.Value.Required {
			return true
		}
	}
	if operation.RequestBody == nil || operation.RequestBody.Value == nil {
		return false
	}
	for _, media := range operation.RequestBody.Value.Content {
		if schemaRequiresProperty(media.Schema, "revision", map[*openapi3.Schema]bool{}) {
			return true
		}
	}
	return false
}

func schemaRequiresProperty(ref *openapi3.SchemaRef, property string, seen map[*openapi3.Schema]bool) bool {
	return schemaMarksRequired(ref, property, seen) && schemaDefinesProperty(ref, property, map[*openapi3.Schema]bool{})
}

func schemaMarksRequired(ref *openapi3.SchemaRef, property string, seen map[*openapi3.Schema]bool) bool {
	if ref == nil || ref.Value == nil || seen[ref.Value] {
		return false
	}
	seen[ref.Value] = true
	for _, required := range ref.Value.Required {
		if required == property {
			return true
		}
	}
	for _, part := range ref.Value.AllOf {
		if schemaMarksRequired(part, property, seen) {
			return true
		}
	}
	return false
}

func schemaDefinesProperty(ref *openapi3.SchemaRef, property string, seen map[*openapi3.Schema]bool) bool {
	if ref == nil || ref.Value == nil || seen[ref.Value] {
		return false
	}
	seen[ref.Value] = true
	if _, exists := ref.Value.Properties[property]; exists {
		return true
	}
	for _, part := range ref.Value.AllOf {
		if schemaDefinesProperty(part, property, seen) {
			return true
		}
	}
	return false
}
