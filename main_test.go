package main

import "testing"

func TestConfigPathFromArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"default", nil, ".quickserverc"},
		{"short separate", []string{"-config", "local.rc"}, "local.rc"},
		{"long separate", []string{"--config", "local.rc"}, "local.rc"},
		{"short equals", []string{"-config=local.rc"}, "local.rc"},
		{"long equals", []string{"--config=local.rc"}, "local.rc"},
		{"empty disables", []string{"-config="}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := configPathFromArgs(tt.args); got != tt.want {
				t.Fatalf("configPathFromArgs() = %q, want %q", got, tt.want)
			}
		})
	}
}
