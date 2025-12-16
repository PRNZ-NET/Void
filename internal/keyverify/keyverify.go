package keyverify

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

func ComputeKeyFingerprint(publicKey *[32]byte) string {
	hash := sha256.Sum256(publicKey[:])
	return hex.EncodeToString(hash[:])[:16]
}

func VerifyKeyFingerprint(publicKey *[32]byte, expectedFingerprint string) error {
	fingerprint := ComputeKeyFingerprint(publicKey)
	if fingerprint != expectedFingerprint {
		return fmt.Errorf("key fingerprint mismatch: expected %s, got %s", expectedFingerprint, fingerprint)
	}
	return nil
}

