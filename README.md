# yabs-poc

proof of concept for [y]et [a]nother [b]uild [s]ystem

## Installation

*Not yet published*

`go get github.com/jakegut/yabs-poc`

## Usage

```go
bs := yabs.New()

// create a target given a name, list of dependencies, and a task function
bs.Register("target", []string{}, func(bc yabs.BuildCtx) {
    if err := bc.Run("echo", "hello world").Exec(); err != nil {
        log.Fatal("whoopsies")
    }
})

if err := bs.ExecWithDefault("target"); err != nil {
    log.Fatal(err)
}
```

### Build Go binaries

```go
bs := yabs.New()

// create a target based on a set of files given name and a list of globs
// Glob format: https://github.com/bmatcuk/doublestar#patterns
fileDeps := yabs.Fs(bs, "go_files", []string{"go.mod", "go.sum", "**/*.go"})

oss := []string{"windows", "linux", "darwin"}
goBuildTargets := []string{}
for _, targetOS := range oss {
    targetOS := targetOS
    target := fmt.Sprintf("build_%s", targetOS)
    goBuildTargets = append(goBuildTargets, target)

    bs.Register(target, []string{fileDeps}, func(bc yabs.BuildCtx) {
        // output from dependencies are avaliable via the `BuildCtx.Dep` map
        // outputs are symlinks from `.yabs/cache/...` to `.yabs/out/...`
        goFiles, err := os.Readlink(bc.Dep["go_files"])
        if err != nil {
            log.Fatalf("read go_files: %s", err)
        }
        // Store any outputs from a task with the `BuildCtx.Out` path which will be cached
        // by yabs, directories and files are supported
        err = bc.Run("go", "build", "-o", bc.Out, filepath.Join(goFiles, "main.go")).
            WithEnv("GOOS", targetOS).
            Exec()
        if err != nil {
            log.Fatal(err)
        }
    })
}

bs.Register("release", goBuildTargets, func(bc yabs.BuildCtx) {
    fmt.Println("releasing...")
    for name, dep := range bc.Dep {
        fmt.Println(name, dep)
    }
})

if err := bs.ExecWithDefault("release"); err != nil {
    log.Fatal(err)
}

// Only cache outputs from the previous build
bs.Prune()
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