// Package webui carries the built editor inside the server binary.
//
// `npm run build` in web/ writes the editor to internal/webui/dist, and
// `go build` embeds that directory, so a release is one file. A checkout
// without a frontend build still compiles: the directory holds a
// placeholder, Dist reports ok == false, and the server shows a hint page
// instead of the editor.
package webui

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// Dist returns the built editor with the dist directory as its root. ok is
// false when this binary was built without a frontend build (no
// index.html), in which case the returned FS is still usable but empty.
func Dist() (fsys fs.FS, ok bool) {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		return dist, false
	}
	if _, err := fs.Stat(sub, "index.html"); err != nil {
		return sub, false
	}
	return sub, true
}
