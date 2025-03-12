package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetConfigDir(t *testing.T) {
	// Save original home dir
	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)

	// Set test home dir
	testHome := "/test/home"
	os.Setenv("HOME", testHome)

	expected := filepath.Join(testHome, ".config", ConfigDirName)
	got, err := GetConfigDir()
	if err != nil {
		t.Errorf("GetConfigDir() error = %v", err)
	}
	if got != expected {
		t.Errorf("GetConfigDir() = %v, want %v", got, expected)
	}

	// Test error case: unset HOME
	os.Unsetenv("HOME")
	_, err = GetConfigDir()
	if err == nil {
		t.Error("GetConfigDir() with unset HOME should return error")
	}
}

func TestGetSourcesDir(t *testing.T) {
	// Save original home dir
	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)

	// Set test home dir
	testHome := "/test/home"
	os.Setenv("HOME", testHome)

	expected := filepath.Join(testHome, ".config", ConfigDirName, SourcesDirName)
	got, err := GetSourcesDir()
	if err != nil {
		t.Errorf("GetSourcesDir() error = %v", err)
	}
	if got != expected {
		t.Errorf("GetSourcesDir() = %v, want %v", got, expected)
	}

	// Test error case: unset HOME
	os.Unsetenv("HOME")
	_, err = GetSourcesDir()
	if err == nil {
		t.Error("GetSourcesDir() with unset HOME should return error")
	}
}

func TestGetCacheDir(t *testing.T) {
	tests := []struct {
		name     string
		xdgCache string
		home     string
		want     string
		wantErr  bool
	}{
		{
			name:     "with XDG_CACHE_HOME",
			xdgCache: "/test/cache",
			home:     "/test/home",
			want:     "/test/cache/getgit",
		},
		{
			name:     "without XDG_CACHE_HOME",
			xdgCache: "",
			home:     "/test/home",
			want:     "/test/home/.cache/getgit",
		},
		{
			name:     "without HOME",
			xdgCache: "",
			home:     "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original env
			origCache := os.Getenv("XDG_CACHE_HOME")
			origHome := os.Getenv("HOME")
			defer func() {
				os.Setenv("XDG_CACHE_HOME", origCache)
				os.Setenv("HOME", origHome)
			}()

			// Set test env
			os.Setenv("XDG_CACHE_HOME", tt.xdgCache)
			os.Setenv("HOME", tt.home)

			got, err := GetCacheDir()
			if (err != nil) != tt.wantErr {
				t.Errorf("GetCacheDir() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("GetCacheDir() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	// Save original getwd function
	origGetwd := getwd
	defer func() { getwd = origGetwd }()

	tests := []struct {
		name    string
		setup   func(t *testing.T) (string, func())
		want    interface{} // Can be *Config or func(string) *Config
		wantErr bool
	}{
		{
			name: "valid config file",
			setup: func(t *testing.T) (string, func()) {
				tmpDir, err := os.MkdirTemp("", "getgit-test")
				if err != nil {
					t.Fatalf("Failed to create temp dir: %v", err)
				}
				cleanup := func() {
					os.RemoveAll(tmpDir)
					os.Setenv("HOME", os.Getenv("HOME"))
				}

				configDir := filepath.Join(tmpDir, ".config", ConfigDirName)
				if err := os.MkdirAll(configDir, 0755); err != nil {
					t.Fatalf("Failed to create config dir: %v", err)
				}

				configFile := filepath.Join(configDir, "config.yaml")
				if err := os.WriteFile(configFile, []byte("root: /test/tools"), 0644); err != nil {
					t.Fatalf("Failed to write config file: %v", err)
				}

				os.Setenv("HOME", tmpDir)
				return tmpDir, cleanup
			},
			want: &Config{Root: "/test/tools"},
		},
		{
			name: "invalid yaml",
			setup: func(t *testing.T) (string, func()) {
				tmpDir, err := os.MkdirTemp("", "getgit-test")
				if err != nil {
					t.Fatalf("Failed to create temp dir: %v", err)
				}
				cleanup := func() {
					os.RemoveAll(tmpDir)
					os.Setenv("HOME", os.Getenv("HOME"))
				}

				configDir := filepath.Join(tmpDir, ".config", ConfigDirName)
				if err := os.MkdirAll(configDir, 0755); err != nil {
					t.Fatalf("Failed to create config dir: %v", err)
				}

				configFile := filepath.Join(configDir, "config.yaml")
				if err := os.WriteFile(configFile, []byte("root: [invalid: yaml]"), 0644); err != nil {
					t.Fatalf("Failed to write config file: %v", err)
				}

				os.Setenv("HOME", tmpDir)
				return tmpDir, cleanup
			},
			wantErr: true,
		},
		{
			name: "no config file - creates default",
			setup: func(t *testing.T) (string, func()) {
				tmpDir, err := os.MkdirTemp("", "getgit-test")
				if err != nil {
					t.Fatalf("Failed to create temp dir: %v", err)
				}
				cleanup := func() {
					os.RemoveAll(tmpDir)
					os.Setenv("HOME", os.Getenv("HOME"))
				}

				configDir := filepath.Join(tmpDir, ".config", ConfigDirName)
				if err := os.MkdirAll(configDir, 0755); err != nil {
					t.Fatalf("Failed to create config dir: %v", err)
				}

				// Set up mock for getwd
				toolDir := filepath.Join(tmpDir, "tools", "sometool")
				getwd = func() (string, error) {
					return toolDir, nil
				}

				os.Setenv("HOME", tmpDir)
				return tmpDir, cleanup
			},
			want: func(tmpDir string) *Config {
				return &Config{Root: filepath.Join(tmpDir, "tools")}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir, cleanup := tt.setup(t)
			defer cleanup()

			got, err := LoadConfig()
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				var want *Config
				if w, ok := tt.want.(*Config); ok {
					want = w
				} else if f, ok := tt.want.(func(string) *Config); ok {
					want = f(tmpDir)
				}
				if got.Root != want.Root {
					t.Errorf("LoadConfig() = %v, want %v", got.Root, want.Root)
				}
			}
		})
	}
}

func TestGetWorkDir(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T) (string, func())
		want    string
		wantErr bool
	}{
		{
			name: "valid config",
			setup: func(t *testing.T) (string, func()) {
				tmpDir, err := os.MkdirTemp("", "getgit-test")
				if err != nil {
					t.Fatalf("Failed to create temp dir: %v", err)
				}
				cleanup := func() {
					os.RemoveAll(tmpDir)
					os.Setenv("HOME", os.Getenv("HOME"))
				}

				configDir := filepath.Join(tmpDir, ".config", ConfigDirName)
				if err := os.MkdirAll(configDir, 0755); err != nil {
					t.Fatalf("Failed to create config dir: %v", err)
				}

				configFile := filepath.Join(configDir, "config.yaml")
				if err := os.WriteFile(configFile, []byte("root: /test/work"), 0644); err != nil {
					t.Fatalf("Failed to write config file: %v", err)
				}

				os.Setenv("HOME", tmpDir)
				return "/test/work", cleanup
			},
			want: "/test/work",
		},
		{
			name: "invalid config file",
			setup: func(t *testing.T) (string, func()) {
				tmpDir, err := os.MkdirTemp("", "getgit-test")
				if err != nil {
					t.Fatalf("Failed to create temp dir: %v", err)
				}
				cleanup := func() {
					os.RemoveAll(tmpDir)
					os.Setenv("HOME", os.Getenv("HOME"))
				}

				configDir := filepath.Join(tmpDir, ".config", ConfigDirName)
				if err := os.MkdirAll(configDir, 0755); err != nil {
					t.Fatalf("Failed to create config dir: %v", err)
				}

				configFile := filepath.Join(configDir, "config.yaml")
				if err := os.WriteFile(configFile, []byte("invalid: [yaml"), 0644); err != nil {
					t.Fatalf("Failed to write config file: %v", err)
				}

				os.Setenv("HOME", tmpDir)
				return "", cleanup
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, cleanup := tt.setup(t)
			defer cleanup()

			got, err := GetWorkDir()
			if (err != nil) != tt.wantErr {
				t.Errorf("GetWorkDir() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("GetWorkDir() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetAliasFile(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T) (string, func())
		want    string
		wantErr bool
	}{
		{
			name: "valid config",
			setup: func(t *testing.T) (string, func()) {
				tmpDir, err := os.MkdirTemp("", "getgit-test")
				if err != nil {
					t.Fatalf("Failed to create temp dir: %v", err)
				}
				cleanup := func() {
					os.RemoveAll(tmpDir)
					os.Setenv("HOME", os.Getenv("HOME"))
				}

				configDir := filepath.Join(tmpDir, ".config", ConfigDirName)
				if err := os.MkdirAll(configDir, 0755); err != nil {
					t.Fatalf("Failed to create config dir: %v", err)
				}

				configFile := filepath.Join(configDir, "config.yaml")
				if err := os.WriteFile(configFile, []byte("root: /test/work"), 0644); err != nil {
					t.Fatalf("Failed to write config file: %v", err)
				}

				os.Setenv("HOME", tmpDir)
				return filepath.Join("/test/work", ".alias"), cleanup
			},
			want: "/test/work/.alias",
		},
		{
			name: "invalid config",
			setup: func(t *testing.T) (string, func()) {
				tmpDir, err := os.MkdirTemp("", "getgit-test")
				if err != nil {
					t.Fatalf("Failed to create temp dir: %v", err)
				}
				cleanup := func() {
					os.RemoveAll(tmpDir)
					os.Setenv("HOME", os.Getenv("HOME"))
				}

				configDir := filepath.Join(tmpDir, ".config", ConfigDirName)
				if err := os.MkdirAll(configDir, 0755); err != nil {
					t.Fatalf("Failed to create config dir: %v", err)
				}

				configFile := filepath.Join(configDir, "config.yaml")
				if err := os.WriteFile(configFile, []byte("invalid: [yaml"), 0644); err != nil {
					t.Fatalf("Failed to write config file: %v", err)
				}

				os.Setenv("HOME", tmpDir)
				return "", cleanup
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, cleanup := tt.setup(t)
			defer cleanup()

			got, err := GetAliasFile()
			if (err != nil) != tt.wantErr {
				t.Errorf("GetAliasFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("GetAliasFile() = %v, want %v", got, tt.want)
			}
		})
	}
}
