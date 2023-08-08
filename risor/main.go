package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"

	"github.com/fatih/color"
	"github.com/jakegut/yabs"
	"github.com/risor-io/risor"
	"github.com/risor-io/risor/object"
)

const BC_KEY = "yabs:buildctx"

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

func registerFunc(y *yabs.Yabs) object.BuiltinFunction {
	return func(ctx context.Context, args ...object.Object) object.Object {
		if len(args) != 3 {
			return object.NewArgsError("register", 3, len(args))
		}
		targetStr, ok := args[0].(*object.String)
		if !ok {
			return object.NewError(fmt.Errorf("wrong type for first arg, want=string, got=%T", args[0]))
		}
		target := targetStr.String()
		deps, ok := args[1].(*object.List)
		if !ok {
			return object.NewError(fmt.Errorf("wrong type for second arg, want=string, got=%T", args[1]))
		}
		depsStr := []string{}
		it := deps.Iter()
		for {
			dep, ok := it.Next()
			if !ok {
				break
			}
			depStr, ok := dep.(*object.String)
			if !ok {
				return object.NewError(fmt.Errorf("wrong type of dep list, expected all strings (target=%s)", target))
			}
			depsStr = append(depsStr, depStr.String())
		}
		taskFnObj, ok := args[2].(*object.Function)
		if !ok {
			return object.NewError(fmt.Errorf("wrong type for second arg, want=func(bc), got=%T", args[2]))
		}
		y.Register(target, depsStr, func(bc yabs.BuildCtx) {
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
		return targetStr
	}
}

var red = color.New(color.FgRed).SprintfFunc()

func main() {

	bs := yabs.New()
	register := registerFunc(bs)
	registerBuiltIn := object.NewBuiltin("register", register)

	fileContent, err := os.ReadFile("risor/main.yb")
	if err != nil {
		log.Fatalf("reading: %s", err)
	}

	ctx := context.Background()

	// Create a Risor proxy for the service
	// proxy, err := object.NewProxy(svc)
	// if err != nil {
	// 	fmt.Println(red(err.Error()))
	// 	os.Exit(1)
	// }

	shBuiltin := object.NewBuiltin("sh", sh)

	// Build up options for Risor, including the proxy as a variable named "svc"
	opts := []risor.Option{
		risor.WithDefaultBuiltins(),
		risor.WithBuiltins(map[string]object.Object{
			"register": registerBuiltIn,
			"sh":       shBuiltin,
		}),
	}

	if _, err = risor.Eval(ctx, string(fileContent), opts...); err != nil {
		fmt.Println(red(err.Error()))
		os.Exit(1)
	}

	if err := bs.ExecWithDefault("target"); err != nil {
		log.Fatal(err)
	}
}
