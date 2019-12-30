package process

import (
	"os"
	"os/exec"
	"strings"
	"syscall"
)

type commandProcess struct {
	command *exec.Cmd
}

// NewCommand create a new command line process that manages itself
func NewCommand(command string) Process {
	cmdWithArgs := strings.Split(command, " ")
	cmd := exec.Command(cmdWithArgs[0], cmdWithArgs[1:]...)
	cmd.Stdout = os.Stdout

	return commandProcess{
		command: cmd,
	}
}

func (proc commandProcess) Start() error {
	return proc.command.Start()
}

func (proc commandProcess) Stop() error {
	return proc.command.Process.Signal(syscall.SIGQUIT)
}
