package cli

import (
	"os"
	"os/exec"

	log "github.com/sirupsen/logrus"
)

type GenericExecutor interface {
	Run(*map[string]string) (string, error)
	SetCommand(string)
	SetArgs([]string)
}

type MockExecutor struct {
	Command    string
	Args       []string
	MockOutput string
	MockError  error
}

func (e *MockExecutor) SetCommand(cmd string) {
	e.Command = cmd
}

func (e *MockExecutor) SetArgs(args []string) {
	e.Args = args
}

func (e *MockExecutor) Run(*map[string]string) (string, error) {
	return e.MockOutput, e.MockError
}

type Executor struct {
	Command string
	Args    []string
}

func (e *Executor) SetCommand(cmd string) {
	e.Command = cmd
}

func (e *Executor) SetArgs(args []string) {
	e.Args = args
}

func (e *Executor) Run(env *map[string]string) (string, error) {
	log.Infof("Executor running command=%s, args=%v", e.Command, e.Args)
	cmd := exec.Command(e.Command, e.Args...)
	envs := map[string]string{}
	if env != nil {
		envs = *env
	}
	for k, v := range envs {
		cmd.Env = append(os.Environ(), "%s=%s", k, v)
	}
	output, err := cmd.CombinedOutput()
	return string(output), err
}
