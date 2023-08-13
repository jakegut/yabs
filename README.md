# yabs

![GitHub Workflow Status](https://img.shields.io/github/actions/workflow/status/jakegut/yabs/main.yml?style=for-the-badge&logo=github)
[![Apache-2.0 license](https://img.shields.io/github/license/jakegut/yabs?style=for-the-badge)](https://opensource.org/license/apache-2-0/)
[![Go Report Card](https://goreportcard.com/badge/github.com/jakegut/yabs?style=for-the-badge)](https://goreportcard.com/report/github.com/jakegut/yabs)
[![Releases](https://img.shields.io/github/release/jakegut/yabs/all.svg?style=for-the-badge)](https://github.com/jakegut/yabs/releases)

[y]et [a]nother [b]uild [s]ystem

a build system, configurable in [risor](https://github.com/risor-io/risor), a go-like scripting language

## Installation

`go install github.com/jakegut/yabs/cmd/yabs@latest`

## Usage

Make a `build.yb` file at the root of your project with the logic for your builds

```go
register('hello', [], func(bc) {
    sh("echo 'hello world!'")
})
```

```
> yabs hello
2023/08/11 20:22:12 running "hello"
hello world!
```

### Build Multiple Go Binaries

```go
// use a go toolchain with a specific version, returns the name of the target
go_tc := go("go1.20.5")

// depend on files using `fs(name, list of globs)`, returns the name of the target
go_files := fs("go_files", ["go.mod", "go.sum", "**/*.go"])

// create a target `register(name, list of deps, task func)`
register("go_download", [go_tc, go_files], func(bc){
    go_bin := bc.GetDep(go_tc) + "/go"

    sh('{go_bin} mod download')
})

build_all := []

for _, goos := range ["windows", "darwin", "linux"] {
    for _, goarch := range ["amd64", "arm64"] {
        name := 'build_{goos}_{goarch}'
        build_all.append(name)
        register(name, [go_tc, go_files, "go_download"], func(bc) {
            goos := goos
            goarch := goarch
            
            go_bin := bc.GetDep(go_tc) + "/go"

            // Put any outputs in bc.Out to cache them with yabs
            sh('GOOS={goos} GOARCH={goarch} {go_bin} build -o {bc.Out} .')
        })
    }
}

register("build_all", build_all, func(bc) {
    for _, build := range build_all {
        // direct dependencies' outputs are available at `bc.GetDep(target)`
        bin_loc := bc.GetDep(build)
        print(build, bin_loc)
    }
})
```

```
> yabs build_all
2023/08/11 20:33:15 running "go_files"
2023/08/11 20:33:15 running "go1.20.5"
2023/08/11 20:33:15 downloading "go1.20.5" from https://go.dev/dl/go1.20.5.linux-amd64.tar.gz
2023/08/11 20:33:15 extracting tar.gz
2023/08/11 20:33:19 running "go_download"
2023/08/11 20:33:23 running "build_linux_arm64"
2023/08/11 20:33:26 running "build_linux_amd64"
2023/08/11 20:33:26 running "build_windows_arm64"
2023/08/11 20:33:27 running "build_darwin_amd64"
2023/08/11 20:33:27 running "build_darwin_arm64"
2023/08/11 20:33:27 running "build_windows_amd64"
2023/08/11 20:33:28 running "build_all"
build_windows_amd64 /abs/path/to/project/.yabs/out/yabs-out-650635068
build_windows_arm64 /abs/path/to/project/.yabs/out/yabs-out-3597857295
build_darwin_amd64 /abs/path/to/project/.yabs/out/yabs-out-3821563234
build_darwin_arm64 /abs/path/to/project/.yabs/out/yabs-out-2023236471
build_linux_amd64 /abs/path/to/project/.yabs/out/yabs-out-2379893467
build_linux_arm64 /abs/path/to/project/.yabs/out/yabs-out-1121798096
```

## Design

### Goals
* Composable, write Go for targets, rules
* Distribute `yabs` as a binary, not as a module
* Build ~15 projects efficiently
* Replace `make`
* Run in CI and locally

### Non-goals
* Build millions+ LoC
