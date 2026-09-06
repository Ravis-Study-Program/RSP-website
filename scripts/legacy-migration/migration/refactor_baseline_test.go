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
	const expected = "99cbd186729a54f8b22d50e993a6ffa1b8f80730c7d8e6f959cc812b066f216e"
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
