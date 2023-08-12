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

type Prefixer struct {
	prefix string
	color  color.Attribute
}

var _ io.Writer = Prefixer{}

func New(prefix string) Prefixer {
	rand.Seed(time.Now().UnixNano())
	min := int(color.FgBlack)
	max := int(color.FgWhite)
	col := color.Attribute(rand.Intn(max-min+1) + min)

	return Prefixer{
		prefix: prefix,
		color:  col,
	}
}

func (p Prefixer) Write(bs []byte) (int, error) {
	scanner := bufio.NewScanner(bytes.NewReader(bs))
	scanner.Split(bufio.ScanLines)

	for scanner.Scan() {
		line := scanner.Text()
		fmt.Printf("[")
		_, err := color.New(p.color).Print(p.prefix)
		if err != nil {
			return 0, err
		}
		_, err = fmt.Printf("] %s\n", line)
		if err != nil {
			return 0, err
		}
	}

	return len(bs), nil
}
