package app

import "testing"

func TestParseCommand(t *testing.T) {
	tests := []struct {
		in     string
		want   CommandKind
		arg    string
		hasArg bool
	}{
		{"", CmdUnknown, "", false},
		{"   ", CmdUnknown, "", false},
		{"q", CmdQuit, "", false},
		{"quit", CmdQuit, "", false},
		{"wq", CmdQuitForce, "", false},
		{"qa", CmdQuitForce, "", false},
		{"qw", CmdQuitForce, "", false},
		{"ls", CmdBuffers, "", false},
		{"buffers", CmdBuffers, "", false},
		{"bn", CmdBufferNext, "", false},
		{"bnext", CmdBufferNext, "", false},
		{"bp", CmdBufferPrev, "", false},
		{"bprev", CmdBufferPrev, "", false},
		{"bd", CmdBufferDelete, "", false},
		{"bdelete", CmdBufferDelete, "", false},
		{"bd 5", CmdBufferDelete, "5", true},
		{"b 2", CmdBufferSwitch, "2", true},
		{"b2", CmdBufferSwitch, "2", true},
		{"b#", CmdBufferAlt, "", false},
		{"b #", CmdBufferAlt, "", false},
		{"vs", CmdVSplit, "", false},
		{"vsplit", CmdVSplit, "", false},
		{"vs 1", CmdVSplit, "1", true},
		{"vs Chats", CmdVSplit, "Chats", true},
		{"close", CmdClose, "", false},
		{"clo", CmdClose, "", false},
		{"b", CmdUnknown, "", false},     // bare :b without arg
		{"bogus", CmdUnknown, "", false}, // unknown verb
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			got := ParseCommand(tc.in)
			if got.Kind != tc.want {
				t.Errorf("ParseCommand(%q).Kind = %v, want %v", tc.in, got.Kind, tc.want)
			}
			if got.Arg != tc.arg {
				t.Errorf("ParseCommand(%q).Arg = %q, want %q", tc.in, got.Arg, tc.arg)
			}
			if got.HasArg != tc.hasArg {
				t.Errorf("ParseCommand(%q).HasArg = %v, want %v", tc.in, got.HasArg, tc.hasArg)
			}
		})
	}
}

func TestParseCommandTrimsWhitespace(t *testing.T) {
	if ParseCommand("  qa  ").Kind != CmdQuitForce {
		t.Fatal("leading/trailing spaces should be trimmed")
	}
}
