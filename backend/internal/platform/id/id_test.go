package id

import (
	"bytes"
	"testing"
	"time"
)

func TestUUIDv7Format(t *testing.T) {
	got := NewAt(time.UnixMilli(0x010203040506), bytes.NewReader(make([]byte, 16)))
	if got != "01020304-0506-7000-8000-000000000000" {
		t.Fatalf("got %s", got)
	}
}
