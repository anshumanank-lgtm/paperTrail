//go:build windows

package cli

import (
	"fmt"
	"os"
	"os/exec"
)

// openTerminal opens a terminal for user interaction on Windows.
// It uses the Windows Terminal (wt.exe) to launch a new tab with the current executable and the "--cli" flag.
func openTerminal() error {
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}

	cmd := exec.Command(
		"wt.exe",
		"new-tab",
		executable,
		"--cli",
	)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf(
			"start Windows Terminal: %w",
			err,
		)
	}

	return nil
}
