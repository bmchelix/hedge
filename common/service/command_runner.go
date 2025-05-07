package service

import "os/exec"

type CommandRunnerInterface interface {
	Run(cmd *exec.Cmd) error
	GetCmd(command string, args ...string) *exec.Cmd
}

type CommandRunner struct{}

func (c CommandRunner) Run(cmd *exec.Cmd) error {
	return cmd.Run()
}

func (c CommandRunner) GetCmd(command string, args ...string) *exec.Cmd {
	return exec.Command(command, args...)
}
