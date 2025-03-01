package main

import (
	"strings"
	"testing"
)

func TestSaveUploadedFiles(t *testing.T) {
	tests := []struct {
		name  string
		files []struct {
			filename string
			content  []byte
		}
		shouldFail bool
	}{
		{
			name: "successful save",
			files: []struct {
				filename string
				content  []byte
			}{
				{
					filename: "test1.json",
					content:  []byte("test1 content"),
				},
				{
					filename: "test2.json",
					content:  []byte("test2 content"),
				},
			},
			shouldFail: false,
		},
		{
			name: "filesystem error",
			files: []struct {
				filename string
				content  []byte
			}{
				{
					filename: "test1.json",
					content:  []byte("test1 content"),
				},
			},
			shouldFail: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockFS := NewMockFS()
			mockFS.ShouldFail = tt.shouldFail

			// Create mock uploaded files
			files := make([]UploadedFile, len(tt.files))
			for i, f := range tt.files {
				files[i] = NewMockUploadedFile(f.filename, f.content)
			}

			b := &Builder{
				appRoot:    "app",
				configsDir: "configs",
				buildDir:   "build",
				fs:         mockFS,
			}

			err := b.SaveUploadedFiles(files)

			if tt.shouldFail && err == nil {
				t.Error("expected error but got none")
			}

			if !tt.shouldFail {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}

				// Verify correct directory was created
				if len(mockFS.MkdirCalls) != 1 {
					t.Errorf("expected 1 mkdir call, got %d", len(mockFS.MkdirCalls))
				}

				// Verify files were created
				expectedCreateCalls := len(tt.files)
				if len(mockFS.CreateCalls) != expectedCreateCalls {
					t.Errorf("expected %d create calls, got %d", expectedCreateCalls, len(mockFS.CreateCalls))
				}
			}
		})
	}
}

func TestExecuteBuild(t *testing.T) {
	tests := []struct {
		name       string
		stdout     string
		stderr     string
		shouldFail bool
	}{
		{
			name:       "successful build",
			stdout:     "build successful\n",
			stderr:     "",
			shouldFail: false,
		},
		{
			name:       "build failure",
			stdout:     "",
			stderr:     "build failed\n",
			shouldFail: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRunner := &MockCommandRunner{
				ShouldFail: tt.shouldFail,
				Stdout:     tt.stdout,
				Stderr:     tt.stderr,
			}

			mockFactory := &MockCommandFactory{
				Runner: mockRunner,
			}

			mockFS := NewMockFS()

			b := &Builder{
				appRoot:    "app",
				buildDir:   "build",
				buildCmd:   "make build",
				cmdFactory: mockFactory,
				fs:         mockFS,
			}

			output, err := b.ExecuteBuild()

			if tt.shouldFail && err == nil {
				t.Error("expected error but got none")
			}

			if !tt.shouldFail {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}

				if !mockRunner.StartCalled {
					t.Error("Start was not called")
				}

				if !mockRunner.WaitCalled {
					t.Error("Wait was not called")
				}

				if !strings.Contains(string(output), tt.stdout) {
					t.Errorf("expected output to contain %q, got %q", tt.stdout, string(output))
				}
			}
		})
	}
}

func TestNormalizeFilename(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "standard format - unchanged",
			input:    "prefix-123.json",
			expected: "prefix-123.json",
		},
		{
			name:     "genesis format",
			input:    "genesis-123.json",
			expected: "123-genesis-l2.json",
		},
		{
			name:     "rollup format",
			input:    "rollup-456.json",
			expected: "456-rollup.json",
		},
		{
			name:     "no number",
			input:    "test.json",
			expected: "test.json",
		},
		{
			name:     "invalid number",
			input:    "prefix-abc.json",
			expected: "prefix-abc.json",
		},
		{
			name:     "no json extension",
			input:    "prefix-123.txt",
			expected: "prefix-123.txt",
		},
	}

	b := &Builder{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := b.normalizeFilename(tt.input)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}
