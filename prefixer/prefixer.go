package prefixer

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"time"

	"github.com/fatih/color"
)

var rint = rand.New(rand.NewSource(time.Now().UnixNano()))

type Prefixer struct {
	prefix string
	color  color.Attribute
	writer io.Writer
}

var _ io.Writer = Prefixer{}

func New(prefix string, writer io.Writer) Prefixer {
	min := int(color.FgBlack)
	max := int(color.FgWhite)
	col := color.Attribute(rint.Intn(max-min+1) + min)

	return Prefixer{
		prefix: prefix,
		color:  col,
		writer: writer,
	}
}

func (p Prefixer) Write(bs []byte) (int, error) {
	scanner := bufio.NewScanner(bytes.NewReader(bs))
	scanner.Split(bufio.ScanLines)

	for scanner.Scan() {
		line := scanner.Text()
		fmt.Fprintf(p.writer, "[")
		_, err := color.New(p.color).Fprint(p.writer, p.prefix)
		if err != nil {
			return 0, err
		}
		_, err = fmt.Fprintf(p.writer, "] %s\n", line)
		if err != nil {
			return 0, err
		}
	}

	return len(bs), nil
}
