package apikey

import (
	"strings"
	"testing"
)

func TestGenerateProducesDistinctHighEntropyKeys(t *testing.T) {
	first, err := Generate()
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}
	second, err := Generate()
	if err != nil {
		t.Fatalf("Generate() second error: %v", err)
	}

	if !strings.HasPrefix(first, Prefix) {
		t.Fatalf("key must start with %q, got %q", Prefix, first)
	}
	if len(first) < 40 {
		t.Fatalf("generated key is unexpectedly short: %d", len(first))
	}
	if first == second {
		t.Fatal("two generated keys must not be equal")
	}
	if VisiblePrefix(first) == first {
		t.Fatal("visible prefix must not expose the full key")
	}
}

func TestHashIsStableAndDoesNotContainPlaintext(t *testing.T) {
	const key = "wrk_example-secret"
	got := Hash(key)
	if got != Hash(key) {
		t.Fatal("hash must be deterministic")
	}
	if len(got) != 64 {
		t.Fatalf("SHA-256 hex digest length = %d, want 64", len(got))
	}
	if strings.Contains(got, key) {
		t.Fatal("hash must not contain the plaintext key")
	}
}

func TestIsAPIKey(t *testing.T) {
	if !IsAPIKey("wrk_secret") {
		t.Fatal("wrk_ token must be recognized as API key")
	}
	if IsAPIKey("jwt.token.value") {
		t.Fatal("JWT must not be recognized as API key")
	}
}
