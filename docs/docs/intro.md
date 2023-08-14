---
sidebar_position: 1
---

# Getting Started

## Installation

```
go install github.com/jakegut/yabs/cmd/yabs@latest
```

Or use the precompiled binaries from [the releases page](https://github.com/jakegut/yabs/releases).

## Usage

yabs scripts are written in the [risor](https://github.com/risor-io/risor) scripting language.

Create a `build.yb` at the root of your project:

```go
register('hello', [], func(bc) {
  sh('echo "hello world!"')
})
```

and run `yabs hello`:

```
2023/08/13 19:16:35 running "hello"
[hello] hello world!
```

And that's all you need to get started!