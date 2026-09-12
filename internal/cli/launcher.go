package cli

// OpenTerminal opens a terminal for user interaction.
// It is a wrapper around the platform-specific implementation of opening a terminal.
func OpenTerminal() error {
	return openTerminal()
}
