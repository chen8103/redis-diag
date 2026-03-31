package config

import "testing"

func TestResolveTargetFromURIHidesCredentialsInDisplay(t *testing.T) {
	cfg := Default()
	cfg.Target.URI = "rediss://user:pass@example.com:6379/1"

	resolved, err := ResolveTarget(cfg)
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}
	if resolved.Addr != "example.com:6379" {
		t.Fatalf("unexpected addr %q", resolved.Addr)
	}
	if resolved.Display != "example.com:6379" {
		t.Fatalf("display should not include credentials, got %q", resolved.Display)
	}
	if resolved.Username != "user" || resolved.Password != "pass" {
		t.Fatalf("expected URI credentials to be preserved")
	}
	if !resolved.UseTLS {
		t.Fatalf("expected rediss URI to enable tls")
	}
}
