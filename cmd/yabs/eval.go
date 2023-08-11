package main

import (
	"context"

	"github.com/jakegut/yabs"
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

// minor change

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
