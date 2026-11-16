package main

import (
	"errors"
	"fmt"
	"testing"

	"github.com/llehouerou/waves/internal/app"
)

func TestRunErrorLine(t *testing.T) {
	cause := errors.New("open /no/such/folder: no such file or directory")
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "failed startup",
			err:  &app.StartupError{Err: cause},
			want: "Error initializing: open /no/such/folder: no such file or directory\n",
		},
		{
			name: "failed startup and failed close",
			err:  errors.Join(&app.StartupError{Err: cause}, errors.New("close the core: disk full")),
			want: "Error initializing: open /no/such/folder: no such file or directory\nError: close the core: disk full\n",
		},
		{
			name: "failed startup and failed program",
			err:  errors.Join(&app.StartupError{Err: cause}, errors.New("run the program: program was killed")),
			want: "Error initializing: open /no/such/folder: no such file or directory\nError: run the program: program was killed\n",
		},
		{
			name: "another error",
			err:  fmt.Errorf("close the core: %w", errors.New("disk full")),
			want: "Error: close the core: disk full\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := runErrorLine(tt.err); got != tt.want {
				t.Errorf("runErrorLine = %q, want %q", got, tt.want)
			}
		})
	}
}
