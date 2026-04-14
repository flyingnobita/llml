//go:build windows

package tui

import (
	"os"
	"os/exec"
)

func applySplitCmdSysProcAttr(cmd *exec.Cmd) {}

func interruptServerProcess(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	return cmd.Process.Signal(os.Interrupt)
}
