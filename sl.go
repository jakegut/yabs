package yabs

import (
	"log"
	"os/exec"

	"go.starlark.net/starlark"
)

var starlarkRun = func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	execArgs := make([]string, len(args))
	for i, arg := range args {
		str, ok := starlark.AsString(arg)
		if !ok {
			log.Fatal("arg not a string")
		}
		execArgs[i] = str
	}

	cmd := exec.Command(execArgs[0], execArgs[1:]...)
	cmd.Start()
	if err := cmd.Wait(); err != nil {
		return starlark.None, err
	}

	return starlark.None, nil
}

type SLBuildCtx struct {
	Run *starlark.Builtin
}

func (b SLBuildCtx) Type() string          { return "SLBuildCtx" }
func (b SLBuildCtx) Freeze()               {}
func (b SLBuildCtx) Truth() starlark.Bool  { return starlark.False }
func (b SLBuildCtx) Hash() (uint32, error) { return b.Run.Hash() }
func (b SLBuildCtx) String() string        { return "Normal SLBuildCtx" }
func (b SLBuildCtx) AttrNames() []string   { return []string{"Run"} }
func (b SLBuildCtx) Attr(name string) (starlark.Value, error) {
	switch name {
	case "Run":
		return b.Run, nil
	}
	return starlark.None, nil
}

func NewSLBuildCtx() *SLBuildCtx {
	return &SLBuildCtx{
		Run: starlark.NewBuiltin("Run", starlarkRun),
	}
}
