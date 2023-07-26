package main

import (
	"fmt"
	"log"
	"os"
)

func main() {

	Register("foo", []string{}, func(bc BuildCtx) {
		err := bc.Run("echo", "'hello foo\n'").
			StdoutToFile(bc.Out).
			Exec()

		if err != nil {
			log.Fatalf("echo hello: %s", err)
		}
		fmt.Println(bc.Out)
	})

	Register("bar", []string{}, func(bc BuildCtx) {
		err := bc.Run("echo", "'hello bar\n'").
			StdoutToFile(bc.Out).
			Exec()

		if err != nil {
			log.Fatal(err)
		}
	})

	oss := []string{"windows", "linux", "darwin"}
	goBuildTargets := []string{}
	for _, targetOS := range oss {
		targetOS := targetOS
		target := fmt.Sprintf("build_%s", targetOS)
		goBuildTargets = append(goBuildTargets, target)

		Register(target, []string{}, func(bc BuildCtx) {
			os.Mkdir(bc.Out, 0770)
			err := bc.Run("go", "build", "-o", bc.Out, ".").WithEnv("GOOS", targetOS).Exec()
			if err != nil {
				log.Fatal(err)
			}
		})
	}

	Register("release", goBuildTargets, func(bc BuildCtx) {
		fmt.Println("releasing...")
		for _, dep := range bc.Dep {
			fmt.Println(dep)
		}
	})

	if err := ExecWithDefault("release"); err != nil {
		log.Fatal(err)
	}
}
