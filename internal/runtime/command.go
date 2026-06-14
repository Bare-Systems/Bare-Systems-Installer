package runtime

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

const ComposeProjectName = "bare-systems"

type Command struct {
	Name string
	Args []string
	Dir  string
}

func (c Command) String() string {
	if len(c.Args) == 0 {
		return c.Name
	}
	return c.Name + " " + strings.Join(c.Args, " ")
}

type Result struct {
	Stdout string
	Stderr string
}

type Runner interface {
	Run(ctx context.Context, command Command) (Result, error)
}

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, command Command) (Result, error) {
	cmd := exec.CommandContext(ctx, command.Name, command.Args...)
	cmd.Dir = command.Dir

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	return Result{Stdout: stdout.String(), Stderr: stderr.String()}, err
}

func IsCommandNotFound(err error) bool {
	if err == nil {
		return false
	}
	var execErr *exec.Error
	return errors.As(err, &execErr) && errors.Is(execErr.Err, exec.ErrNotFound)
}

func CommandError(command Command, result Result, err error) error {
	if err == nil {
		return nil
	}
	if result.Stderr != "" {
		return fmt.Errorf("%s failed: %s", command.String(), strings.TrimSpace(result.Stderr))
	}
	if result.Stdout != "" {
		return fmt.Errorf("%s failed: %s", command.String(), strings.TrimSpace(result.Stdout))
	}
	return fmt.Errorf("%s failed: %w", command.String(), err)
}
