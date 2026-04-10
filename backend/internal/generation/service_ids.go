package generation

import (
	"crypto/rand"
	"encoding/hex"
)

func newAssetID() string {
	buf := make([]byte, 10)
	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}
	return "asset_" + hex.EncodeToString(buf)
}
