package cli

import "embed"

// templatesFS embeds the repo-level templates/ tree so `hive init --template X`
// can copy full examples (agents + README) into a fresh project.
//
//go:embed all:templates
var templatesFS embed.FS
