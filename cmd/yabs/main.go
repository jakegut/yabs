package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/jakegut/yabs"
	"github.com/jakegut/yabs/toolchain"
	"github.com/risor-io/risor/object"
	"github.com/risor-io/risor/vm"
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
// returns str based on stdout
// if no stdout => nil, singular line => string, multi-line => list of strings
func sh(ctx context.Context, args ...object.Object) object.Object {
	if len(args) != 1 {
		return object.NewArgsError("sh", 1, len(args))
	}

	cmdArg, ok := args[0].(*object.String)
	if !ok {
		return object.Errorf("expected string as arg, got=%T", args[0])
	}
	cmdStr := cmdArg.String()

	var outBuf bytes.Buffer

	cmd := exec.Command("sh", "-c", cmdStr)
	cmd.Stdout = io.MultiWriter(os.Stdout, &outBuf)
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	if err := cmd.Start(); err != nil {
		return object.Errorf("cmd start: %s", err)
	}
	if err := cmd.Wait(); err != nil {
		return object.Errorf("cmd wait: %s", err)
	}

	str := outBuf.String()
	lines := strings.Split(str, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 {
		return object.Nil
	} else if len(lines) == 1 {
		return object.NewString(lines[0])
	} else {
		return object.NewStringList(lines)
	}

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
			newVM, ok := ctx.Value(vmFuncKey).(VmFunc)
			if !ok {
				log.Fatalf("vm not found")
			}
			machine := newVM()

			if err := machine.Run(ctx); err != nil {
				log.Fatal(err)
			}

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

type contextKey string

const vmFuncKey = contextKey("yabs:vmfunc")
const registerTargetKey = contextKey("yabs:regsiterTarget")

type VmFunc func() *vm.VirtualMachine

func newVMFunc(code *object.Code, builtins map[string]object.Object) VmFunc {
	return func() *vm.VirtualMachine {
		return getVM(code, builtins)
	}
}

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

	builtins := getBuiltins(bs)

	code, err := compile(ctx, string(fileContent), builtins)
	if err != nil {
		log.Fatalf("compiling: %s", err)
	}

	ctx = context.WithValue(ctx, vmFuncKey, newVMFunc(code, builtins))
	ctx = context.WithValue(ctx, registerTargetKey, "*")

	if err = eval(ctx, code, builtins); err != nil {
		log.Fatalf("eval: %s", err)
	}

	if err := bs.ExecWithDefault(defaultTarget); err != nil {
		log.Fatal(err)
	}
}
