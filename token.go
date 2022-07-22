package flow

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// GenerateToken returns a sha256 of the specified data
func GenerateToken(v ...string) string {
	data := fmt.Sprintf("%s", time.Now())
	for _, x := range v {
		data += x
	}
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}
