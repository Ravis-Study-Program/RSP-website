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

	legacy "github.com/magedmg/RSP-website/backend/internal/migration"
)

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) < 2 || args[0] != "legacy" {
		printUsage(stderr)
		return 2
	}
	engine := legacy.NewEngine(time.Now)
	var err error
	switch args[1] {
	case "dry-run":
		err = dryRun(ctx, engine, args[2:], stdout, stderr)
	case "apply":
		err = apply(ctx, engine, args[2:], stdout, stderr)
	case "verify":
		err = verify(ctx, engine, args[2:], stdout, stderr)
	case "rollback":
		err = rollback(ctx, engine, args[2:], stdout, stderr)
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

func rollback(ctx context.Context, engine *legacy.Engine, args []string, stdout, stderr io.Writer) error {
	set := flag.NewFlagSet("legacy rollback", flag.ContinueOnError)
	set.SetOutput(stderr)
	var targetOptions targetFlags
	targetOptions.register(set)
	runID := set.String("run-id", "", "migration run ID (required)")
	if err := set.Parse(args); err != nil {
		return err
	}
	if *runID == "" {
		return errors.New("--run-id is required")
	}
	target, err := targetOptions.target()
	if err != nil {
		return err
	}
	if err := engine.Rollback(ctx, target, *runID); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "rolledBack runId=%s\n", *runID)
	return nil
}

func printUsage(writer io.Writer) {
	fmt.Fprintln(writer, "usage:")
	fmt.Fprintln(writer, "  rsp-migrate legacy dry-run --source-fixture FILE|--source-dsn DSN --manifest FILE [--resolution FILE]")
	fmt.Fprintln(writer, "  rsp-migrate legacy apply --source-fixture FILE|--source-dsn DSN --manifest FILE --target-state FILE|--target-dsn DSN [--resolution FILE]")
	fmt.Fprintln(writer, "  rsp-migrate legacy verify --manifest FILE --target-state FILE|--target-dsn DSN")
	fmt.Fprintln(writer, "  rsp-migrate legacy rollback --run-id ID --target-state FILE|--target-dsn DSN")
}
