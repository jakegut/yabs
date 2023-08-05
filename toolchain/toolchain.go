package toolchain

import (
	"os"

	"github.com/jakegut/yabs"
)

func Go(bs *yabs.Yabs, version string) string {
	bs.Register(version, []string{}, func(bc yabs.BuildCtx) {
		tc := newGo(version)

		tc.download()

		os.Link(tc.binLocation, bc.Out)
	})
	return version
}
