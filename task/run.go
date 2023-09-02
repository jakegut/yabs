package task

import (
	"fmt"
	"os"
	"os/exec"
)

type RunConfig struct {
	Cmd []string
	env map[string]string
	out string
}

func (r *RunConfig) WithEnv(key, value string) *RunConfig {
	r.env[key] = value
	return r
}

func (r *RunConfig) StdoutToFile(file string) *RunConfig {
	r.out = file
	return r
}

func (r *RunConfig) Exec() error {
	fd := os.Stdout
	if len(r.out) > 0 {
		var err error
		fd, err = os.Create(r.out)
		if err != nil {
			return fmt.Errorf("opening file: %s", err)
		}
		defer fd.Close()
	}

	cmd := exec.Command(r.Cmd[0], r.Cmd[1:]...)
	cmd.Stdout = fd
	cmd.Stderr = os.Stderr

	env := os.Environ()
	for k, v := range r.env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = env

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("cmd start: %s", err)
	}
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("cmd wait: %s", err)
	}
	return nil
}
