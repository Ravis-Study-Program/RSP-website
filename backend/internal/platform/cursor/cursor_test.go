package cursor

import "testing"

func TestCursorIsBoundAndTamperProof(t *testing.T) {
	secret := []byte("test-secret")
	v, err := Encode(secret, "row-42", "sort=createdAt&role=student")
	if err != nil {
		t.Fatal(err)
	}

	got, err := Decode(secret, v, "sort=createdAt&role=student")
	if err != nil || got != "row-42" {
		t.Fatalf("got %q, %v", got, err)
	}
	if _, err := Decode(secret, v, "sort=name"); err == nil {
		t.Fatal("accepted different filters")
	}
	if _, err := Decode(secret, v+"x", "sort=createdAt&role=student"); err == nil {
		t.Fatal("accepted tampering")
	}
}
