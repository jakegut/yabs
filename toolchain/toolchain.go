package toolchain

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/jakegut/yabs"
)

type bs *yabs.Yabs

type Toolchain struct {
	ybs bs
}

func New(y *yabs.Yabs) *Toolchain {
	return &Toolchain{
		y,
	}
}

func (t *Toolchain) Go(version string) string {
	return Go(t.ybs, version)
}

func Go(bs *yabs.Yabs, version string) string {
	goRoot, _ := filepath.Abs(filepath.Join(".yabs", "go", version, "go"))
	goPath, _ := filepath.Abs(filepath.Join(".yabs", "go"))
	goCache, _ := filepath.Abs(filepath.Join(".yabs", "go", "go-build"))

	goBinPath, _ := filepath.Abs(filepath.Join(goRoot, "bin"))
	os.Setenv("GOROOT", goRoot)
	os.Setenv("GOPATH", goPath)
	os.Setenv("GOCACHE", goCache)
	path := os.Getenv("PATH")
	os.Setenv("PATH", goBinPath+":"+path)

	tp := ToolchainProvider{
		Type:    "go",
		Version: version,
		BinLoc:  []string{"go", "bin"},
		DownloadURL: func(tp ToolchainProvider) string {
			os := runtime.GOOS
			arch := runtime.GOARCH

			archiveExt := "tar.gz"
			if os == "windows" {
				archiveExt = "zip"
			}

			return fmt.Sprintf("https://go.dev/dl/%s.%s-%s.%s", version, os, arch, archiveExt)
		},
	}

	tp.Register(bs)

	return tp.GetTargetName()
}

func Node(bs *yabs.Yabs, version string) string {

	arch := runtime.GOARCH
	if arch == "amd64" {
		arch = "x64"
	}

	goos := runtime.GOOS
	ext := "tar.gz"
	if goos == "windows" {
		goos = "win"
		ext = "zip"
	}

	fileName := fmt.Sprintf("node-%s-%s-%s", version, goos, arch)

	tp := ToolchainProvider{
		Type:    "node",
		Version: version,
		BinLoc:  []string{fileName, "bin"},
		DownloadURL: func(tp ToolchainProvider) string {
			// https://nodejs.org/dist/v18.17.1/node-v18.17.1-linux-arm64.tar.xz
			return fmt.Sprintf("https://nodejs.org/dist/%s/%s.%s", version, fileName, ext)
		},
	}

	bin := filepath.Join(tp.BinLoc...)
	nodeBinAbs, _ := filepath.Abs(filepath.Join(tp.getPrefix(), bin))
	npmCacheAbs, _ := filepath.Abs(filepath.Join(".yabs", "node", ".npm_cache"))

	path := os.Getenv("PATH")
	os.Setenv("PATH", nodeBinAbs+":"+path)
	os.Setenv("npm_config_cache", npmCacheAbs)

	tp.Register(bs)

	return tp.GetTargetName()
}
