package process

import (
	"os/exec"
	"strings"
	"syscall"
)

type commandProcess struct {
	command   *exec.Cmd
	isRunning bool
}

// NewCommand create a new command line process that manages itself
func NewCommand(command string) Process {
	cmdWithArgs := strings.Split(command, " ")
	cmd := exec.Command(cmdWithArgs[0], cmdWithArgs[1:]...)
	// cmd.Stdout = os.Stdout

	return commandProcess{
		command:   cmd,
		isRunning: false,
	}
}

func (proc commandProcess) Start() error {
	if err := proc.command.Start(); err != nil {
		return err
	}

	proc.isRunning = true

	return nil
}

func (proc commandProcess) Stop() error {
	p := proc.command.Process
	if p == nil {
		return nil
	}

	if err := p.Signal(syscall.SIGQUIT); err != nil {
		return err
	}

	proc.isRunning = false

	return nil
}

func (proc commandProcess) IsRunning() bool {
	return proc.isRunning
}
