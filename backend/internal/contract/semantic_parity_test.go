package contract_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
)

const (
	legacySourceRepository = "/Users/maged/code/RSP-Web"
	legacySourceCommit     = "8391ba51f4d56226c4ac28afd9748e83183f9016"
	legacySourceTree       = "13d4a47b2164cb3fc898ffe16383a748157fd2c6"
)

var (
	gitObjectID       = regexp.MustCompile(`^[0-9a-f]{40}$`)
	templateParameter = regexp.MustCompile(`\{[^/{}]+\}`)
)

type semanticParityFixture struct {
	Version int `json:"version"`
	Source  struct {
		Repository string `json:"repository"`
		Commit     string `json:"commit"`
		Tree       string `json:"tree"`
	} `json:"source"`
	Conventions struct {
		Timestamps         string `json:"timestamps"`
		LegacyIDs          string `json:"legacyIds"`
		FixtureOnly        bool   `json:"fixtureOnly"`
		ExpectationIndexes string `json:"expectationIndexes"`
	} `json:"conventions"`
	Workflows []semanticParityWorkflow `json:"workflows"`
	Residuals []semanticParityResidual `json:"residualCoverage"`
}

type semanticParityWorkflow struct {
	ID         string                   `json:"id"`
	Family     string                   `json:"family"`
	Actor      string                   `json:"actor"`
	Setup      []string                 `json:"setup"`
	Operations []string                 `json:"operations"`
	Expect     []string                 `json:"expect"`
	V2         []string                 `json:"v2"`
	Evidence   []semanticParityEvidence `json:"evidence"`
}

type semanticParityEvidence struct {
	Expectations []int  `json:"expectations"`
	Path         string `json:"path"`
	Anchor       string `json:"anchor"`
}

type semanticParityResidual struct {
	Workflow string `json:"workflow"`
	Missing  string `json:"missing"`
}

func TestLegacyWorkflowSemanticParityEvidence(t *testing.T) {
	repositoryRoot := contractRepositoryRoot(t)
	fixture := loadSemanticParityFixture(t, repositoryRoot)

	if fixture.Version != 2 {
		t.Fatalf("semantic parity fixture version = %d, want 2", fixture.Version)
	}
	if !gitObjectID.MatchString(fixture.Source.Commit) || !gitObjectID.MatchString(fixture.Source.Tree) {
		t.Fatalf("source commit and tree must be lowercase 40-character Git object IDs: commit=%q tree=%q", fixture.Source.Commit, fixture.Source.Tree)
	}
	if fixture.Source.Repository != legacySourceRepository || fixture.Source.Commit != legacySourceCommit || fixture.Source.Tree != legacySourceTree {
		t.Fatalf(
			"legacy source provenance drifted: got repository=%q commit=%q tree=%q, want repository=%q commit=%q tree=%q",
			fixture.Source.Repository,
			fixture.Source.Commit,
			fixture.Source.Tree,
			legacySourceRepository,
			legacySourceCommit,
			legacySourceTree,
		)
	}
	if fixture.Conventions.ExpectationIndexes != "zero-based" || !fixture.Conventions.FixtureOnly {
		t.Fatalf("fixture conventions must declare zero-based expectation indexes and fixtureOnly=true: %#v", fixture.Conventions)
	}

	document := loadContract(t)
	contractOperations := make(map[string]bool)
	for path, item := range document.Paths.Map() {
		for method := range item.Operations() {
			contractOperations[method+" "+normalizeTemplatePath(path)] = true
		}
	}

	allowedMethods := map[string]bool{"GET": true, "POST": true, "PATCH": true, "DELETE": true}
	workflowIDs := make(map[string]bool)
	fixtureRoutes := make(map[string]bool)
	families := make(map[string]bool)
	for index, workflow := range fixture.Workflows {
		if workflow.ID == "" || workflow.Family == "" || workflow.Actor == "" {
			t.Errorf("workflow %d must have non-empty id, family and actor", index)
			continue
		}
		if workflowIDs[workflow.ID] {
			t.Errorf("duplicate workflow id %q", workflow.ID)
			continue
		}
		workflowIDs[workflow.ID] = true
		families[workflow.Family] = true
		t.Run(workflow.ID, func(t *testing.T) {
			if len(workflow.Setup) == 0 || len(workflow.Operations) == 0 || len(workflow.Expect) == 0 || len(workflow.V2) == 0 || len(workflow.Evidence) == 0 {
				t.Fatalf("setup, operations, expectations, v2 routes and evidence must all be non-empty")
			}

			seenRoutes := make(map[string]bool)
			for _, route := range workflow.V2 {
				if seenRoutes[route] {
					t.Errorf("duplicate v2 route %q", route)
					continue
				}
				seenRoutes[route] = true
				fixtureRoutes[route] = true
				method, path, err := parseFixtureRoute(route, allowedMethods)
				if err != nil {
					t.Errorf("v2 route %q: %v", route, err)
					continue
				}

				key := method + " " + normalizeTemplatePath(path)
				if !contractOperations[key] {
					t.Errorf("v2 route %q normalizes to %q, which is absent from api/openapi.yaml", route, key)
				}
			}

			covered := make([]bool, len(workflow.Expect))
			for evidenceIndex, evidence := range workflow.Evidence {
				if err := validateEvidenceAnchor(repositoryRoot, evidence); err != nil {
					t.Errorf("evidence %d: %v", evidenceIndex, err)
				}
				if len(evidence.Expectations) == 0 {
					t.Errorf("evidence %d has no expectation indexes", evidenceIndex)
					continue
				}
				seenIndexes := make(map[int]bool)
				for _, expectationIndex := range evidence.Expectations {
					if expectationIndex < 0 || expectationIndex >= len(workflow.Expect) {
						t.Errorf("evidence %d expectation index %d is outside [0,%d)", evidenceIndex, expectationIndex, len(workflow.Expect))
						continue
					}
					if seenIndexes[expectationIndex] {
						t.Errorf("evidence %d repeats expectation index %d", evidenceIndex, expectationIndex)
						continue
					}
					seenIndexes[expectationIndex] = true
					covered[expectationIndex] = true
				}
			}
			for expectationIndex, isCovered := range covered {
				if !isCovered {
					t.Errorf("expectation %d has no named executable evidence: %s", expectationIndex, workflow.Expect[expectationIndex])
				}
			}
		})
	}

	wantFamilies := []string{"admin_crud", "attempts_recommendation", "enrollment_mentorship", "mock_interviews", "seasons_weeks", "user_profile"}
	var gotFamilies []string
	for family := range families {
		gotFamilies = append(gotFamilies, family)
	}
	sort.Strings(gotFamilies)
	if fmt.Sprint(gotFamilies) != fmt.Sprint(wantFamilies) {
		t.Errorf("workflow families = %v, want %v", gotFamilies, wantFamilies)
	}

	requiredReplacementRoutes := []string{
		"GET /api/v2/admin/users",
		"POST /api/v2/admin/users/{id}/account-state",
		"POST /api/v2/admin/users/{id}/global-roles",
		"GET /api/v2/admin/users/{id}/global-roles",
		"DELETE /api/v2/admin/users/{id}/global-roles/{role}",
		"POST /api/v2/seasons/{id}/close",
		"POST /api/v2/seasons/{id}/reopen",
		"PATCH /api/v2/seasons/{id}/weeks/{weekId}",
		"DELETE /api/v2/seasons/{id}/weeks/{weekId}",
		"PATCH /api/v2/seasons/{id}/members/{memberId}",
		"POST /api/v2/seasons/{id}/members/{memberId}/remove",
		"PATCH /api/v2/seasons/{id}/mentorships/{mentorshipId}",
		"DELETE /api/v2/seasons/{id}/mentorships/{mentorshipId}",
	}
	for _, route := range requiredReplacementRoutes {
		if !fixtureRoutes[route] {
			t.Errorf("intentional replacement route %q is missing from the parity fixture", route)
		}
	}

	if fixture.Residuals == nil {
		t.Fatal("residualCoverage must be present; use an empty array when every expectation has direct evidence")
	}
	seenResiduals := make(map[string]bool)
	for index, residual := range fixture.Residuals {
		if !workflowIDs[residual.Workflow] {
			t.Errorf("residual coverage item %d references unknown workflow %q", index, residual.Workflow)
		}
		if strings.TrimSpace(residual.Missing) == "" {
			t.Errorf("residual coverage item %d has no exact missing behavior", index)
		}
		key := residual.Workflow + "\x00" + residual.Missing
		if seenResiduals[key] {
			t.Errorf("duplicate residual coverage item for workflow %q: %s", residual.Workflow, residual.Missing)
		}
		seenResiduals[key] = true
		t.Errorf("residual coverage item %d is a release blocker for workflow %q: %s", index, residual.Workflow, residual.Missing)
	}
}

func loadSemanticParityFixture(t *testing.T, repositoryRoot string) semanticParityFixture {
	t.Helper()
	path := filepath.Join(repositoryRoot, "docs", "golden", "legacy-workflows.json")
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read semantic parity fixture: %v", err)
	}

	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	var fixture semanticParityFixture
	if err := decoder.Decode(&fixture); err != nil {
		t.Fatalf("decode semantic parity fixture: %v", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		t.Fatalf("semantic parity fixture contains trailing JSON: %v", err)
	}
	return fixture
}

func contractRepositoryRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate semantic parity contract test")
	}
	root, err := filepath.Abs(filepath.Join(filepath.Dir(filename), "..", "..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	return root
}

func parseFixtureRoute(route string, allowedMethods map[string]bool) (string, string, error) {
	method, target, ok := strings.Cut(route, " ")
	if !ok || method == "" || target == "" || strings.Contains(target, " ") {
		return "", "", fmt.Errorf("want exactly 'METHOD /api/v2/path'")
	}
	if !allowedMethods[method] {
		return "", "", fmt.Errorf("method %q is not one of GET, POST, PATCH or DELETE", method)
	}
	path, rawQuery, hasQuery := strings.Cut(target, "?")
	if strings.Contains(path, "#") || strings.Contains(rawQuery, "#") {
		return "", "", fmt.Errorf("fragments are not allowed")
	}
	if hasQuery {
		if rawQuery == "" {
			return "", "", fmt.Errorf("empty query string")
		}
		if _, err := url.ParseQuery(rawQuery); err != nil {
			return "", "", fmt.Errorf("invalid query string: %w", err)
		}
	}
	if !strings.HasPrefix(path, "/api/v2/") {
		return "", "", fmt.Errorf("path %q is outside /api/v2", path)
	}
	return method, strings.TrimPrefix(path, "/api/v2"), nil
}

func normalizeTemplatePath(path string) string {
	return templateParameter.ReplaceAllString(path, "{}")
}

func validateEvidenceAnchor(repositoryRoot string, evidence semanticParityEvidence) error {
	if evidence.Path == "" || evidence.Anchor == "" {
		return fmt.Errorf("path and anchor must be non-empty")
	}
	if filepath.IsAbs(evidence.Path) || filepath.Clean(evidence.Path) != evidence.Path {
		return fmt.Errorf("path %q must be a clean repository-relative path", evidence.Path)
	}
	if !strings.HasSuffix(evidence.Path, "_test.go") && !strings.Contains(filepath.Base(evidence.Path), ".test.") && !strings.Contains(filepath.Base(evidence.Path), ".spec.") {
		return fmt.Errorf("path %q is not a named test source", evidence.Path)
	}
	if strings.HasSuffix(evidence.Path, ".go") {
		if !strings.HasPrefix(evidence.Anchor, "func Test") {
			return fmt.Errorf("Go anchor %q must name a Test function", evidence.Anchor)
		}
	} else if !strings.HasPrefix(evidence.Anchor, "it('") && !strings.HasPrefix(evidence.Anchor, "it(\"") && !strings.HasPrefix(evidence.Anchor, "test('") && !strings.HasPrefix(evidence.Anchor, "test(\"") {
		return fmt.Errorf("TypeScript anchor %q must name an it or test case", evidence.Anchor)
	}

	realRoot, err := filepath.EvalSymlinks(repositoryRoot)
	if err != nil {
		return fmt.Errorf("resolve repository root: %w", err)
	}

	candidate := filepath.Join(repositoryRoot, evidence.Path)
	realCandidate, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return fmt.Errorf("resolve evidence path %q: %w", evidence.Path, err)
	}

	relative, err := filepath.Rel(realRoot, realCandidate)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("evidence path %q escapes the repository", evidence.Path)
	}

	contents, err := os.ReadFile(realCandidate)
	if err != nil {
		return fmt.Errorf("read evidence path %q: %w", evidence.Path, err)
	}
	if !bytes.Contains(contents, []byte(evidence.Anchor)) {
		return fmt.Errorf("named test anchor %q is absent from %s", evidence.Anchor, evidence.Path)
	}
	return nil
}
