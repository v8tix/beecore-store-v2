//go:build e2e && unix

package e2e

import (
	"os/exec"
	"syscall"
)

// isolate puts the server in a process group of its own, so that a Ctrl-C
// to this test binary doesn't also interrupt Chrome mid-call -- the
// session closes it in an orderly way instead. Mirrors the same pattern
// used for the same reason in inside-agent-web-ui's devtools package.
func isolate(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killTree ends the MCP server and everything it started -- including
// Chrome, which chrome-devtools-mcp spawns as its own child, not ours.
// Killing just cmd.Process risks leaving Chrome (and its own
// renderer/GPU subprocesses) orphaned.
func killTree(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil {
		_ = cmd.Process.Kill()
	}
}
