//go:build e2e && !unix

package e2e

import "os/exec"

// isolate and killTree have no real process-group implementation on a
// non-Unix platform -- harmless fallback, not a supported target for this
// test (the team develops on macOS/Linux; see k3d/local dev tooling).
func isolate(*exec.Cmd) {}

func killTree(cmd *exec.Cmd) {
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}
