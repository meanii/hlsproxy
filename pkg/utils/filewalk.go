package utils

import (
	"io/fs"
	"path/filepath"
)

func FindFiles(root, ext string) []string {
	var a []string
	filepath.WalkDir(root, func(s string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if filepath.Ext(d.Name()) == ext {
			a = append(a, s)
		}
		return nil
	})
	return a
}
