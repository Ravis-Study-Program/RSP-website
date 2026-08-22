package migration

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// CanonicalJSON uses encoding/json's deterministic lexical map-key ordering.
// All importer structures use JSON-compatible primitives and stable-sorted
// slices, producing the same bytes across repeated runs.
func CanonicalJSON(value any) ([]byte, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("canonical json: %w", err)
	}
	return encoded, nil
}

// Checksum checks a value.
func Checksum(value any) (string, error) {
	encoded, err := CanonicalJSON(value)
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

// SealManifest performs the operation.
func SealManifest(manifest *Manifest) error {
	manifest.Checksum = ""
	checksum, err := Checksum(manifest)
	if err != nil {
		return err
	}

	manifest.Checksum = checksum
	return nil
}

// ValidateManifest validates a value.
func ValidateManifest(manifest Manifest) error {
	if manifest.Version != ManifestVersion {
		return fmt.Errorf("%w: unsupported version %d", ErrManifestChecksum, manifest.Version)
	}
	want := manifest.Checksum
	manifest.Checksum = ""
	got, err := Checksum(manifest)
	if err != nil {
		return err
	}
	if want == "" || want != got {
		return fmt.Errorf("%w: expected %s, calculated %s", ErrManifestChecksum, want, got)
	}
	return nil
}

// SealResolutionFile performs the operation.
func SealResolutionFile(file *ResolutionFile) error {
	file.Checksum = ""
	checksum, err := Checksum(file)
	if err != nil {
		return err
	}

	file.Checksum = checksum
	return nil
}

// ValidateResolutionFile validates a value.
func ValidateResolutionFile(file ResolutionFile) error {
	if file.Version != ResolutionVersion {
		return fmt.Errorf("%w: unsupported version %d", ErrResolutionChecksum, file.Version)
	}
	want := file.Checksum
	file.Checksum = ""
	got, err := Checksum(file)
	if err != nil {
		return err
	}
	if want == "" || got != want {
		return fmt.Errorf("%w: expected %s, calculated %s", ErrResolutionChecksum, want, got)
	}
	return nil
}
