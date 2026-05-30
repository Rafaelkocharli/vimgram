package app

import "testing"

func TestSelfDisplayName(t *testing.T) {
	tests := []struct {
		self Self
		want string
	}{
		{Self{FirstName: "Ada", LastName: "Lovelace"}, "Ada Lovelace"},
		{Self{FirstName: "Ada"}, "Ada"},
		{Self{LastName: "Lovelace"}, "Lovelace"},
		{Self{}, "you"},
	}
	for _, tc := range tests {
		if got := tc.self.DisplayName(); got != tc.want {
			t.Errorf("DisplayName(%+v) = %q, want %q", tc.self, got, tc.want)
		}
	}
}

func TestVimModeLabel(t *testing.T) {
	tests := map[VimMode]string{
		ModeNormal:  " NORMAL ",
		ModeVisual:  " VISUAL ",
		ModeEdit:    " INSERT ",
		ModeCommand: " COMMAND ",
	}
	for mode, want := range tests {
		if got := mode.Label(); got != want {
			t.Errorf("VimMode(%d).Label() = %q, want %q", mode, got, want)
		}
	}
}
