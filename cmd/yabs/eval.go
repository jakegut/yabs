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
	"github.com/jakegut/yabs/prefixer"
	"github.com/jakegut/yabs/toolchain"
	"github.com/risor-io/risor/builtins"
	"github.com/risor-io/risor/compiler"
	"github.com/risor-io/risor/importer"
	"github.com/risor-io/risor/object"
	"github.com/risor-io/risor/parser"
	"github.com/risor-io/risor/vm"

	modAws "github.com/risor-io/risor/modules/aws"
	modBase64 "github.com/risor-io/risor/modules/base64"
	modBytes "github.com/risor-io/risor/modules/bytes"
	modFetch "github.com/risor-io/risor/modules/fetch"
	modFmt "github.com/risor-io/risor/modules/fmt"
	modHash "github.com/risor-io/risor/modules/hash"
	modImage "github.com/risor-io/risor/modules/image"
	modJson "github.com/risor-io/risor/modules/json"
	modMath "github.com/risor-io/risor/modules/math"
	modOs "github.com/risor-io/risor/modules/os"
	modPgx "github.com/risor-io/risor/modules/pgx"
	modRand "github.com/risor-io/risor/modules/rand"
	modStrconv "github.com/risor-io/risor/modules/strconv"
	modStrings "github.com/risor-io/risor/modules/strings"
	modTime "github.com/risor-io/risor/modules/time"
	modUuid "github.com/risor-io/risor/modules/uuid"
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

	var stdout io.Writer = &outBuf
	var stderr io.Writer = os.Stderr
	if targetName, ok := ctx.Value(targetNameKey).(string); ok {
		stdout = io.MultiWriter(prefixer.New(targetName, os.Stdout), &outBuf)
		stderr = prefixer.New(targetName, os.Stderr)
	}

	cmd := exec.Command("sh", "-c", cmdStr)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
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

			// callFunc, ok := object.GetCallFunc(ctx)
			// if !ok {
			// 	log.Fatalf("oof")
			// }

			bcProxy, err := object.NewProxy(&bc)
			if err != nil {
				log.Fatalf("creating new proxy; %s", err)
			}

			ctx = context.WithValue(ctx, targetNameKey, target)

			_, err = machine.CallFunction(ctx, taskFnObj, []object.Object{bcProxy})
			if err != nil {
				log.Fatalf("calling func for target %q: %s", target, err)
			}
		})
		return object.NewString(target)
	}
}

type contextKey string

const vmFuncKey = contextKey("yabs:vmfunc")

const targetNameKey = contextKey("yabs:targetname")

type VmFunc func() *vm.VirtualMachine

func newVMFunc(code *object.Code, builtins map[string]object.Object) VmFunc {
	return func() *vm.VirtualMachine {
		return getVM(code, builtins)
	}
}

func getBuiltins(bs *yabs.Yabs) map[string]object.Object {
	allBuiltins := map[string]object.Object{
		// default modules
		"math":    modMath.Module(),
		"json":    modJson.Module(),
		"strings": modStrings.Module(),
		"time":    modTime.Module(),
		"rand":    modRand.Module(),
		"strconv": modStrconv.Module(),
		"pgx":     modPgx.Module(),
		"uuid":    modUuid.Module(),
		"os":      modOs.Module(),
		"bytes":   modBytes.Module(),
		"base64":  modBase64.Module(),
		"fmt":     modFmt.Module(),
		"image":   modImage.Module(),
		// custom builtins
		"register": object.NewBuiltin("register", registerFunc(bs)),
		"sh":       object.NewBuiltin("sh", sh),
		"fs":       object.NewBuiltin("fs", fsFunc(bs)),
		"go":       object.NewBuiltin("go", goTcFunc(bs)),
	}
	if awsMod := modAws.Module(); awsMod != nil {
		allBuiltins["aws"] = awsMod
	}

	// default builtins

	for k, v := range builtins.Builtins() {
		allBuiltins[k] = v
	}
	for k, v := range modFetch.Builtins() {
		allBuiltins[k] = v
	}
	for k, v := range modFmt.Builtins() {
		allBuiltins[k] = v
	}
	for k, v := range modHash.Builtins() {
		allBuiltins[k] = v
	}
	for k, v := range modOs.Builtins() {
		allBuiltins[k] = v
	}

	return allBuiltins
}

func compile(ctx context.Context, source string, allBuiltins map[string]object.Object) (*object.Code, error) {

	ast, err := parser.Parse(ctx, source)
	if err != nil {
		return nil, err
	}

	compilerOpts := []compiler.Option{
		compiler.WithBuiltins(allBuiltins),
	}
	comp, err := compiler.New(compilerOpts...)
	if err != nil {
		return nil, err
	}

	return comp.Compile(ast)
}

func getVM(code *object.Code, builtins map[string]object.Object) *vm.VirtualMachine {
	localImporter := importer.NewLocalImporter(importer.LocalImporterOptions{Extensions: []string{".yb", ".yabs"}, Builtins: builtins})

	vmOpts := []vm.Option{
		vm.WithImporter(localImporter),
	}

	return vm.New(code, vmOpts...)
}

func eval(ctx context.Context, code *object.Code, builtins map[string]object.Object) error {

	machine := getVM(code, builtins)

	return machine.Run(ctx)
}
