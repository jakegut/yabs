package yabs

import (
	"crypto/sha256"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"golang.org/x/exp/slices"
)

func Fs(name string, globs []string) string {
	slices.Sort(globs)
	if len(globs) == 0 {
		log.Fatalf("list of globs can't be empty")
	}
	nameHash := sha256.New()
	for _, glob := range globs {
		nameHash.Sum([]byte(glob))
	}
	Register(name, []string{}, func(bc BuildCtx) {
		for _, glob := range globs {

			err := doublestar.GlobWalk(os.DirFS("."), glob, func(path string, d fs.DirEntry) error {
				if d.IsDir() {
					switch d.Name() {
					case ".git", ".yabs":
						return doublestar.SkipDir
					}
				} else {
					if strings.HasPrefix(path, ".yabs") {
						return doublestar.SkipDir
					}
					newname := filepath.Join(bc.Out, path)
					if err := os.MkdirAll(filepath.Dir(newname), 0777); err != nil {
						log.Fatalf("mkdir: %+v", err)
					}
					if err := os.Link(path, newname); err != nil {
						log.Fatalf("link error: %s", err)
					}
				}
				return nil
			})
			if err != nil {
				log.Fatalf("traversing glob %q %s", glob, err)
			}
		}
	})

	return name
}
