// Package ui loads the embedded UI files
package ui

import (
	"embed"
	"io/fs"
)

//go:embed files/build
var embedUI embed.FS

func GetFS() (fs.FS, error) {
	return fs.Sub(embedUI, "files/build")
}
