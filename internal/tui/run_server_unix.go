//go:build !windows

package tui

import (
	"os"
	"os/exec"
	"syscall"
)

func applySplitCmdSysProcAttr(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
}

func interruptServerProcess(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	pid := cmd.Process.Pid
	// Signal the whole process group so `sh -c '…vllm…'` and children receive SIGINT.
	if err := syscall.Kill(-pid, syscall.SIGINT); err != nil {
		return cmd.Process.Signal(os.Interrupt)
	}
	return nil
}
