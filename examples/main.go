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

	bs := yabs.New()

	bs.Register("sleepy", []string{}, func(bc yabs.BuildCtx) {
		time.Sleep(time.Second * 1)

		bc.Run("echo", "h3223223ry").
			StdoutToFile(bc.Out).
			Exec()
	})

	bs.Register("sleep_dep_1", []string{"sleepy"}, func(bc yabs.BuildCtx) {
		time.Sleep(time.Second * 5)
		fmt.Println("sleep_dep_1")
		fmt.Println(bc.Dep["sleepy"])
	})
	bs.Register("sleep_dep_2", []string{"sleepy"}, func(bc yabs.BuildCtx) {
		log.Fatal("rip")
		fmt.Println("sleep_dep_2")
		fmt.Println(bc.Dep["sleepy"])
	})

	bs.Register("sleepy_final", []string{"sleep_dep_1", "sleep_dep_2"}, func(bc yabs.BuildCtx) {})

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
