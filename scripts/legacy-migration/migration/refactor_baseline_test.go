package migration

import "testing"

func TestPreparedImportPreservesRecordsAndManifest(t *testing.T) {
	prepared, err := NewPlanner(fixedNow).Plan(validSnapshot(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	checksum, err := Checksum(prepared)
	if err != nil {
		t.Fatal(err)
	}
	// Captured before the readability refactor, including every transformed row.
	const expected = "b0740a2a67c4a4a6462e28b5f62a4f4464a835041462c0611ce517149064b6c6"
	if checksum != expected {
		t.Fatalf("import output changed: %s", checksum)
	}
	for _, table := range prepared.Tables {
		for _, row := range table.Rows {
			checksum, err := Checksum(row.Values)
			if err != nil {
				t.Fatal(err)
			}
			for _, provenance := range prepared.Provenance {
				if provenance.TargetTable == table.Name && provenance.TargetID == row.ID && provenance.TransformedChecksum != checksum {
					t.Fatalf("%s/%s provenance checksum does not match final row", table.Name, row.ID)
				}
			}
		}
	}
}
