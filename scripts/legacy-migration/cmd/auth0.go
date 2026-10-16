package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	legacy "github.com/magedmg/RSP-website/scripts/legacy-migration/migration"
)

func noPositional(set *flag.FlagSet, args []string) error {
	if err := set.Parse(args); err != nil {
		return err
	}
	if set.NArg() != 0 {
		return fmt.Errorf("%s does not accept positional arguments", set.Name())
	}
	return nil
}

func requireFlags(values map[string]string) error {
	missing := make([]string, 0)
	for name, value := range values {
		if strings.TrimSpace(value) == "" {
			missing = append(missing, "--"+name)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	sort.Strings(missing)
	return fmt.Errorf("required: %s", strings.Join(missing, ", "))
}

func normalizeAuth0(args []string, stdout, stderr io.Writer) error {
	set := flag.NewFlagSet("auth0 normalize", flag.ContinueOnError)
	set.SetOutput(stderr)
	exportPath := set.String("export", "", "Auth0 users-exports job file, gzip or NDJSON (required)")
	outPath := set.String("out", "", "normalized Auth0 users JSON (required)")
	if err := noPositional(set, args); err != nil {
		return err
	}
	if err := requireFlags(map[string]string{"export": *exportPath, "out": *outPath}); err != nil {
		return err
	}
	file, err := os.Open(*exportPath)
	if err != nil {
		return fmt.Errorf("open Auth0 export: %w", err)
	}
	defer file.Close()
	users, err := legacy.NormalizeAuth0Export(file)
	if err != nil {
		return err
	}
	if err := legacy.WriteJSON(*outPath, users); err != nil {
		return err
	}
	identities, blocked, withLegacyID := 0, 0, 0
	for _, user := range users {
		identities += len(user.Identities)
		if user.Blocked {
			blocked++
		}
		if user.LegacyUserID != "" {
			withLegacyID++
		}
	}
	fmt.Fprintf(stdout, "out=%s auth0Users=%d identities=%d blocked=%d withLegacyUserId=%d\n",
		*outPath, len(users), identities, blocked, withLegacyID)
	return nil
}

func auth0Candidates(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	set := flag.NewFlagSet("auth0 candidates", flag.ContinueOnError)
	set.SetOutput(stderr)
	runID := set.String("run-id", "", "verified data migration run id (required)")
	dsn := set.String("target-dsn", os.Getenv("DATABASE_URL"), "target PostgreSQL DSN (default DATABASE_URL)")
	outPath := set.String("out", "", "app identity candidates JSON (required)")
	if err := noPositional(set, args); err != nil {
		return err
	}
	if err := requireFlags(map[string]string{"run-id": *runID, "target-dsn": *dsn, "out": *outPath}); err != nil {
		return err
	}
	target := &legacy.Auth0ImportTarget{ConnectionString: *dsn}
	candidates, err := target.ExportCandidates(ctx, *runID)
	if err != nil {
		return err
	}
	if err := legacy.WriteJSON(*outPath, candidates); err != nil {
		return err
	}
	withLegacyID := 0
	for _, candidate := range candidates {
		if candidate.LegacyUserID != "" {
			withLegacyID++
		}
	}
	fmt.Fprintf(stdout, "out=%s candidates=%d withLegacyUserId=%d\n", *outPath, len(candidates), withLegacyID)
	return nil
}

func proposeAuth0(args []string, stdout, stderr io.Writer) error {
	set := flag.NewFlagSet("auth0 propose", flag.ContinueOnError)
	set.SetOutput(stderr)
	usersPath := set.String("auth0-users", "", "normalized Auth0 users JSON (required)")
	candidatesPath := set.String("app-candidates", "", "app identity candidates JSON (required)")
	outPath := set.String("out", "", "proposed resolutions JSON, for review (required)")
	reportPath := set.String("report", "", "manual-resolution report JSON (required)")
	if err := noPositional(set, args); err != nil {
		return err
	}
	if err := requireFlags(map[string]string{"auth0-users": *usersPath, "app-candidates": *candidatesPath, "out": *outPath, "report": *reportPath}); err != nil {
		return err
	}
	var users []legacy.Auth0User
	if err := loadJSON(*usersPath, "Auth0 users", &users); err != nil {
		return err
	}
	var candidates []legacy.AppIdentityCandidate
	if err := loadJSON(*candidatesPath, "app identity candidates", &candidates); err != nil {
		return err
	}
	proposal := legacy.ProposeAuth0Resolutions(users, candidates)
	if err := legacy.WriteJSON(*outPath, proposal.Resolutions); err != nil {
		return err
	}
	if err := legacy.WriteJSON(*reportPath, struct {
		Counts map[string]int            `json:"counts"`
		Gaps   []legacy.Auth0ProposalGap `json:"gaps"`
	}{proposal.Counts, proposal.Gaps}); err != nil {
		return err
	}
	counts, err := json.Marshal(proposal.Counts)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "out=%s report=%s proposed=%d manual=%d counts=%s\n",
		*outPath, *reportPath, len(proposal.Resolutions), len(proposal.Gaps), counts)
	return nil
}

func applyAuth0(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	set := flag.NewFlagSet("auth0 apply", flag.ContinueOnError)
	set.SetOutput(stderr)
	planPath := set.String("plan", "", "approved Auth0 import plan JSON (required)")
	usersPath := set.String("auth0-users", "", "normalized Auth0 users JSON the plan was built from (required)")
	candidatesPath := set.String("app-candidates", "", "app identity candidates JSON the plan was built from (required)")
	resolutionsPath := set.String("resolutions", "", "approved identity resolutions JSON the plan was built from")
	runID := set.String("run-id", "", "verified data migration run id (required)")
	dsn := set.String("target-dsn", os.Getenv("DATABASE_URL"), "target PostgreSQL DSN (default DATABASE_URL)")
	allowUnresolved := set.Bool("allow-unresolved", false, "import resolved identities and leave unresolved Auth0 users unimported")
	if err := noPositional(set, args); err != nil {
		return err
	}
	if err := requireFlags(map[string]string{"plan": *planPath, "auth0-users": *usersPath, "app-candidates": *candidatesPath, "run-id": *runID, "target-dsn": *dsn}); err != nil {
		return err
	}
	var plan legacy.Auth0ImportPlan
	if err := loadJSON(*planPath, "Auth0 import plan", &plan); err != nil {
		return err
	}
	var users []legacy.Auth0User
	if err := loadJSON(*usersPath, "Auth0 users", &users); err != nil {
		return err
	}
	var candidates []legacy.AppIdentityCandidate
	if err := loadJSON(*candidatesPath, "app identity candidates", &candidates); err != nil {
		return err
	}
	resolutions := make([]legacy.IdentityResolution, 0)
	if *resolutionsPath != "" {
		if err := loadJSON(*resolutionsPath, "identity resolutions", &resolutions); err != nil {
			return err
		}
	}
	if err := legacy.VerifyAuth0PlanBinding(plan, users, candidates, resolutions); err != nil {
		return err
	}
	groups, err := legacy.BuildAuth0ImportGroups(plan, users, *allowUnresolved)
	if err != nil {
		return err
	}
	target := &legacy.Auth0ImportTarget{ConnectionString: *dsn}
	summary, err := target.Apply(ctx, *runID, plan, groups)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "applied planChecksum=%s authUsers=%d googleAccounts=%d passwordUsers=%d activeLinks=%d pendingLinks=%d unresolved=%d\n",
		plan.Checksum, summary.AuthUsers, summary.GoogleAccounts, summary.PasswordUsers,
		summary.ActiveLinks, summary.PendingLinks, summary.Unresolved)
	return nil
}

func verifyAuth0(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	set := flag.NewFlagSet("auth0 verify", flag.ContinueOnError)
	set.SetOutput(stderr)
	runID := set.String("run-id", "", "data migration run id the import was recorded against (required)")
	dsn := set.String("target-dsn", os.Getenv("DATABASE_URL"), "target PostgreSQL DSN (default DATABASE_URL)")
	if err := noPositional(set, args); err != nil {
		return err
	}
	if err := requireFlags(map[string]string{"run-id": *runID, "target-dsn": *dsn}); err != nil {
		return err
	}
	target := &legacy.Auth0ImportTarget{ConnectionString: *dsn}
	verification, err := target.Verify(ctx, *runID)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(verification); err != nil {
		return err
	}
	if problems := verification.Problems(); len(problems) > 0 {
		return fmt.Errorf("%w: %s", legacy.ErrAuth0ImportInvariant, strings.Join(problems, "; "))
	}
	return nil
}
