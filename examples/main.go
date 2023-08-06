package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/jakegut/yabs"
	"github.com/jakegut/yabs/toolchain"
)

func main() {
	//
	bs := yabs.New()

	goTc := toolchain.Go(bs, "go1.20.7")

	fileDeps := yabs.Fs(bs, "go_files", []string{"go.mod", "go.sum", "**/*.go"})

	oss := []string{"windows", "linux", "darwin"}
	goBuildTargets := []string{}
	for _, targetOS := range oss {
		targetOS := targetOS
		target := fmt.Sprintf("build_%s", targetOS)
		goBuildTargets = append(goBuildTargets, target)

		bs.Register(target, []string{fileDeps, goTc}, func(bc yabs.BuildCtx) {
			goFiles, _ := os.Readlink(bc.Dep[fileDeps])

			goBinLoc, _ := os.Readlink(bc.Dep[goTc])
			goBin := filepath.Join(goBinLoc, "go")

			err := bc.Run(goBin, "build", "-o", bc.Out, filepath.Join(goFiles, "examples/main.go")).WithEnv("GOOS", targetOS).Exec()
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

// func main() {
// 	bs := yabs.New()

// 	fileDeps := yabs.Fs(bs, "go_files", []string{"go.mod", "go.sum", "**/*.go"})

// 	bs.Register("build", []string{v, fileDeps}, func(bc yabs.BuildCtx) {
// 		goBinLoc, _ := os.Readlink(bc.Dep[v])
// 		goBin := filepath.Join(goBinLoc, "go")

// 		bc.Run(goBin, "build", "examples/main.go").Exec()
// 	})

// 	bs.ExecWithDefault("build")
// }
