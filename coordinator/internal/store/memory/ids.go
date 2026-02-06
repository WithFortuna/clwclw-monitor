package memory

import (
	"crypto/rand"
	"encoding/hex"
)

func newID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])

	// UUIDv4 (RFC 4122)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80

	hexStr := hex.EncodeToString(b[:])
	// 8-4-4-4-12
	return hexStr[0:8] + "-" + hexStr[8:12] + "-" + hexStr[12:16] + "-" + hexStr[16:20] + "-" + hexStr[20:32]
}
