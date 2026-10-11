package usage

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/suho-han/one-click-ai-tools/internal/netclient"
)

// CodexResetCredit is one banked rate-limit reset credit from the wham
// rate-limit-reset-credits listing. Status "available" means it can still
// be spent; expires_at is when the grant lapses.
type CodexResetCredit struct {
	ID          string `json:"id"`
	Status      string `json:"status"`
	Title       string `json:"title"`
	Description string `json:"description"`
	GrantedAt   string `json:"granted_at"`
	ExpiresAt   string `json:"expires_at"`
}

// CodexResetCredits is the wham rate-limit-reset-credits listing payload.
type CodexResetCredits struct {
	Credits        []CodexResetCredit `json:"credits"`
	AvailableCount int                `json:"available_count"`
}

// CodexConsumeResult is the response to spending one reset credit. The
// upstream reply carries code "reset" on success; windows_reset counts the
// rate-limit windows that were cleared.
type CodexConsumeResult struct {
	Code            string `json:"code"`
	WindowsReset    int    `json:"windows_reset"`
	RedeemRequestID string `json:"redeem_request_id"`
}

// codexWhamEndpoint resolves a wham API path. OCT_CODEX_USAGE_ENDPOINT keeps
// its documented meaning (the exact usage URL), so the credit endpoints strip
// its "/usage" suffix — which also lets one httptest server host every codex
// endpoint in tests.
func codexWhamEndpoint(path string) string {
	base := strings.TrimSpace(os.Getenv("OCT_CODEX_USAGE_ENDPOINT"))
	if base == "" {
		return "https://chatgpt.com/backend-api/wham" + path
	}
	return strings.TrimSuffix(base, "/usage") + path
}

// FetchCodexResetCredits lists the rate-limit reset credits banked on one
// codex account. An empty home resolves the default codex home chain.
func FetchCodexResetCredits(ctx context.Context, home string) (CodexResetCredits, error) {
	auth, ok := readCodexBackendAuthFromHome(home)
	if !ok {
		return CodexResetCredits{}, fmt.Errorf("codex auth unavailable; run 'codex login'")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, codexWhamEndpoint("/rate-limit-reset-credits"), nil)
	if err != nil {
		return CodexResetCredits{}, fmt.Errorf("build reset-credits request failed: %w", err)
	}
	setCodexWhamHeaders(req, auth, "")

	resp, err := netclient.DefaultClient.DoWithRetry(req)
	if err != nil {
		return CodexResetCredits{}, fmt.Errorf("reset-credits request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := readAllCapped(resp.Body)
	if err != nil {
		return CodexResetCredits{}, fmt.Errorf("read reset-credits response failed: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return CodexResetCredits{}, fmt.Errorf("reset-credits request returned %s: %s", resp.Status, truncateText(string(body), 200))
	}
	var credits CodexResetCredits
	if err := json.Unmarshal(body, &credits); err != nil {
		return CodexResetCredits{}, fmt.Errorf("parse reset-credits response failed: %w", err)
	}
	return credits, nil
}

// ConsumeCodexResetCredit spends one banked reset credit, clearing the
// account's rate-limit windows. redeemRequestID pins the redemption for
// idempotent retries (UUID v4); empty generates one. The POST is sent once —
// a timeout retry could double-spend a credit, so the caller retries only
// with the same redeemRequestID, which the backend deduplicates.
func ConsumeCodexResetCredit(ctx context.Context, home, redeemRequestID string) (CodexConsumeResult, error) {
	auth, ok := readCodexBackendAuthFromHome(home)
	if !ok {
		return CodexConsumeResult{}, fmt.Errorf("codex auth unavailable; run 'codex login'")
	}
	if strings.TrimSpace(redeemRequestID) == "" {
		redeemRequestID = newRedeemRequestID()
	}

	payload, err := json.Marshal(map[string]string{"redeem_request_id": redeemRequestID})
	if err != nil {
		return CodexConsumeResult{}, fmt.Errorf("encode consume payload failed: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, codexWhamEndpoint("/rate-limit-reset-credits/consume"), bytes.NewReader(payload))
	if err != nil {
		return CodexConsumeResult{}, fmt.Errorf("build consume request failed: %w", err)
	}
	setCodexWhamHeaders(req, auth, "application/json")

	// Single attempt on purpose: a retry after an ambiguous timeout could
	// spend a second credit before the first response lands.
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return CodexConsumeResult{}, fmt.Errorf("consume request failed (credit not spent unless it actually landed; re-check available credits): %w", err)
	}
	defer resp.Body.Close()

	body, err := readAllCapped(resp.Body)
	if err != nil {
		return CodexConsumeResult{}, fmt.Errorf("read consume response failed: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return CodexConsumeResult{}, fmt.Errorf("consume request returned %s: %s", resp.Status, truncateText(string(body), 200))
	}
	var result CodexConsumeResult
	if err := json.Unmarshal(body, &result); err != nil {
		return CodexConsumeResult{}, fmt.Errorf("parse consume response failed: %w", err)
	}
	result.RedeemRequestID = redeemRequestID
	return result, nil
}

// setCodexWhamHeaders mirrors the headers fetchCodexBackendUsage sends so
// the credit endpoints see the same client identity.
func setCodexWhamHeaders(req *http.Request, auth codexBackendAuth, contentType string) {
	req.Header.Set("Authorization", "Bearer "+auth.AccessToken)
	req.Header.Set("User-Agent", "codex-cli")
	req.Header.Set("OpenAI-Beta", "codex-1")
	req.Header.Set("originator", "Codex Desktop")
	req.Header.Set("Accept", "application/json")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if auth.AccountID != "" {
		req.Header.Set("ChatGPT-Account-Id", auth.AccountID)
	}
}

// newRedeemRequestID mints a random UUID v4 without a dependency.
func newRedeemRequestID() string {
	var b [16]byte
	if _, err := io.ReadFull(rand.Reader, b[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
