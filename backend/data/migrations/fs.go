package migrations

import "embed"

//go:embed *.up.sql
var UpFS embed.FS
