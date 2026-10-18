// Package main provides administrative commands for the RSP backend.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/mail"
	"os"
	"strings"

	"github.com/magedmg/RSP-website/backend/internal/dal"
)

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "rspctl:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: rspctl seed|bootstrap-admin")
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return errors.New("DATABASE_URL is required")
	}

	repository, err := dal.Open(ctx, dsn)
	if err != nil {
		return err
	}

	defer repository.Close()
	switch args[0] {
	case "seed":
		appEnv := strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV")))
		if appEnv != "development" && appEnv != "test" {
			return errors.New("seed is allowed only when APP_ENV=development or APP_ENV=test")
		}
		options, err := parseSeedOptions(args[1:])
		if err != nil {
			return err
		}
		return repository.Seed(ctx, options)
	case "bootstrap-admin":
		set := flag.NewFlagSet("bootstrap-admin", flag.ContinueOnError)
		subject := set.String("auth-subject", "", "Better Auth user id")
		email := set.String("email", "", "verified administrator email")
		if err := set.Parse(args[1:]); err != nil {
			return err
		}
		return repository.BootstrapAdmin(ctx, *subject, *email)
	default:
		return errors.New("usage: rspctl seed|bootstrap-admin")
	}
}

func parseSeedOptions(args []string) (dal.SeedOptions, error) {
	set := flag.NewFlagSet("seed", flag.ContinueOnError)
	var options dal.SeedOptions
	set.StringVar(&options.StudentEmail, "student-email", "", "existing verified Better Auth user to enrol as the development student")
	set.StringVar(&options.MentorEmail, "mentor-email", "", "existing verified Better Auth user to enrol as the development mentor")
	set.StringVar(&options.CoordinatorEmail, "coordinator-email", "", "existing verified Better Auth user to enrol as the development coordinator")
	set.StringVar(&options.DirectorEmail, "director-email", "", "existing verified Better Auth user to grant the development Director role")
	set.StringVar(&options.SiteAdminEmail, "site-admin-email", "", "existing verified Better Auth user to grant the development Site Admin role")
	set.BoolVar(&options.AllowExisting, "allow-existing", false, "succeed when the development season already exists")
	if err := set.Parse(args); err != nil {
		return dal.SeedOptions{}, err
	}
	if set.NArg() != 0 {
		return dal.SeedOptions{}, errors.New("seed does not accept positional arguments")
	}

	values := []*string{
		&options.StudentEmail,
		&options.MentorEmail,
		&options.CoordinatorEmail,
		&options.DirectorEmail,
		&options.SiteAdminEmail,
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		normalized, err := normalizeSeedEmail(*value)
		if err != nil {
			return dal.SeedOptions{}, err
		}

		*value = normalized
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			return dal.SeedOptions{}, fmt.Errorf("seed role emails must be distinct: %s", normalized)
		}
		seen[normalized] = struct{}{}
	}

	if options.DirectorEmail != "" && options.SiteAdminEmail != "" {
		return dal.SeedOptions{}, errors.New("seed accepts either --director-email or --site-admin-email, not both")
	}
	return options, nil
}

func normalizeSeedEmail(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "", nil
	}

	parsed, err := mail.ParseAddress(value)
	if err != nil || !strings.EqualFold(parsed.Address, value) {
		return "", fmt.Errorf("invalid seed email %q", value)
	}

	return value, nil
}
