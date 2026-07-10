package safety

import (
	"log/slog"
	"strings"
	"sync"
	"time"
)

// Provider no-train / no-retain policy identity (audit evidence stub).
const (
	ProviderNoTrainPolicyID      = "provider-no-train-v1"
	ProviderNoTrainPolicySummary = "images_ephemeral_no_train_no_retain"
)

// PolicyAuditEntry is an in-memory audit proof of the no-train policy assertion.
// Production deployments should sink this to durable audit storage; the stub
// keeps a bounded ring buffer for tests and local verification.
type PolicyAuditEntry struct {
	At            time.Time `json:"at"`
	PolicyID      string    `json:"policy_id"`
	PolicySummary string    `json:"policy_summary"`
	Provider      string    `json:"provider"` // coarse: vision|llm — never secrets
	Purpose       string    `json:"purpose"`  // detect|analyze|value
	ModelHint     string    `json:"model_hint,omitempty"`
	RetainImage   bool      `json:"retain_image"`
	AllowTrain    bool      `json:"allow_train"`
	InputDigest   string    `json:"input_digest,omitempty"` // sha256 hex only
	RequestID     string    `json:"request_id,omitempty"`
	DeviceID      string    `json:"device_id,omitempty"`
}

const policyRingMax = 256

var (
	policyMu   sync.Mutex
	policyRing []PolicyAuditEntry
)

// LogProviderNoTrain records that a provider call was made under the no-train
// / no-retain policy. Never pass image bytes or raw model prompts here.
func LogProviderNoTrain(provider, purpose, modelHint, inputDigest, deviceID, requestID string) PolicyAuditEntry {
	entry := PolicyAuditEntry{
		At:            time.Now().UTC(),
		PolicyID:      ProviderNoTrainPolicyID,
		PolicySummary: ProviderNoTrainPolicySummary,
		Provider:      sanitizeProvider(provider),
		Purpose:       purpose,
		ModelHint:     truncateHint(modelHint, 64),
		RetainImage:   false,
		AllowTrain:    false,
		InputDigest:   inputDigest,
		RequestID:     requestID,
		DeviceID:      deviceID,
	}
	policyMu.Lock()
	policyRing = append(policyRing, entry)
	if len(policyRing) > policyRingMax {
		policyRing = policyRing[len(policyRing)-policyRingMax:]
	}
	policyMu.Unlock()

	slog.Info("provider_policy_audit",
		"policy_id", entry.PolicyID,
		"policy", entry.PolicySummary,
		"provider", entry.Provider,
		"purpose", entry.Purpose,
		"model_hint", entry.ModelHint,
		"retain_image", false,
		"allow_train", false,
		"input_digest", entry.InputDigest,
		"device_id", entry.DeviceID,
		// deliberately omit image bytes / base64 / prompts
	)
	return entry
}

// RecentPolicyAudits returns a copy of recent policy audit entries (newest last).
func RecentPolicyAudits() []PolicyAuditEntry {
	policyMu.Lock()
	defer policyMu.Unlock()
	out := make([]PolicyAuditEntry, len(policyRing))
	copy(out, policyRing)
	return out
}

// ResetPolicyAudits clears the ring buffer (tests only).
func ResetPolicyAudits() {
	policyMu.Lock()
	policyRing = nil
	policyMu.Unlock()
}

func sanitizeProvider(p string) string {
	p = strings.ToLower(strings.TrimSpace(p))
	switch p {
	case "vision", "vlm", "llm", "text":
		return p
	default:
		if p == "" {
			return "unknown"
		}
		return "provider"
	}
}

func truncateHint(s string, n int) string {
	if n <= 0 || len(s) <= n {
		return s
	}
	return s[:n]
}
