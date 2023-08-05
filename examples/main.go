package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/jakegut/yabs"
	"github.com/jakegut/yabs/toolchain"
)

func _main() {

	bs := yabs.New()

	fileDeps := yabs.Fs(bs, "go_files", []string{"go.mod", "go.sum", "**/*.go"})

	oss := []string{"windows", "linux", "darwin"}
	goBuildTargets := []string{}
	for _, targetOS := range oss {
		targetOS := targetOS
		target := fmt.Sprintf("build_%s", targetOS)
		goBuildTargets = append(goBuildTargets, target)

		bs.Register(target, []string{fileDeps}, func(bc yabs.BuildCtx) {
			goFiles, err := os.Readlink(bc.Dep["go_files"])
			if err != nil {
				log.Fatalf("read go_files: %s", err)
			}
			err = bc.Run("go", "build", "-o", bc.Out, filepath.Join(goFiles, "examples/main.go")).WithEnv("GOOS", targetOS).Exec()
			if err != nil {
				log.Fatal(err)
			}
		})
	}

	bs.Register("release", goBuildTargets, func(bc yabs.BuildCtx) {
		fmt.Println("releasing...")
		for name, dep := range bc.Dep {
			fmt.Println(name, dep)
		}
	})

	if err := bs.ExecWithDefault("release"); err != nil {
		log.Fatal(err)
	}

	bs.Prune()
}

func main() {
	bs := yabs.New()

	v := toolchain.Go(bs, "go1.20.7")

	fileDeps := yabs.Fs(bs, "go_files", []string{"go.mod", "go.sum", "**/*.go"})

	bs.Register("env", []string{v, fileDeps}, func(bc yabs.BuildCtx) {
		goBinLoc, _ := os.Readlink(bc.Dep[v])
		goBin := filepath.Join(goBinLoc, "go")

		bc.Run(goBin, "env").Exec()
	})

	bs.ExecWithDefault("env")
}
