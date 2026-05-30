package app

import "strings"

// Command is the parsed result of a vim-style ":xxx" line.
type Command int

const (
	CmdUnknown   Command = iota
	CmdQuit              // :q
	CmdQuitForce         // :wq, :qa, :qw — always exit the whole app
)

// ParseCommand parses a colon-command body (without the leading ":").
func ParseCommand(s string) Command {
	switch strings.TrimSpace(s) {
	case "q":
		return CmdQuit
	case "wq", "qa", "qw":
		return CmdQuitForce
	default:
		return CmdUnknown
	}
}
