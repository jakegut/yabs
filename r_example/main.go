package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/risor-io/risor"
	"github.com/risor-io/risor/object"
)

type Service struct {
	name       string
	running    bool
	startCount int
	stopCount  int
}

func IntPtr(i int) *int {
	return &i
}

func (s *Service) Start() error {
	if s.running {
		return fmt.Errorf("service %s already running", s.name)
	}
	s.running = true
	s.startCount = 1
	return nil
}

func (s *Service) GetMetrics() map[string]interface{} {
	return map[string]interface{}{
		"running":     s.running,
		"start_count": s.startCount,
		"stop_count":  s.stopCount,
	}
}

const defaultExample = `
print(svc.Start())
print(svc.name)
`

var red = color.New(color.FgRed).SprintfFunc()

func main() {
	var code string
	flag.StringVar(&code, "code", defaultExample, "Code to evaluate")
	flag.Parse()

	ctx := context.Background()

	// Initialize the service
	svc := &Service{}

	// Create a Risor proxy for the service
	proxy, err := object.NewProxy(svc)
	if err != nil {
		fmt.Println(red(err.Error()))
		os.Exit(1)
	}

	// Build up options for Risor, including the proxy as a variable named "svc"
	opts := []risor.Option{
		risor.WithDefaultBuiltins(),
		risor.WithBuiltins(map[string]object.Object{"svc": proxy}),
	}

	// Run the Risor code which can access the service as `svc`
	if _, err = risor.Eval(ctx, code, opts...); err != nil {
		fmt.Println(red(err.Error()))
		os.Exit(1)
	}

	fmt.Println(svc.GetMetrics())
}
