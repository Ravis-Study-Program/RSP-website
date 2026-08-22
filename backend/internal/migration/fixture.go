package migration

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// DecodeSnapshot decodes a value.
func DecodeSnapshot(reader io.Reader) (Snapshot, error) {
	decoder := json.NewDecoder(reader)
	decoder.UseNumber()
	var snapshot Snapshot
	if err := decoder.Decode(&snapshot); err != nil {
		return Snapshot{}, fmt.Errorf("decode source fixture: %w", err)
	}
	if snapshot.Tables == nil {
		snapshot.Tables = make(map[string][]Row)
	}
	return snapshot, nil
}

// LoadSnapshot loads a value.
func LoadSnapshot(path string) (Snapshot, error) {
	file, err := os.Open(path)
	if err != nil {
		return Snapshot{}, fmt.Errorf("open source fixture: %w", err)
	}

	defer file.Close()
	return DecodeSnapshot(file)
}

// WriteJSON writes a response.
func WriteJSON(path string, value any) error {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
	}

	encoded = append(encoded, '\n')
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// LoadManifest loads a value.
func LoadManifest(path string) (Manifest, error) {
	file, err := os.Open(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("open manifest: %w", err)
	}

	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.UseNumber()
	var manifest Manifest
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode manifest: %w", err)
	}
	return manifest, nil
}

// LoadResolutionFile loads a value.
func LoadResolutionFile(path string) (*ResolutionFile, error) {
	if path == "" {
		return nil, nil
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open resolution file: %w", err)
	}

	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.UseNumber()
	var result ResolutionFile
	if err := decoder.Decode(&result); err != nil {
		return nil, fmt.Errorf("decode resolution file: %w", err)
	}
	return &result, nil
}

// FixtureSource represents a backend data structure.
type FixtureSource struct {
	Value       Snapshot
	LastOptions SnapshotOptions
}

// Snapshot performs the operation.
func (f *FixtureSource) Snapshot(_ context.Context, options SnapshotOptions) (Snapshot, error) {
	f.LastOptions = options
	return cloneSnapshot(f.Value)
}

func cloneSnapshot(snapshot Snapshot) (Snapshot, error) {
	encoded, err := CanonicalJSON(snapshot)
	if err != nil {
		return Snapshot{}, err
	}
	return DecodeSnapshotBytes(encoded)
}

// DecodeSnapshotBytes decodes a value.
func DecodeSnapshotBytes(encoded []byte) (Snapshot, error) {
	var snapshot Snapshot
	decoder := json.NewDecoder(bytesReader(encoded))
	decoder.UseNumber()
	if err := decoder.Decode(&snapshot); err != nil {
		return Snapshot{}, err
	}
	return snapshot, nil
}

type byteReader struct {
	data []byte
	off  int
}

func bytesReader(data []byte) *byteReader { return &byteReader{data: data} }

// Read reads data.
func (r *byteReader) Read(target []byte) (int, error) {
	if r.off >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(target, r.data[r.off:])
	r.off += n
	return n, nil
}
