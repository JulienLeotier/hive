package webhook

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractPRNumberFromJSON(t *testing.T) {
	ctx := extractGitHubContext(`{"pr_number":42,"repository":"acme/widgets"}`)
	assert.Equal(t, 42, ctx["pr_number"])
	assert.Equal(t, "acme/widgets", ctx["repository"])
}

func TestExtractIssueNumberFromAltKey(t *testing.T) {
	ctx := extractGitHubContext(`{"issue":7}`)
	assert.Equal(t, 7, ctx["issue_number"])
}

func TestExtractPRNumberFromURL(t *testing.T) {
	ctx := extractGitHubContext(`{"link":"https://github.com/acme/widgets/pull/123"}`)
	assert.Equal(t, 123, ctx["pr_number"])
}

func TestExtractReturnsNilWhenNoContext(t *testing.T) {
	assert.Nil(t, extractGitHubContext(`{"note":"just some text"}`))
	assert.Nil(t, extractGitHubContext(""))
}

func TestExtractCommitSha(t *testing.T) {
	ctx := extractGitHubContext(`{"sha":"abcdef0"}`)
	assert.Equal(t, "abcdef0", ctx["commit_sha"])
}
