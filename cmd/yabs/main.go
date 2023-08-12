package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"runtime/pprof"

	"github.com/jakegut/yabs"
	"github.com/urfave/cli/v2"
	"golang.org/x/exp/slices"
)

func getAvailableTargets(bs *yabs.Yabs) string {
	targets := bs.GetTaskNames()
	if len(targets) == 0 {
		return ""
	}
	slices.Sort(targets)
	var buffer bytes.Buffer
	buffer.WriteString("\nTARGETS:\n")
	for _, target := range targets {
		buffer.WriteString("\t")
		buffer.WriteString(target)
		buffer.WriteString("\n")
	}
	return buffer.String()
}

func main() {

	bs := yabs.New()

	fileContent, err := os.ReadFile("build.yb")
	if err != nil {
		log.Fatalf("reading: %s", err)
	}

	ctx := context.Background()

	builtins := getBuiltins(bs)

	code, err := compile(ctx, string(fileContent), builtins)
	if err != nil {
		log.Fatalf("compiling: %s", err)
	}

	ctx = context.WithValue(ctx, vmFuncKey, newVMFunc(code, builtins))

	if err = eval(ctx, code, builtins); err != nil {
		log.Fatalf("eval: %s", err)
	}

	availableTargets := getAvailableTargets(bs)

	cli.AppHelpTemplate = fmt.Sprintf(`NAME:
   {{.Name}} - {{.Usage}}
USAGE:
   {{.HelpName}} {{if .VisibleFlags}}[global options]{{end}}{{if .Commands}} command [command options]{{end}} {{if .ArgsUsage}}{{.ArgsUsage}}{{else}}[arguments...]{{end}}
   {{.HelpName}} target
   {{if len .Authors}}
AUTHOR:
   {{range .Authors}}{{ . }}{{end}}
   {{end}}{{if .Commands}}%s
COMMANDS:
{{range .Commands}}{{if not .HideHelp}}   {{join .Names ", "}}{{ "\t"}}{{.Usage}}{{ "\n" }}{{end}}{{end}}{{end}}{{if .VisibleFlags}}
GLOBAL OPTIONS:
   {{range .VisibleFlags}}{{.}}
   {{end}}{{end}}{{if .Copyright }}
COPYRIGHT:
   {{.Copyright}}
   {{end}}{{if .Version}}
VERSION:
   {{.Version}}
   {{end}}
`, availableTargets)

	var profile bool
	app := &cli.App{
		EnableBashCompletion: true,
		Usage:                "yet another build system",
		Copyright:            "Apache-2.0",
		Commands: []*cli.Command{
			{
				Name:  "prune",
				Usage: "removes un-used caches from `.yabs` directory",
				Action: func(cCtx *cli.Context) error {
					bs.RestoreTasks()
					bs.Prune()
					return nil
				},
			},
		},
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:        "cpuprofile",
				Value:       false,
				Usage:       "profile usage to `yabs.prof`",
				Destination: &profile,
			},
		},
		Action: func(cCtx *cli.Context) error {
			target := "build"
			if cCtx.NArg() > 0 {
				target = cCtx.Args().Get(0)
			}

			if profile {
				f, err := os.Create("yabs.prof")
				if err != nil {
					log.Fatal(err)
				}
				pprof.StartCPUProfile(f)
				defer pprof.StopCPUProfile()
			}

			return bs.ExecWithDefault(target)
		},
		BashComplete: func(ctx *cli.Context) {
			for _, task := range bs.GetTaskNames() {
				fmt.Println(task)
			}
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
