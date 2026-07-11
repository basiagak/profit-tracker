// Package migrations embeds the golang-migrate SQL files into the binary so
// cmd/server can apply/verify them at startup without depending on a
// filesystem path relative to the process's working directory.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
