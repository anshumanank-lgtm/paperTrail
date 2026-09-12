//go:build linux

package cli

import (
	"fmt"
	"os"
	"os/exec"
)

// openTerminal opens a terminal for user interaction on Linux.
// It checks if the application is running in Windows Subsystem for Linux (WSL) and uses the appropriate terminal command.
// If not running in WSL, it attempts to open a terminal emulator (gnome-terminal, konsole, xfce4-terminal, or xterm) with the current executable and the "--cli" flag.
func openTerminal() error {
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}

	if isWSL() {
		return openWSLTerminal(executable)
	}

	return openLinuxTerminal(executable)
}

// isWSL checks if the application is running in Windows Subsystem for Linux (WSL) by reading the /proc/version file
// and looking for "microsoft" or "wsl" in the content.
func isWSL() bool {
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}

	version := string(data)

	return containsIgnoreCase(version, "microsoft") ||
		containsIgnoreCase(version, "wsl")
}

// openWSLTerminal opens a terminal for user interaction in Windows Subsystem for Linux (WSL).
// It uses the Windows Terminal (wt.exe) to launch a new tab with the current executable and the "--cli" flag.
func openWSLTerminal(executable string) error {
	cmd := exec.Command(
		"wt.exe",
		"wsl.exe",
		executable,
		"--cli",
	)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start Windows Terminal: %w", err)
	}

	return nil
}

// openLinuxTerminal opens a terminal for user interaction on Linux.
// It attempts to find a supported terminal emulator (gnome-terminal, konsole, xfce4-terminal, or xterm) and
// launches it with the current executable and the "--cli" flag.
func openLinuxTerminal(executable string) error {
	terminals := []struct {
		command string
		args    []string
	}{
		{
			"gnome-terminal",
			[]string{"--", executable, "--cli"},
		},
		{
			"konsole",
			[]string{"-e", executable, "--cli"},
		},
		{
			"xfce4-terminal",
			[]string{"--command", executable + " --cli"},
		},
		{
			"xterm",
			[]string{"-e", executable, "--cli"},
		},
	}

	for _, terminal := range terminals {
		if _, err := exec.LookPath(terminal.command); err != nil {
			continue
		}

		cmd := exec.Command(
			terminal.command,
			terminal.args...,
		)

		if err := cmd.Start(); err != nil {
			return fmt.Errorf(
				"start %s: %w",
				terminal.command,
				err,
			)
		}

		return nil
	}

	return fmt.Errorf(
		"no supported terminal emulator found",
	)
}

// containsIgnoreCase checks if the target string is present in the source string, ignoring case differences.
func containsIgnoreCase(s, target string) bool {
	return len(s) >= len(target) &&
		containsFold(s, target)
}

// containsFold checks if the target string is present in the source string, ignoring case differences.
func containsFold(s, target string) bool {
	for i := 0; i+len(target) <= len(s); i++ {
		match := true

		for j := range target {
			a := s[i+j]
			b := target[j]

			if a >= 'A' && a <= 'Z' {
				a += 'a' - 'A'
			}

			if b >= 'A' && b <= 'Z' {
				b += 'a' - 'A'
			}

			if a != b {
				match = false
				break
			}
		}

		if match {
			return true
		}
	}

	return false
}
