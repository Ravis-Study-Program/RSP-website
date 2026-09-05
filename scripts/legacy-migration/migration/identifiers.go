package migration

import (
	"encoding/hex"
	"io"
	"time"
)

func newMigrationIDAt(at time.Time, source io.Reader) string {
	var raw [16]byte
	_, _ = io.ReadFull(source, raw[:])
	millis := uint64(at.UTC().UnixMilli())
	for index := 5; index >= 0; index-- {
		raw[index] = byte(millis)
		millis >>= 8
	}
	raw[6] = (raw[6] & 0x0f) | 0x70
	raw[8] = (raw[8] & 0x3f) | 0x80
	var output [36]byte
	hex.Encode(output[0:8], raw[0:4])
	output[8] = '-'
	hex.Encode(output[9:13], raw[4:6])
	output[13] = '-'
	hex.Encode(output[14:18], raw[6:8])
	output[18] = '-'
	hex.Encode(output[19:23], raw[8:10])
	output[23] = '-'
	hex.Encode(output[24:36], raw[10:16])
	return string(output[:])
}
