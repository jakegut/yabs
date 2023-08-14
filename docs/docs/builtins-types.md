---
sidebar_position: 2
---

# Builtins & Types

## Builtins

### `register`

```go
/*
register(name: string, deps: []string, func(bc BuildCtx))
name: of the target, used in deps list or when invoking directly `yabs <name>`
deps: list of strings by target name, these targets will be invoked before running the current target
func: function to run when the target is invoked
*/
register("name", ["any", "deps"], func(bc){
    sh('echo "hello!"')
})
```

### `sh`
```go
/*
sh(cmd: string) []string | string | nil
cmd: command to run in the shell, internally invokes exec("sh", "-c", cmd)
    * the host's environment is inherited
Returns based on stdout:
    * []string for each line
    * string if only one line
    * nil if nothing
*/
sh('echo "run any command in here as if it was shell!"')
```

### `go`

Download and install a `go` toolchain specified by the version. The toolchain will be download in the project's `.yabs/go/<version>` directory.
The `GOROOT` and `GOPATH` environment variables will be set to be within the `.yabs/go` directory. The `PATH` env var will be modified to include the toolchain's `bin` directory.


While the `PATH` is modified, it will be good practice to get the path of the `go` or `gofmt` binaries directory by using `BuildCtx.GetDep(target)`.

```go
/*
go(version: string) string
returns the name of the target to use a a dep (the version)
the `go` and `gofmt` binaries are available in its' out directory
*/
go_tc := go("1.20.7")

register("env", [go_tc], func(bc) {
    go_bin := bc.GetDep(go_tc) + "/go"
    sh('{go_bin} env')
})
```

### `fs`
```go
/*
fs(name: string, globs: []string) string
Depend on a glob of files, returns the name of the target
Its' out directory will be a directory of hardlinks of matching files
name: name of target
globs: a list of globs to depend on
*/
readme := fs("readme", ["README.md"])

go_files := fs("go_files", ["go.mod", "go.sum", "**/*.go"])

register("cat", [readme], func(bc){
    sh('cat bc.GetDep(readme)')
})

register("build", [go_files], func(bc) {
    sh('go build -C bc.GetDep(go_files) -o {bc.Out} .')
})
```

## Types

## `BuildCtx`

This is the main type included and gives you access to the location of the output for the current target and a map of targets to outputs.

### `BuildCtx.Out`
```go
/*
`BuildCtx.Out` is the absolute path of where to store any outputs from the target, the output can be a file or a directory
If there's an output, it will be tracked by `yabs` and stored within the `.yabs/out` directory
*/
register("build", [], func(bc) {
    sh('go build -o {bc.Out} .')
})
```

### `BuildCtx.GetDep(target: string) string`

Get the absolute path of a target's output. The target must be a direct dependency. If the target isn't there or there were no outputs, it will return an empty string.

```go
register("hello", [], func(bc) {
    sh('echo "hello" > {bc.Out}')
})

register("cat", ["hello"], func(bc) {
    sh('cat {bc.GetDep("hello")}')
})
```

## Risor Features

There are a number of builtins, modules and types that are included, explore them at https://risor.io/docs.