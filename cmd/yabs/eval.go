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

	modBase64 "github.com/risor-io/risor/modules/base64"
	modBytes "github.com/risor-io/risor/modules/bytes"
	modFetch "github.com/risor-io/risor/modules/fetch"
	modFmt "github.com/risor-io/risor/modules/fmt"
	modHash "github.com/risor-io/risor/modules/hash"
	modJson "github.com/risor-io/risor/modules/json"
	modMath "github.com/risor-io/risor/modules/math"
	modOs "github.com/risor-io/risor/modules/os"
	modRand "github.com/risor-io/risor/modules/rand"
	modStrconv "github.com/risor-io/risor/modules/strconv"
	modStrings "github.com/risor-io/risor/modules/strings"
	modTime "github.com/risor-io/risor/modules/time"
	modUuid "github.com/risor-io/risor/modules/uuid"
)

func fsFunc(y *yabs.Yabs) object.BuiltinFunction {
	// args: name string, fileGlobs []string
	return func(ctx context.Context, args ...object.Object) object.Object {
		if len(args) < 2 || len(args) > 3 {
			return object.NewArgsRangeError("fs", 2, 3, len(args))
		}
		name, err := validateString(args[0])
		if err != nil {
			return object.NewError(err)
		}
		globs, err := validateList[string](args[1])
		if err != nil {
			object.NewError(err)
		}
		exclude := []string{}
		if len(args) == 3 {
			var err error
			exclude, err = validateList[string](args[2])
			if err != nil {
				object.NewError(err)
			}
		}
		return object.NewString(yabs.Fs(y, name, globs, exclude))
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

func nodeTcFunc(y *yabs.Yabs) object.BuiltinFunction {
	return func(ctx context.Context, args ...object.Object) object.Object {
		if len(args) != 1 {
			return object.NewArgsError("node", 1, len(args))
		}
		version, err := validateString(args[0])
		if err != nil {
			object.NewError(err)
		}
		return object.NewString(toolchain.Node(y, version))
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

			bcProxy, err := object.NewProxy(&bc)
			if err != nil {
				log.Fatalf("creating new proxy; %s", err)
			}

			ctx = context.WithValue(ctx, targetNameKey, target)

			_, err = machine.Call(ctx, taskFnObj, []object.Object{bcProxy})
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

func newVMFunc(parent *vm.VirtualMachine) VmFunc {
	return func() *vm.VirtualMachine {
		clone, err := parent.Clone()
		if err != nil {
			log.Fatalf("failed to clone vm: %s", err)
		}
		return clone
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
		"uuid":    modUuid.Module(),
		"os":      modOs.Module(),
		"bytes":   modBytes.Module(),
		"base64":  modBase64.Module(),
		"fmt":     modFmt.Module(),
		// custom builtins
		"register": object.NewBuiltin("register", registerFunc(bs)),
		"sh":       object.NewBuiltin("sh", sh),
		"fs":       object.NewBuiltin("fs", fsFunc(bs)),
		"go":       object.NewBuiltin("go", goTcFunc(bs)),
		"node":     object.NewBuiltin("node", nodeTcFunc(bs)),
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

func compile(ctx context.Context, source string, allBuiltins map[string]object.Object) (*compiler.Code, error) {

	ast, err := parser.Parse(ctx, source)
	if err != nil {
		return nil, err
	}

	compilerOpts := []compiler.Option{
		compiler.WithGlobalNames(mapKeys(allBuiltins)),
	}
	comp, err := compiler.New(compilerOpts...)
	if err != nil {
		return nil, err
	}

	return comp.Compile(ast)
}

func getVM(code *compiler.Code, globals map[string]object.Object) *vm.VirtualMachine {

	globalNames := mapKeys(globals)
	genericGlobals := map[string]any{}
	for k, v := range globals {
		genericGlobals[k] = v
	}

	localImporter := importer.NewLocalImporter(importer.LocalImporterOptions{
		Extensions:  []string{".yb", ".yabs"},
		GlobalNames: globalNames,
	})

	vmOpts := []vm.Option{
		vm.WithImporter(localImporter),
		vm.WithGlobals(genericGlobals),
	}
	return vm.New(code, vmOpts...)
}

func mapKeys(m map[string]object.Object) []string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
