package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"

	"github.com/fatih/color"
	"github.com/jakegut/yabs"
	"github.com/jakegut/yabs/toolchain"
	"github.com/risor-io/risor"
	"github.com/risor-io/risor/object"
)

func fsFunc(y *yabs.Yabs) object.BuiltinFunction {
	// args: name string, fileGlobs []string
	return func(ctx context.Context, args ...object.Object) object.Object {
		if len(args) != 2 {
			return object.NewArgsError("fs", 2, len(args))
		}
		name, err := validateString(args[0])
		if err != nil {
			return object.NewError(err)
		}
		globs, err := validateList[string](args[1])
		if err != nil {
			object.NewError(err)
		}
		return object.NewString(yabs.Fs(y, name, globs))
	}
}

// args: command string
func sh(ctx context.Context, args ...object.Object) object.Object {
	if len(args) != 1 {
		return object.NewArgsError("sh", 1, len(args))
	}

	cmdArg, ok := args[0].(*object.String)
	if !ok {
		return object.Errorf("expected string as arg, got=%T", args[0])
	}
	cmdStr := cmdArg.String()

	cmd := exec.Command("sh", "-c", cmdStr)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	if err := cmd.Start(); err != nil {
		return object.Errorf("cmd start: %s", err)
	}
	if err := cmd.Wait(); err != nil {
		return object.Errorf("cmd wait: %s", err)
	}
	return object.Nil
}

func goTcFunc(y *yabs.Yabs) object.BuiltinFunction {
	return func(ctx context.Context, args ...object.Object) object.Object {
		if len(args) != 1 {
			return object.NewArgsError("go", 1, len(args))
		}
		version, err := validateString(args[0])
		if err != nil {
			object.NewError(err)
		}
		return object.NewString(toolchain.Go(y, version))
	}
}

func registerFunc(y *yabs.Yabs) object.BuiltinFunction {
	// args: name string, deps []string, task func(bc BuildCtx)
	return func(ctx context.Context, args ...object.Object) object.Object {
		if len(args) != 3 {
			return object.NewArgsError("register", 3, len(args))
		}
		target, err := validateString(args[0])
		if err != nil {
			object.NewError(err)
		}
		deps, err := validateList[string](args[1])
		if err != nil {
			return object.NewError(err)
		}
		taskFnObj, ok := args[2].(*object.Function)
		if !ok {
			return object.NewError(fmt.Errorf("wrong type for second arg, want=func(bc), got=%T", args[2]))
		}
		y.Register(target, deps, func(bc yabs.BuildCtx) {
			callFunc, ok := object.GetCallFunc(ctx)
			if !ok {
				log.Fatalf("oof")
			}

			bcProxy, err := object.NewProxy(&bc)
			if err != nil {
				log.Fatalf("creating new proxy; %s", err)
			}

			_, err = callFunc(ctx, taskFnObj, []object.Object{bcProxy})
			if err != nil {
				log.Fatalf("calling func for target %q: %s", target, err)
			}
		})
		return object.NewString(target)
	}
}

var red = color.New(color.FgRed).SprintfFunc()

func main() {

	args := os.Args
	defaultTarget := "build"
	if len(args) == 2 {
		defaultTarget = args[1]
	}

	bs := yabs.New()

	fileContent, err := os.ReadFile("build.yb")
	if err != nil {
		log.Fatalf("reading: %s", err)
	}

	ctx := context.Background()

	// Build up options for Risor, including the proxy as a variable named "svc"
	opts := []risor.Option{
		risor.WithDefaultBuiltins(),
		risor.WithBuiltins(map[string]object.Object{
			"register": object.NewBuiltin("register", registerFunc(bs)),
			"sh":       object.NewBuiltin("sh", sh),
			"fs":       object.NewBuiltin("fs", fsFunc(bs)),
			"go":       object.NewBuiltin("go", goTcFunc(bs)),
		}),
	}

	if _, err = risor.Eval(ctx, string(fileContent), opts...); err != nil {
		fmt.Println(red(err.Error()))
		os.Exit(1)
	}

	if err := bs.ExecWithDefault(defaultTarget); err != nil {
		log.Fatal(err)
	}
}
