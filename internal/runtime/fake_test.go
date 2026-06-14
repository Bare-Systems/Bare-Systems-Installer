package runtime

import (
	"context"
	"errors"
	"strings"
)

var errFake = errors.New("fake command failure")

type FakeResult struct {
	Stdout string
	Stderr string
	Err    error
}

type FakeRunner struct {
	Results  map[string]FakeResult
	Commands []Command
}

func (r *FakeRunner) Run(_ context.Context, command Command) (Result, error) {
	r.Commands = append(r.Commands, command)
	key := command.Name
	if len(command.Args) > 0 {
		key += " " + strings.Join(command.Args, " ")
	}
	if result, ok := r.Results[key]; ok {
		return Result{Stdout: result.Stdout, Stderr: result.Stderr}, result.Err
	}
	return Result{}, nil
}
