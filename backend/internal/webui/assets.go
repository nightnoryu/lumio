package webui

import (
	"embed"
	"io/fs"
)

// Build the frontend with mise run web:build before compiling Go.
//
//go:embed all:dist
var assets embed.FS

func Assets() (fs.FS, error) {
	return fs.Sub(assets, "dist")
}
