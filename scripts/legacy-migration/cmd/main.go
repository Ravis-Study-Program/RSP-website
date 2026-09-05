// Package main provides commands for the RSP data migration tool.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	legacy "github.com/magedmg/RSP-website/scripts/legacy-migration/migration"
)

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) < 2 {
		printUsage(stderr)
		return 2
	}
	engine := legacy.NewEngine(time.Now)
	var err error
	switch args[0] {
	case "legacy":
		switch args[1] {
		case "dry-run":
			err = dryRun(ctx, engine, args[2:], stdout, stderr)
		case "apply":
			err = apply(ctx, engine, args[2:], stdout, stderr)
		case "verify":
			err = verify(ctx, engine, args[2:], stdout, stderr)
		default:
			printUsage(stderr)
			return 2
		}
	case "auth0":
		if args[1] != "plan" {
			printUsage(stderr)
			return 2
		}
		err = planAuth0(args[2:], stdout, stderr, time.Now)
	default:
		printUsage(stderr)
		return 2
	}
	if err == nil {
		return 0
	}

	fmt.Fprintf(stderr, "rsp-migrate: %v\n", err)
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	if errors.Is(err, legacy.ErrBlockingAnomalies) ||
		errors.Is(err, legacy.ErrSourceDrift) ||
		errors.Is(err, legacy.ErrManifestChecksum) ||
		errors.Is(err, legacy.ErrResolutionChecksum) ||
		errors.Is(err, legacy.ErrResolutionBinding) {
		return 3
	}
	return 1
}

func planAuth0(args []string, stdout, stderr io.Writer, now func() time.Time) error {
	set := flag.NewFlagSet("auth0 plan", flag.ContinueOnError)
	set.SetOutput(stderr)
	usersPath := set.String("auth0-users", "", "normalized Auth0 users JSON (required)")
	candidatesPath := set.String("app-candidates", "", "imported app identity candidates JSON (required)")
	resolutionsPath := set.String("resolutions", "", "explicit identity resolutions JSON")
	planPath := set.String("plan", "auth0-import-plan.json", "output reconciliation plan")
	if err := set.Parse(args); err != nil {
		return err
	}
	if set.NArg() != 0 {
		return errors.New("auth0 plan does not accept positional arguments")
	}
	if *usersPath == "" || *candidatesPath == "" {
		return errors.New("--auth0-users and --app-candidates are required")
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
	plan, err := legacy.PlanAuth0Import(users, candidates, resolutions, now(), nil)
	if err != nil {
		return err
	}
	if err := legacy.WriteJSON(*planPath, plan); err != nil {
		return err
	}

	fmt.Fprintf(stdout, "plan=%s auth0Users=%d providerIdentities=%d matched=%d requiresResolution=%d passwordResetRequired=%d checksum=%s\n",
		*planPath,
		plan.Reconciliation.Auth0UserCount,
		plan.Reconciliation.ProviderIdentityCount,
		plan.Reconciliation.StatusCounts[legacy.Auth0StatusMatched],
		plan.Reconciliation.StatusCounts[legacy.Auth0StatusRequiresResolution],
		plan.Reconciliation.StatusCounts[legacy.Auth0StatusPasswordResetRequired],
		plan.Checksum,
	)
	return nil
}

func loadJSON(path, label string, target any) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open %s: %w", label, err)
	}

	defer file.Close()
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode %s: %w", label, err)
	}

	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("decode %s: multiple JSON documents", label)
		}
		return fmt.Errorf("decode %s: %w", label, err)
	}
	return nil
}

type sourceFlags struct {
	fixture string
	dsn     string
}

func (flags *sourceFlags) register(set *flag.FlagSet) {
	set.StringVar(&flags.fixture, "source-fixture", "", "legacy JSON snapshot fixture")
	set.StringVar(&flags.dsn, "source-dsn", os.Getenv("LEGACY_DATABASE_URL"), "legacy PostgreSQL DSN (default LEGACY_DATABASE_URL)")
}

func (flags sourceFlags) source() (legacy.Source, error) {
	if flags.fixture != "" {
		snapshot, err := legacy.LoadSnapshot(flags.fixture)
		if err != nil {
			return nil, err
		}
		return &legacy.FixtureSource{Value: snapshot}, nil
	}
	if flags.dsn != "" {
		return &legacy.PostgresSource{ConnectionString: flags.dsn}, nil
	}
	return nil, errors.New("--source-fixture or --source-dsn/LEGACY_DATABASE_URL is required")
}

type targetFlags struct {
	state string
	dsn   string
}

func (flags *targetFlags) register(set *flag.FlagSet) {
	set.StringVar(&flags.state, "target-state", "", "durable fixture target state (rehearsal only)")
	set.StringVar(&flags.dsn, "target-dsn", os.Getenv("DATABASE_URL"), "target PostgreSQL DSN (default DATABASE_URL)")
}

func (flags targetFlags) target() (legacy.Target, error) {
	if flags.state != "" {
		return &legacy.StateTarget{Path: flags.state}, nil
	}
	if flags.dsn != "" {
		return &legacy.PostgresTarget{ConnectionString: flags.dsn}, nil
	}
	return nil, errors.New("--target-state or --target-dsn/DATABASE_URL is required")
}

func dryRun(ctx context.Context, engine *legacy.Engine, args []string, stdout, stderr io.Writer) error {
	set := flag.NewFlagSet("legacy dry-run", flag.ContinueOnError)
	set.SetOutput(stderr)
	var sourceOptions sourceFlags
	sourceOptions.register(set)
	manifestPath := set.String("manifest", "migration-manifest.json", "output manifest path")
	resolutionPath := set.String("resolution", "", "checksum-bound resolution JSON")
	if err := set.Parse(args); err != nil {
		return err
	}

	source, err := sourceOptions.source()
	if err != nil {
		return err
	}

	resolutions, err := legacy.LoadResolutionFile(*resolutionPath)
	if err != nil {
		return err
	}

	prepared, planErr := engine.DryRun(ctx, source, resolutions)
	if prepared.Manifest.Version != 0 {
		if err := legacy.WriteJSON(*manifestPath, prepared.Manifest); err != nil {
			return err
		}

		fmt.Fprintf(stdout, "manifest=%s runId=%s sourceTables=%d mappings=%d anomalies=%d autoFixes=%d checksum=%s\n",
			*manifestPath, prepared.Manifest.RunID, len(prepared.Manifest.Tables),
			len(prepared.Manifest.Mappings), len(prepared.Manifest.Anomalies),
			len(prepared.Manifest.AutoFixes), prepared.Manifest.Checksum)
	}
	return planErr
}

func apply(ctx context.Context, engine *legacy.Engine, args []string, stdout, stderr io.Writer) error {
	set := flag.NewFlagSet("legacy apply", flag.ContinueOnError)
	set.SetOutput(stderr)
	var sourceOptions sourceFlags
	var targetOptions targetFlags
	sourceOptions.register(set)
	targetOptions.register(set)
	manifestPath := set.String("manifest", "", "approved manifest JSON (required)")
	resolutionPath := set.String("resolution", "", "checksum-bound resolution JSON")
	if err := set.Parse(args); err != nil {
		return err
	}
	if *manifestPath == "" {
		return errors.New("--manifest is required")
	}
	source, err := sourceOptions.source()
	if err != nil {
		return err
	}

	target, err := targetOptions.target()
	if err != nil {
		return err
	}

	manifest, err := legacy.LoadManifest(*manifestPath)
	if err != nil {
		return err
	}

	resolutions, err := legacy.LoadResolutionFile(*resolutionPath)
	if err != nil {
		return err
	}
	if err := engine.Apply(ctx, source, target, manifest, resolutions); err != nil {
		return err
	}

	fmt.Fprintf(stdout, "applied runId=%s manifestChecksum=%s\n", manifest.RunID, manifest.Checksum)
	return nil
}

func verify(ctx context.Context, engine *legacy.Engine, args []string, stdout, stderr io.Writer) error {
	set := flag.NewFlagSet("legacy verify", flag.ContinueOnError)
	set.SetOutput(stderr)
	var targetOptions targetFlags
	targetOptions.register(set)
	manifestPath := set.String("manifest", "", "applied manifest JSON (required)")
	if err := set.Parse(args); err != nil {
		return err
	}
	if *manifestPath == "" {
		return errors.New("--manifest is required")
	}
	target, err := targetOptions.target()
	if err != nil {
		return err
	}

	manifest, err := legacy.LoadManifest(*manifestPath)
	if err != nil {
		return err
	}

	verification, err := engine.Verify(ctx, target, manifest)
	if err != nil {
		return err
	}

	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(verification)
}

func printUsage(writer io.Writer) {
	fmt.Fprintln(writer, "usage:")
	fmt.Fprintln(writer, "  rsp-migrate auth0 plan --auth0-users FILE --app-candidates FILE [--resolutions FILE] --plan FILE")
	fmt.Fprintln(writer, "  rsp-migrate legacy dry-run --source-fixture FILE|--source-dsn DSN --manifest FILE [--resolution FILE]")
	fmt.Fprintln(writer, "  rsp-migrate legacy apply --source-fixture FILE|--source-dsn DSN --manifest FILE --target-state FILE|--target-dsn DSN [--resolution FILE]")
	fmt.Fprintln(writer, "  rsp-migrate legacy verify --manifest FILE --target-state FILE|--target-dsn DSN")
}
