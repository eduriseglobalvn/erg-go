package password

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func TestHashVerifyArgon2ID(t *testing.T) {
	params := NormalizeParams(32*1024, 2)
	hash, err := Hash("correct horse battery staple", params)
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$") {
		t.Fatalf("hash prefix = %q", hash)
	}
	ok, needsRehash := Verify("correct horse battery staple", hash, params)
	if !ok {
		t.Fatal("Verify rejected valid password")
	}
	if needsRehash {
		t.Fatal("Verify unexpectedly requested rehash for current params")
	}
	ok, _ = Verify("wrong password", hash, params)
	if ok {
		t.Fatal("Verify accepted wrong password")
	}
}

func TestVerifyRequestsRehashForLegacyHashes(t *testing.T) {
	sum := sha256.Sum256([]byte("secret" + "salt_65536_3"))
	ok, needsRehash := Verify("secret", hex.EncodeToString(sum[:]), NormalizeParams(64*1024, 3))
	if !ok {
		t.Fatal("Verify rejected legacy salted SHA-256 hash")
	}
	if !needsRehash {
		t.Fatal("Verify did not request legacy rehash")
	}

	plain := sha256.Sum256([]byte("secret"))
	ok, needsRehash = Verify("secret", hex.EncodeToString(plain[:]), NormalizeParams(64*1024, 3))
	if !ok || !needsRehash {
		t.Fatal("Verify did not accept legacy plain SHA-256 with rehash")
	}

	ok, needsRehash = Verify("secret", "sha256:"+string(plain[:]), NormalizeParams(64*1024, 3))
	if !ok || !needsRehash {
		t.Fatal("Verify did not accept legacy binary SHA-256 with rehash")
	}
}

func TestVerifyRequestsRehashWhenParamsChange(t *testing.T) {
	oldParams := NormalizeParams(32*1024, 2)
	hash, err := Hash("secret", oldParams)
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	ok, needsRehash := Verify("secret", hash, NormalizeParams(64*1024, 3))
	if !ok {
		t.Fatal("Verify rejected valid old Argon2id hash")
	}
	if !needsRehash {
		t.Fatal("Verify did not request rehash after params changed")
	}
}
