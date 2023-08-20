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

[View examples](https://yabs.build/docs/examples)
