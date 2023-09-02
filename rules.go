package yabs

import (
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/jakegut/yabs/task"
)

func Fs(y *Yabs, name string, globs []string, exclude []string) string {
	if len(globs) == 0 {
		log.Fatalf("list of globs can't be empty")
	}
	y.Register(name, []string{}, func(bc task.BuildCtx) {
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
					for _, excludeStr := range exclude {
						if ok, err := doublestar.Match(excludeStr, path); ok {
							return nil
						} else if err != nil {
							return err
						}
					}
					newname := filepath.Join(bc.Out, path)
					if err := os.MkdirAll(filepath.Dir(newname), os.ModePerm); err != nil {
						return err
					}
					if err := os.Link(path, newname); err != nil {
						return err
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
