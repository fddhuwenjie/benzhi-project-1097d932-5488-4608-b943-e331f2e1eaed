package accessibility

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
)

func DigestBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func ValidDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func CalculateProfileDigest(version string, rules []string, severities []Severity) string {
	r := append([]string(nil), rules...)
	sort.Strings(r)
	s := make([]string, len(severities))
	for i := range severities {
		s[i] = string(severities[i])
	}
	sort.Strings(s)
	payload, _ := json.Marshal(struct {
		Version    string   `json:"ruleset_version"`
		Rules      []string `json:"rule_codes"`
		Severities []string `json:"blocking_severities"`
	}{version, r, s})
	return DigestBytes(payload)
}

func CalculateReleaseToken(caseID, baseline, content, authorizedBy string, revision int64) string {
	payload, _ := json.Marshal([]any{caseID, baseline, content, authorizedBy, revision})
	return "rel_" + DigestBytes(payload)[:40]
}

func CalculateManifestDigest(m ArchiveManifest) string {
	payload, _ := json.Marshal(struct {
		CaseID, ReleaseToken, BaselineDigest, ContentDigest, EventChainHead string
	}{m.CaseID, m.ReleaseToken, m.BaselineDigest, m.ContentDigest, m.EventChainHead})
	return DigestBytes(payload)
}

func NormalizeCodes(codes []string) []string {
	set := map[string]struct{}{}
	for _, code := range codes {
		code = strings.TrimSpace(strings.ToUpper(code))
		if code != "" {
			set[code] = struct{}{}
		}
	}
	result := make([]string, 0, len(set))
	for code := range set {
		result = append(result, code)
	}
	sort.Strings(result)
	return result
}
