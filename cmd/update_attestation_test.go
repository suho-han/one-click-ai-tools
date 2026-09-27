package cmd

import (
	"context"
	"errors"
	"testing"
)

func TestVerifyReleaseAttestationSkipsWhenGhMissing(t *testing.T) {
	origLookup, origRun := attestationGhLookup, runAttestationVerify
	t.Cleanup(func() { attestationGhLookup, runAttestationVerify = origLookup, origRun })

	attestationGhLookup = func() (string, error) { return "", errors.New("not found") }
	runAttestationVerify = func(ctx context.Context, ghPath, artifactPath, repo string) error {
		t.Fatal("verification must not run without gh")
		return nil
	}
	t.Setenv("OCT_UPDATE_REQUIRE_ATTESTATION", "")

	if err := verifyReleaseAttestation(context.Background(), "suho-han/one-click-ai-tools", "/tmp/asset.tar.gz"); err != nil {
		t.Fatalf("expected skip without gh, got error: %v", err)
	}
}

func TestVerifyReleaseAttestationFailsWhenGhMissingAndRequired(t *testing.T) {
	origLookup := attestationGhLookup
	t.Cleanup(func() { attestationGhLookup = origLookup })

	attestationGhLookup = func() (string, error) { return "", errors.New("not found") }
	t.Setenv("OCT_UPDATE_REQUIRE_ATTESTATION", "1")

	err := verifyReleaseAttestation(context.Background(), "suho-han/one-click-ai-tools", "/tmp/asset.tar.gz")
	if err == nil {
		t.Fatalf("expected error when attestation is required but gh is missing")
	}
}

func TestVerifyReleaseAttestationSkipsWhenCLITooOld(t *testing.T) {
	origLookup, origRun := attestationGhLookup, runAttestationVerify
	t.Cleanup(func() { attestationGhLookup, runAttestationVerify = origLookup, origRun })

	attestationGhLookup = func() (string, error) { return "/usr/bin/gh", nil }
	runAttestationVerify = func(ctx context.Context, ghPath, artifactPath, repo string) error {
		return errAttestationUnsupported
	}
	t.Setenv("OCT_UPDATE_REQUIRE_ATTESTATION", "")

	if err := verifyReleaseAttestation(context.Background(), "suho-han/one-click-ai-tools", "/tmp/asset.tar.gz"); err != nil {
		t.Fatalf("expected skip for old gh, got error: %v", err)
	}
}

func TestVerifyReleaseAttestationFailsOnVerificationFailure(t *testing.T) {
	origLookup, origRun := attestationGhLookup, runAttestationVerify
	t.Cleanup(func() { attestationGhLookup, runAttestationVerify = origLookup, origRun })

	attestationGhLookup = func() (string, error) { return "/usr/bin/gh", nil }
	runAttestationVerify = func(ctx context.Context, ghPath, artifactPath, repo string) error {
		return errors.New("no attestations found")
	}
	t.Setenv("OCT_UPDATE_REQUIRE_ATTESTATION", "")
	t.Setenv("OCT_UPDATE_SKIP_ATTESTATION", "")

	if err := verifyReleaseAttestation(context.Background(), "suho-han/one-click-ai-tools", "/tmp/asset.tar.gz"); err == nil {
		t.Fatalf("expected verification failure to abort the install")
	}
}

func TestVerifyReleaseAttestationSkipsWithoutSubprocessWhenOptedOut(t *testing.T) {
	origLookup, origRun := attestationGhLookup, runAttestationVerify
	t.Cleanup(func() { attestationGhLookup, runAttestationVerify = origLookup, origRun })

	attestationGhLookup = func() (string, error) {
		t.Fatal("gh lookup must not run when attestation is opted out")
		return "", nil
	}
	runAttestationVerify = func(ctx context.Context, ghPath, artifactPath, repo string) error {
		t.Fatal("verification must not run when opted out")
		return nil
	}
	t.Setenv("OCT_UPDATE_SKIP_ATTESTATION", "1")

	if err := verifyReleaseAttestation(context.Background(), "suho-han/one-click-ai-tools", "/tmp/asset.tar.gz"); err != nil {
		t.Fatalf("expected skip, got error: %v", err)
	}
}
