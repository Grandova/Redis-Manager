package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var distFS embed.FS

// GetStaticFS returns the embedded dist sub-filesystem
func GetStaticFS() fs.FS {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		return nil
	}
	return sub
}
