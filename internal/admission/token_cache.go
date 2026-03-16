package admission

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/zhouchenh/transitloom/internal/config"
)

type TokenCachePhase string

const (
	TokenCachePhaseMissing TokenCachePhase = "missing"
	TokenCachePhaseUsable  TokenCachePhase = "usable"
	TokenCachePhaseExpired TokenCachePhase = "expired"
)

// CachedTokenRecord is a placeholder representation of the locally cached
// current admission token. It is deliberately local-state oriented so this
// task does not freeze the final token format or pretend the cache is
// authoritative truth.
type CachedTokenRecord struct {
	TokenID             string    `json:"token_id"`
	NodeID              string    `json:"node_id"`
	IssuerCoordinatorID string    `json:"issuer_coordinator_id"`
	IssuedAt            time.Time `json:"issued_at"`
	ExpiresAt           time.Time `json:"expires_at"`
}

type TokenCacheStatus struct {
	ConfiguredPath string
	ResolvedPath   string
	Exists         bool
}

// TokenCacheState separates local cached token inspection from future live
// coordinator admission checks. A usable cached token is still only a local
// readiness signal for this task.
type TokenCacheState struct {
	Cache TokenCacheStatus
	Token *CachedTokenRecord
	Phase TokenCachePhase
}

func InspectTokenCache(cfg config.NodeConfig, now time.Time) (TokenCacheState, error) {
	var state TokenCacheState

	cache, err := inspectTokenCache(cfg.Storage.DataDir, cfg.Admission.CurrentTokenPath, "cached admission token")
	if err != nil {
		return state, fmt.Errorf("node admission bootstrap validation failed: %w", err)
	}
	state.Cache = cache
	if !state.Cache.Exists {
		state.Phase = TokenCachePhaseMissing
		return state, nil
	}

	token, err := loadCachedToken(state.Cache.ResolvedPath)
	if err != nil {
		return state, fmt.Errorf("node admission bootstrap validation failed: %w", err)
	}
	if token.ExpiresAt.After(now) {
		state.Phase = TokenCachePhaseUsable
	} else {
		state.Phase = TokenCachePhaseExpired
	}
	state.Token = &token

	return state, nil
}

func (s TokenCacheState) HasTokenFile() bool {
	return s.Cache.Exists
}

func (s TokenCacheState) HasUsableToken() bool {
	return s.Phase == TokenCachePhaseUsable
}

func (s TokenCacheState) ReportLines() []string {
	lines := []string{
		fmt.Sprintf("node admission bootstrap state: %s", describeTokenCachePhase(s)),
		fmt.Sprintf("node admission token cache: %s (%s)", s.Cache.DisplayPath(), s.Cache.Presence()),
	}

	if s.Token != nil {
		lines = append(lines, fmt.Sprintf("node admission token metadata: token_id=%s node_id=%s issuer=%s issued_at=%s expires_at=%s",
			s.Token.TokenID,
			s.Token.NodeID,
			s.Token.IssuerCoordinatorID,
			s.Token.IssuedAt.UTC().Format(time.RFC3339),
			s.Token.ExpiresAt.UTC().Format(time.RFC3339),
		))
	}

	lines = append(lines, "node admission note: locally cached token state is not authoritative admission truth; later coordinators must still enforce live admission state")

	return lines
}

func describeTokenCachePhase(state TokenCacheState) string {
	switch state.Phase {
	case TokenCachePhaseMissing:
		return "no locally cached current admission token"
	case TokenCachePhaseUsable:
		return fmt.Sprintf("usable cached current admission token until %s", state.Token.ExpiresAt.UTC().Format(time.RFC3339))
	case TokenCachePhaseExpired:
		return fmt.Sprintf("cached current admission token expired at %s", state.Token.ExpiresAt.UTC().Format(time.RFC3339))
	default:
		return string(state.Phase)
	}
}

func loadCachedToken(path string) (CachedTokenRecord, error) {
	file, err := os.Open(path)
	if err != nil {
		return CachedTokenRecord{}, fmt.Errorf("open cached admission token %q: %w", path, err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()

	var token CachedTokenRecord
	if err := decoder.Decode(&token); err != nil {
		return CachedTokenRecord{}, fmt.Errorf("decode cached admission token %q: %w", path, err)
	}

	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return CachedTokenRecord{}, fmt.Errorf("cached admission token %q must contain exactly one JSON object", path)
		}
		return CachedTokenRecord{}, fmt.Errorf("decode trailing cached admission token %q: %w", path, err)
	}

	if strings.TrimSpace(token.TokenID) == "" {
		return CachedTokenRecord{}, fmt.Errorf("cached admission token %q must set token_id", path)
	}
	if strings.TrimSpace(token.NodeID) == "" {
		return CachedTokenRecord{}, fmt.Errorf("cached admission token %q must set node_id", path)
	}
	if strings.TrimSpace(token.IssuerCoordinatorID) == "" {
		return CachedTokenRecord{}, fmt.Errorf("cached admission token %q must set issuer_coordinator_id", path)
	}
	if token.IssuedAt.IsZero() {
		return CachedTokenRecord{}, fmt.Errorf("cached admission token %q must set issued_at", path)
	}
	if token.ExpiresAt.IsZero() {
		return CachedTokenRecord{}, fmt.Errorf("cached admission token %q must set expires_at", path)
	}
	if !token.IssuedAt.Before(token.ExpiresAt) {
		return CachedTokenRecord{}, fmt.Errorf("cached admission token %q must have issued_at before expires_at", path)
	}

	return token, nil
}

func inspectTokenCache(dataDir, configuredPath, label string) (TokenCacheStatus, error) {
	status := TokenCacheStatus{
		ConfiguredPath: configuredPath,
		ResolvedPath:   resolveTokenPath(dataDir, configuredPath),
	}

	info, err := os.Stat(status.ResolvedPath)
	switch {
	case err == nil:
		if info.IsDir() {
			return status, fmt.Errorf("%s %q must point to a file, not a directory", label, status.ResolvedPath)
		}
		status.Exists = true
		return status, nil
	case errors.Is(err, os.ErrNotExist):
		return status, nil
	default:
		return status, fmt.Errorf("stat %s %q: %w", label, status.ResolvedPath, err)
	}
}

func resolveTokenPath(dataDir, configuredPath string) string {
	configuredPath = strings.TrimSpace(configuredPath)
	if configuredPath == "" {
		return ""
	}
	if filepath.IsAbs(configuredPath) {
		return filepath.Clean(configuredPath)
	}

	dataDir = strings.TrimSpace(dataDir)
	if dataDir == "" {
		return filepath.Clean(configuredPath)
	}

	return filepath.Clean(filepath.Join(dataDir, configuredPath))
}

func (s TokenCacheStatus) Presence() string {
	if s.Exists {
		return "present"
	}
	return "missing"
}

func (s TokenCacheStatus) DisplayPath() string {
	if s.ConfiguredPath == "" || filepath.Clean(s.ConfiguredPath) == s.ResolvedPath {
		return s.ResolvedPath
	}
	return fmt.Sprintf("%s (from %q)", s.ResolvedPath, s.ConfiguredPath)
}
