package webhook

import (
	"encoding/json"
	"regexp"
	"strconv"
)

// extractGitHubContext scans an event payload JSON and pulls out any common
// GitHub-related fields so the `github` webhook format can expose them as
// first-class keys on `client_payload`.
//
// Recognised fields (checked in that order):
//   - pr_number / pull_request_number / pr → "pr_number"
//   - issue_number / issue                 → "issue_number"
//   - repo / repository                    → "repository"
//   - commit / sha                         → "commit_sha"
//
// Raw string payloads that contain a GitHub URL (…/pull/123 or …/issues/456)
// also get parsed so Slack-style text events still surface context.
func extractGitHubContext(payload string) map[string]any {
	if payload == "" {
		return nil
	}
	out := map[string]any{}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(payload), &parsed); err == nil {
		for _, key := range []string{"pr_number", "pull_request_number", "pr"} {
			if v, ok := parsed[key]; ok {
				out["pr_number"] = coerceInt(v)
				break
			}
		}
		for _, key := range []string{"issue_number", "issue"} {
			if v, ok := parsed[key]; ok {
				out["issue_number"] = coerceInt(v)
				break
			}
		}
		for _, key := range []string{"repository", "repo"} {
			if v, ok := parsed[key].(string); ok && v != "" {
				out["repository"] = v
				break
			}
		}
		for _, key := range []string{"commit_sha", "sha", "commit"} {
			if v, ok := parsed[key].(string); ok && v != "" {
				out["commit_sha"] = v
				break
			}
		}
	}

	if _, ok := out["pr_number"]; !ok {
		if n := scanNumber(payload, `/pull/(\d+)`); n > 0 {
			out["pr_number"] = n
		}
	}
	if _, ok := out["issue_number"]; !ok {
		if n := scanNumber(payload, `/issues/(\d+)`); n > 0 {
			out["issue_number"] = n
		}
	}

	if len(out) == 0 {
		return nil
	}
	return out
}

func coerceInt(v any) any {
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case int64:
		return int(t)
	case string:
		if n, err := strconv.Atoi(t); err == nil {
			return n
		}
	}
	return v
}

func scanNumber(s, pattern string) int {
	re := regexp.MustCompile(pattern)
	m := re.FindStringSubmatch(s)
	if len(m) < 2 {
		return 0
	}
	n, _ := strconv.Atoi(m[1])
	return n
}
