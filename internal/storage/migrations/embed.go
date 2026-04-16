package migrations

import "embed"

//go:embed *.sql postgres/*.sql
var FS embed.FS
