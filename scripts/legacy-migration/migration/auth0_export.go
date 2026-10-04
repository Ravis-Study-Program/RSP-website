package migration

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
)

// Auth0 provider names the importer understands. Database users live under
// "auth0"; Google users under "google-oauth2". Anything else must be removed
// from the tenant or handled by a reviewed code change before cutover.
const (
	Auth0ProviderDatabase = "auth0"
	Auth0ProviderGoogle   = "google-oauth2"
)

// legacyUnknownUserID is the placeholder the legacy server wrote into
// app_metadata.userId for members it had just created.
const legacyUnknownUserID = "Unknown User"

type auth0ExportIdentity struct {
	Provider   string          `json:"provider"`
	UserID     json.RawMessage `json:"user_id"`
	Connection string          `json:"connection"`
}

type auth0ExportRecord struct {
	UserID        string                `json:"user_id"`
	Email         string                `json:"email"`
	EmailVerified bool                  `json:"email_verified"`
	Blocked       bool                  `json:"blocked"`
	Identities    []auth0ExportIdentity `json:"identities"`
	AppMetadata   struct {
		UserID string `json:"userId"`
	} `json:"app_metadata"`
}

// NormalizeAuth0Export reads an Auth0 users-exports job file and returns the
// planner's input shape. It accepts the gzip file Auth0 produces, its NDJSON
// content, or a JSON array of the same records.
func NormalizeAuth0Export(reader io.Reader) ([]Auth0User, error) {
	buffered := bufio.NewReader(reader)
	if magic, err := buffered.Peek(2); err == nil && magic[0] == 0x1f && magic[1] == 0x8b {
		decompressed, err := gzip.NewReader(buffered)
		if err != nil {
			return nil, fmt.Errorf("open gzip export: %w", err)
		}
		defer decompressed.Close()
		buffered = bufio.NewReader(decompressed)
	}
	records, err := decodeAuth0Records(buffered)
	if err != nil {
		return nil, err
	}

	users := make([]Auth0User, 0, len(records))
	seen := map[string]bool{}
	var unsupported []string
	for index, record := range records {
		user, err := normalizeAuth0Record(record)
		if err != nil {
			return nil, fmt.Errorf("export record %d: %w", index+1, err)
		}
		if seen[user.UserID] {
			return nil, fmt.Errorf("export contains Auth0 user %s more than once", user.UserID)
		}
		seen[user.UserID] = true
		for _, identity := range user.Identities {
			if identity.Provider != Auth0ProviderDatabase && identity.Provider != Auth0ProviderGoogle {
				unsupported = append(unsupported, user.UserID+" ("+identity.Provider+")")
			}
		}
		users = append(users, user)
	}
	if len(unsupported) > 0 {
		sort.Strings(unsupported)
		return nil, fmt.Errorf("unsupported Auth0 providers: %s", strings.Join(unsupported, ", "))
	}
	sort.Slice(users, func(left, right int) bool { return users[left].UserID < users[right].UserID })
	return users, nil
}

func decodeAuth0Records(reader *bufio.Reader) ([]auth0ExportRecord, error) {
	first, err := firstNonSpace(reader)
	if errors.Is(err, io.EOF) {
		return nil, errors.New("Auth0 export is empty")
	}
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(reader)
	if first == '[' {
		var records []auth0ExportRecord
		if err := decoder.Decode(&records); err != nil {
			return nil, fmt.Errorf("decode Auth0 export array: %w", err)
		}
		return records, nil
	}
	records := make([]auth0ExportRecord, 0)
	for {
		var record auth0ExportRecord
		err := decoder.Decode(&record)
		if errors.Is(err, io.EOF) {
			return records, nil
		}
		if err != nil {
			return nil, fmt.Errorf("decode Auth0 export record %d: %w", len(records)+1, err)
		}
		records = append(records, record)
	}
}

func firstNonSpace(reader *bufio.Reader) (byte, error) {
	for {
		value, err := reader.ReadByte()
		if err != nil {
			return 0, err
		}
		if !bytes.ContainsRune([]byte(" \t\r\n"), rune(value)) {
			return value, reader.UnreadByte()
		}
	}
}

func normalizeAuth0Record(record auth0ExportRecord) (Auth0User, error) {
	userID := strings.TrimSpace(record.UserID)
	provider, accountID, ok := strings.Cut(userID, "|")
	if !ok || provider == "" || accountID == "" {
		return Auth0User{}, fmt.Errorf("Auth0 user id %q is not provider|id", record.UserID)
	}
	// Auth0's user_id is always the primary identity. Linked secondary
	// identities only appear in the identities array, so the export must
	// request it; the primary is derived so an omitted array still fails safe
	// rather than dropping the user.
	primary := Auth0Identity{Provider: provider, ProviderAccountID: accountID}
	identities := []Auth0Identity{primary}
	if len(record.Identities) > 0 {
		identities = identities[:0]
		hasPrimary := false
		for _, raw := range record.Identities {
			id, err := identityAccountID(raw.UserID)
			if err != nil {
				return Auth0User{}, fmt.Errorf("Auth0 user %s: %w", userID, err)
			}
			identity := Auth0Identity{Provider: strings.TrimSpace(raw.Provider), ProviderAccountID: id}
			if identity == primary {
				hasPrimary = true
			}
			identities = append(identities, identity)
		}
		if !hasPrimary {
			return Auth0User{}, fmt.Errorf("Auth0 user %s identities do not include its primary identity", userID)
		}
	}

	legacyUserID := strings.TrimSpace(record.AppMetadata.UserID)
	if legacyUserID == legacyUnknownUserID {
		legacyUserID = ""
	}
	return Auth0User{
		UserID:        userID,
		Email:         strings.TrimSpace(record.Email),
		EmailVerified: record.EmailVerified,
		Blocked:       record.Blocked,
		LegacyUserID:  legacyUserID,
		Identities:    identities,
	}, nil
}

// identityAccountID accepts both forms Auth0 uses: most providers export a
// string, but some export a JSON number.
func identityAccountID(raw json.RawMessage) (string, error) {
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		if strings.TrimSpace(text) == "" {
			return "", errors.New("identity has an empty user_id")
		}
		return text, nil
	}
	var number json.Number
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&number); err != nil || number.String() == "" {
		return "", fmt.Errorf("identity user_id %s is neither a string nor a number", string(raw))
	}
	return number.String(), nil
}
