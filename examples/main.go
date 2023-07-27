package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jakegut/yabs"
)

func main() {

	yabs.Register("foo", []string{}, func(bc yabs.BuildCtx) {
		err := bc.Run("echo", "'hello foo\n'").
			StdoutToFile(bc.Out).
			Exec()

		if err != nil {
			log.Fatalf("echo hello: %s", err)
		}
		fmt.Println(bc.Out)
	})

	yabs.Register("bar", []string{}, func(bc yabs.BuildCtx) {
		err := bc.Run("echo", "'hello bar\n'").
			StdoutToFile(bc.Out).
			Exec()

		if err != nil {
			log.Fatal(err)
		}
	})

	yabs.Register("sleepy", []string{}, func(bc yabs.BuildCtx) {
		time.Sleep(time.Second * 1)

		bc.Run("echo", "h3223223ry").
			Exec()
	})

	yabs.Register("sleep_dep_1", []string{"sleepy"}, func(bc yabs.BuildCtx) {
		fmt.Println(bc.Dep["sleepy"])
	})
	yabs.Register("sleep_dep_2", []string{"sleepy"}, func(bc yabs.BuildCtx) {
		fmt.Println(bc.Dep["sleepy"])
	})

	yabs.Register("sleepy_final", []string{"sleep_dep_1", "sleep_dep_2"}, func(bc yabs.BuildCtx) {})

	oss := []string{"windows", "linux", "darwin"}
	goBuildTargets := []string{}
	for _, targetOS := range oss {
		targetOS := targetOS
		target := fmt.Sprintf("build_%s", targetOS)
		goBuildTargets = append(goBuildTargets, target)

		yabs.Register(target, []string{}, func(bc yabs.BuildCtx) {
			os.MkdirAll(bc.Out, 0770)
			fmt.Println("another change")
			err := bc.Run("go", "build", "-o", bc.Out, ".").WithEnv("GOOS", targetOS).Exec()
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

	if err := yabs.ExecWithDefault("sleepy_final"); err != nil {
		log.Fatal(err)
	}
}
