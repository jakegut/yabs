package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/jakegut/yabs"
)

func main() {

	yabs.Register("sleepy", []string{}, func(bc yabs.BuildCtx) {
		time.Sleep(time.Second * 1)

		bc.Run("echo", "h3223223ry").
			StdoutToFile(bc.Out).
			Exec()
	})

	yabs.Register("sleep_dep_1", []string{"sleepy"}, func(bc yabs.BuildCtx) {
		fmt.Println(bc.Dep["sleepy"])
	})
	yabs.Register("sleep_dep_2", []string{"sleepy"}, func(bc yabs.BuildCtx) {
		fmt.Println(bc.Dep["sleepy"])
	})

	yabs.Register("sleepy_final", []string{"sleep_dep_1", "sleep_dep_2"}, func(bc yabs.BuildCtx) {})

	fileDeps := yabs.Fs("go_files", []string{"go.mod", "go.sum", "**/*.go"})

	oss := []string{"windows", "linux", "darwin"}
	goBuildTargets := []string{}
	for _, targetOS := range oss {
		targetOS := targetOS
		target := fmt.Sprintf("build_%s", targetOS)
		goBuildTargets = append(goBuildTargets, target)

		yabs.Register(target, []string{fileDeps}, func(bc yabs.BuildCtx) {
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

	yabs.Register("release", goBuildTargets, func(bc yabs.BuildCtx) {
		fmt.Println("releasing...")
		for name, dep := range bc.Dep {
			fmt.Println(name, dep)
		}
	})

	if err := yabs.ExecWithDefault("release"); err != nil {
		log.Fatal(err)
	}

	yabs.Prune()
}
