package app

import "strings"

// CommandKind enumerates the recognised vim-style ":" commands.
type CommandKind int

const (
	CmdUnknown CommandKind = iota
	CmdQuit                 // :q
	CmdQuitForce            // :wq, :qa, :qw — always exit the whole app
	CmdBuffers              // :ls, :buffers
	CmdBufferSwitch         // :b N
	CmdBufferAlt            // :b#
	CmdBufferNext           // :bn, :bnext
	CmdBufferPrev           // :bp, :bprev
	CmdBufferDelete         // :bd, :bdelete [N]
	CmdVSplit               // :vsplit, :vs [buffer]
	CmdClose                // :close
	CmdSet                  // :set <option>
)

// Command is a parsed ":" command line.
type Command struct {
	Kind   CommandKind
	Arg    string // raw argument (e.g. "2"); empty if none
	HasArg bool
}

// ParseCommand parses a colon-command body (without the leading ":").
// It tolerates both spaced and unspaced argument forms: ":b 2" and ":b2",
// ":b#" and ":b #".
func ParseCommand(s string) Command {
	s = strings.TrimSpace(s)
	if s == "" {
		return Command{Kind: CmdUnknown}
	}

	word, arg := splitFirst(s)

	// Forms where the argument may be glued to the verb: bN, b#, bdN.
	switch {
	case word == "b" || word == "buffer":
		return bufferArgCommand(arg)
	case strings.HasPrefix(word, "b#"):
		return Command{Kind: CmdBufferAlt}
	case strings.HasPrefix(word, "b") && isAllDigits(word[1:]):
		return Command{Kind: CmdBufferSwitch, Arg: word[1:], HasArg: true}
	}

	switch word {
	case "q", "quit":
		return Command{Kind: CmdQuit}
	case "wq", "qa", "qw", "qall", "quitall":
		return Command{Kind: CmdQuitForce}
	case "ls", "buffers":
		return Command{Kind: CmdBuffers}
	case "chats":
		// Alias for :b1 — the chat-list buffer always has id 1.
		return Command{Kind: CmdBufferSwitch, Arg: "1", HasArg: true}
	case "bn", "bnext":
		return Command{Kind: CmdBufferNext}
	case "bp", "bprev", "bprevious":
		return Command{Kind: CmdBufferPrev}
	case "bd", "bdelete":
		return bufferDeleteCommand(arg)
	case "vs", "vsplit", "vsp":
		return Command{Kind: CmdVSplit, Arg: arg, HasArg: arg != ""}
	case "close", "clo":
		return Command{Kind: CmdClose}
	case "set":
		return Command{Kind: CmdSet, Arg: arg, HasArg: arg != ""}
	}
	return Command{Kind: CmdUnknown}
}

// bufferArgCommand handles ":b <arg>" where arg is "#", a number, or empty.
func bufferArgCommand(arg string) Command {
	switch {
	case arg == "#":
		return Command{Kind: CmdBufferAlt}
	case arg != "" && isAllDigits(arg):
		return Command{Kind: CmdBufferSwitch, Arg: arg, HasArg: true}
	default:
		return Command{Kind: CmdUnknown}
	}
}

func bufferDeleteCommand(arg string) Command {
	if arg != "" && isAllDigits(arg) {
		return Command{Kind: CmdBufferDelete, Arg: arg, HasArg: true}
	}
	return Command{Kind: CmdBufferDelete}
}

func splitFirst(s string) (word, rest string) {
	if i := strings.IndexAny(s, " \t"); i >= 0 {
		return s[:i], strings.TrimSpace(s[i+1:])
	}
	return s, ""
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
