// Package id generates backend identifiers.
package id

import (
	"crypto/rand"
	"encoding/hex"
	"io"
	"time"
)

func New() string { return NewAt(time.Now(), rand.Reader) }

func NewAt(at time.Time, source io.Reader) string {
	var raw [16]byte
	_, _ = io.ReadFull(source, raw[:])
	millis := uint64(at.UTC().UnixMilli())
	for i := 5; i >= 0; i-- {
		raw[i] = byte(millis)
		millis >>= 8
	}
	raw[6] = (raw[6] & 0x0f) | 0x70
	raw[8] = (raw[8] & 0x3f) | 0x80
	var out [36]byte
	hex.Encode(out[0:8], raw[0:4])
	out[8] = '-'
	hex.Encode(out[9:13], raw[4:6])
	out[13] = '-'
	hex.Encode(out[14:18], raw[6:8])
	out[18] = '-'
	hex.Encode(out[19:23], raw[8:10])
	out[23] = '-'
	hex.Encode(out[24:36], raw[10:16])
	return string(out[:])
}
