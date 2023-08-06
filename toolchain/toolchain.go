package toolchain

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/jakegut/yabs"
)

func Go(bs *yabs.Yabs, version string) string {
	goRoot, _ := filepath.Abs(fmt.Sprintf(".yabs/go/%s/go", version))
	goPath, _ := filepath.Abs(".yabs/go")
	goCache, _ := filepath.Abs(".yabs/go/go-build")

	os.Setenv("GOROOT", goRoot)
	os.Setenv("GOPATH", goPath)
	os.Setenv("GOCACHE", goCache)

	bs.Register(version, []string{}, func(bc yabs.BuildCtx) {
		tc := newGo(version)
		tc.download()

		if err := os.Mkdir(bc.Out, 0770); err != nil {
			log.Fatal(err)
		}

		bins := []string{"go", "gofmt"}

		for _, bin := range bins {
			if err := os.Link(filepath.Join(tc.binLocation, bin), filepath.Join(bc.Out, bin)); err != nil {
				log.Fatal(err)
			}
		}
	})
	return version
}
