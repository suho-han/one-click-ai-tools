package analyticsserver

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"
)

const maxFeedbackBytes = 4000
const maxFeedbackBodyBytes = 32 * 1024

func (s *Service) ingestFeedback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.feedbackRate.allow() {
		http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
		return
	}
	invalid := func() { http.Error(w, "invalid feedback", http.StatusBadRequest) }
	if r.ContentLength > maxFeedbackBodyBytes || r.ContentLength == 0 || r.Header.Get("Content-Type") != "application/json" {
		invalid()
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxFeedbackBodyBytes+1))
	if err != nil || len(body) > maxFeedbackBodyBytes || !utf8.Valid(body) {
		invalid()
		return
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	if jsonValue(decoder) != nil {
		invalid()
		return
	}
	if _, err := decoder.Token(); err != io.EOF {
		invalid()
		return
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(body, &fields) != nil || len(fields) != 1 {
		invalid()
		return
	}
	raw, ok := fields["message"]
	var message string
	if !ok || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) || json.Unmarshal(raw, &message) != nil || !utf8.ValidString(message) || len(message) > maxFeedbackBytes || strings.TrimSpace(message) == "" {
		invalid()
		return
	}
	if _, err := s.db.Exec("INSERT INTO feedback(received_at,message) VALUES(?,?)", time.Now().UTC().Format(time.RFC3339), message); err != nil {
		http.Error(w, "storage unavailable", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}
