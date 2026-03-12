package main

import (
	"os"
	"strings"
	"testing"
)

func TestValidatePort(t *testing.T) {
	tests := []struct {
		name    string
		port    int
		wantErr bool
	}{
		{
			name:    "valid default port",
			port:    9090,
			wantErr: false,
		},
		{
			name:    "valid min port",
			port:    1,
			wantErr: false,
		},
		{
			name:    "valid max port",
			port:    65535,
			wantErr: false,
		},
		{
			name:    "port below minimum",
			port:    0,
			wantErr: true,
		},
		{
			name:    "port above maximum",
			port:    65536,
			wantErr: true,
		},
		{
			name:    "negative port",
			port:    -1,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePort(tt.port)

			if tt.wantErr && err == nil {
				t.Errorf("validatePort(%d) expected error, got nil", tt.port)
			}

			if !tt.wantErr && err != nil {
				t.Errorf("validatePort(%d) unexpected error = %v", tt.port, err)
			}
		})
	}
}

func TestValidateRefreshInterval(t *testing.T) {
	tests := []struct {
		name    string
		input   int
		wantErr bool
	}{
		{
			name:    "valid default interval",
			input:   120,
			wantErr: false,
		},
		{
			name:    "valid minimum interval",
			input:   10,
			wantErr: false,
		},
		{
			name:    "valid large interval",
			input:   600,
			wantErr: false,
		},
		{
			name:    "interval below minimum",
			input:   5,
			wantErr: true,
		},
		{
			name:    "zero interval",
			input:   0,
			wantErr: true,
		},
		{
			name:    "negative interval",
			input:   -1,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRefreshInterval(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateRefreshInterval(%d) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestValidateUIRefreshInterval(t *testing.T) {
	tests := []struct {
		name    string
		input   int
		wantErr bool
	}{
		{
			name:    "valid default interval",
			input:   5,
			wantErr: false,
		},
		{
			name:    "valid minimum interval",
			input:   1,
			wantErr: false,
		},
		{
			name:    "valid large interval",
			input:   60,
			wantErr: false,
		},
		{
			name:    "zero interval",
			input:   0,
			wantErr: true,
		},
		{
			name:    "negative interval",
			input:   -1,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateUIRefreshInterval(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateUIRefreshInterval(%d) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestExpandPath_Main(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantHome bool
	}{
		{
			name:     "tilde expansion",
			input:    "~/.qwen/todos",
			wantHome: true,
		},
		{
			name:     "tilde with slash",
			input:    "~/test",
			wantHome: true,
		},
		{
			name:     "absolute path",
			input:    "/tmp/test",
			wantHome: false,
		},
		{
			name:     "relative path",
			input:    "test/path",
			wantHome: false,
		},
		{
			name:     "tilde only",
			input:    "~",
			wantHome: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandPath(tt.input)

			if tt.wantHome {
				home, _ := os.UserHomeDir()
				if !strings.HasPrefix(result, home) {
					t.Errorf("expandPath(%q) = %q, want prefix %q", tt.input, result, home)
				}
			}
		})
	}
}
