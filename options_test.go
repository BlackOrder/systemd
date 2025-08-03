package systemd

import (
	"fmt"
	"reflect"
	"sync"
	"testing"
)

const (
	numOperations = 10 // Number of operations for concurrent tests
)

// TestNewServiceConfig tests the NewServiceConfig function
func TestNewServiceConfig(t *testing.T) {
	tests := []struct {
		name     string
		user     string
		group    string
		bin      string
		logDir   string
		opts     []ServiceOpt
		expected ServiceConfig
	}{
		{
			name:   "basic config",
			user:   "testuser",
			group:  "testgroup",
			bin:    "/usr/bin/my-app",
			logDir: "/var/log/myapp",
			expected: ServiceConfig{
				User:        "testuser",
				Group:       "testgroup",
				BinaryPath:  "/usr/bin/my-app",
				LogDir:      "/var/log/myapp",
				UniqueName:  "bin-my-app",
				ServiceName: "bin-my-app.service",
				SystemdFile: "/etc/systemd/system/bin-my-app.service",
			},
		},
		{
			name:   "path with special characters",
			user:   "testuser",
			group:  "testgroup",
			bin:    "/opt/my@app/bin/my-app!",
			logDir: "/var/log/myapp",
			expected: ServiceConfig{
				User:        "testuser",
				Group:       "testgroup",
				BinaryPath:  "/opt/my@app/bin/my-app!",
				LogDir:      "/var/log/myapp",
				UniqueName:  "bin-my-app",
				ServiceName: "bin-my-app.service",
				SystemdFile: "/etc/systemd/system/bin-my-app.service",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewServiceConfig(tt.user, tt.group, tt.bin, tt.logDir, tt.opts...)

			// Compare relevant fields
			if result.User != tt.expected.User {
				t.Errorf("User: expected %s, got %s", tt.expected.User, result.User)
			}
			if result.Group != tt.expected.Group {
				t.Errorf("Group: expected %s, got %s", tt.expected.Group, result.Group)
			}
			if result.BinaryPath != tt.expected.BinaryPath {
				t.Errorf("BinaryPath: expected %s, got %s", tt.expected.BinaryPath, result.BinaryPath)
			}
			if result.LogDir != tt.expected.LogDir {
				t.Errorf("LogDir: expected %s, got %s", tt.expected.LogDir, result.LogDir)
			}
			if result.UniqueName != tt.expected.UniqueName {
				t.Errorf("UniqueName: expected %s, got %s", tt.expected.UniqueName, result.UniqueName)
			}
			if result.ServiceName != tt.expected.ServiceName {
				t.Errorf("ServiceName: expected %s, got %s", tt.expected.ServiceName, result.ServiceName)
			}
			if result.SystemdFile != tt.expected.SystemdFile {
				t.Errorf("SystemdFile: expected %s, got %s", tt.expected.SystemdFile, result.SystemdFile)
			}
		})
	}
}

// TestServiceOptions tests all service options
func TestServiceOptions(t *testing.T) {
	t.Run("WithWatchdog", func(t *testing.T) {
		cfg := ServiceConfig{}
		WithWatchdog("30s")(&cfg)

		expected := "WatchdogSec=30s"
		if len(cfg.ServiceLines) != 1 || cfg.ServiceLines[0] != expected {
			t.Errorf("Expected ServiceLines to contain '%s', got %v", expected, cfg.ServiceLines)
		}
	})

	t.Run("WithServiceLine", func(t *testing.T) {
		cfg := ServiceConfig{}
		WithServiceLine("CustomLine=value")(&cfg)

		expected := "CustomLine=value"
		if len(cfg.ServiceLines) != 1 || cfg.ServiceLines[0] != expected {
			t.Errorf("Expected ServiceLines to contain '%s', got %v", expected, cfg.ServiceLines)
		}
	})

	t.Run("WithJournal", func(t *testing.T) {
		cfg := ServiceConfig{}
		WithJournal()(&cfg)

		if len(cfg.ServiceLines) != 2 {
			t.Errorf("Expected 2 ServiceLines, got %d", len(cfg.ServiceLines))
		}

		expectedLines := []string{"StandardOutput=journal", "StandardError=journal"}
		for i, expected := range expectedLines {
			if cfg.ServiceLines[i] != expected {
				t.Errorf("Expected ServiceLines[%d] to be '%s', got '%s'", i, expected, cfg.ServiceLines[i])
			}
		}
	})

	t.Run("WithUMask", func(t *testing.T) {
		cfg := ServiceConfig{}
		WithUMask("0022")(&cfg)

		expected := "UMask=0022"
		if len(cfg.ServiceLines) != 1 || cfg.ServiceLines[0] != expected {
			t.Errorf("Expected ServiceLines to contain '%s', got %v", expected, cfg.ServiceLines)
		}
	})

	t.Run("WithLimitNOFILE", func(t *testing.T) {
		cfg := ServiceConfig{}
		WithLimitNOFILE("65536")(&cfg)

		expected := "LimitNOFILE=65536"
		if len(cfg.ServiceLines) != 1 || cfg.ServiceLines[0] != expected {
			t.Errorf("Expected ServiceLines to contain '%s', got %v", expected, cfg.ServiceLines)
		}
	})

	t.Run("WithExecReload", func(t *testing.T) {
		cfg := ServiceConfig{}
		WithExecReload("5", "30", "30")(&cfg)

		expectedLines := []string{
			"ExecReload=/bin/kill -HUP $MAINPID",
			"RestartSec=5",
			"KillSignal=SIGTERM",
			"TimeoutStartSec=30",
			"TimeoutStopSec=30",
		}

		if len(cfg.ServiceLines) != len(expectedLines) {
			t.Errorf("Expected %d ServiceLines, got %d", len(expectedLines), len(cfg.ServiceLines))
		}

		for i, expected := range expectedLines {
			if i < len(cfg.ServiceLines) && cfg.ServiceLines[i] != expected {
				t.Errorf("Expected ServiceLines[%d] to be '%s', got '%s'", i, expected, cfg.ServiceLines[i])
			}
		}
	})

	t.Run("WithNotifyAccess", func(t *testing.T) {
		cfg := ServiceConfig{}
		WithNotifyAccess()(&cfg)

		expected := "NotifyAccess=main"
		if len(cfg.ServiceLines) != 1 || cfg.ServiceLines[0] != expected {
			t.Errorf("Expected ServiceLines to contain '%s', got %v", expected, cfg.ServiceLines)
		}
	})

	t.Run("WithLogrotate", func(t *testing.T) {
		cfg := ServiceConfig{}
		WithLogrotate()(&cfg)

		if !cfg.MakeLogrotate {
			t.Error("Expected MakeLogrotate to be true")
		}
	})

	t.Run("WithStream", func(t *testing.T) {
		cfg := ServiceConfig{}
		WithStream("app", "app.log")(&cfg)

		if cfg.Streams == nil {
			t.Fatal("Expected Streams to be initialized")
		}

		if cfg.Streams["app"] != "app.log" {
			t.Errorf("Expected Streams['app'] to be 'app.log', got '%s'", cfg.Streams["app"])
		}
	})

	t.Run("WithStreams", func(t *testing.T) {
		cfg := ServiceConfig{}
		streams := map[string]string{
			"app":   "app.log",
			"error": "error.log",
		}
		WithStreams(streams)(&cfg)

		if cfg.Streams == nil {
			t.Fatal("Expected Streams to be initialized")
		}

		if !reflect.DeepEqual(cfg.Streams, streams) {
			t.Errorf("Expected Streams to be %v, got %v", streams, cfg.Streams)
		}
	})
}

// TestMultipleOptions tests applying multiple options
func TestMultipleOptions(t *testing.T) {
	cfg := NewServiceConfig(
		"testuser",
		"testgroup",
		"/usr/bin/test",
		"/var/log/test",
		WithWatchdog("30s"),
		WithJournal(),
		WithLogrotate(),
		WithStream("app", "app.log"),
		WithUMask("0022"),
	)

	// Check that all options were applied
	if !cfg.MakeLogrotate {
		t.Error("Expected MakeLogrotate to be true")
	}

	if cfg.Streams == nil || cfg.Streams["app"] != "app.log" {
		t.Error("Expected stream 'app' to be set")
	}

	expectedLines := []string{
		"WatchdogSec=30s",
		"StandardOutput=journal",
		"StandardError=journal",
		"UMask=0022",
	}

	if len(cfg.ServiceLines) != len(expectedLines) {
		t.Errorf("Expected %d ServiceLines, got %d", len(expectedLines), len(cfg.ServiceLines))
	}

	for i, expected := range expectedLines {
		if i < len(cfg.ServiceLines) && cfg.ServiceLines[i] != expected {
			t.Errorf("Expected ServiceLines[%d] to be '%s', got '%s'", i, expected, cfg.ServiceLines[i])
		}
	}
}

// TestSanitize tests the sanitize function
func TestSanitize(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{"with-dashes", "with-dashes"},
		{"with_underscores", "with_underscores"},
		{"with123numbers", "with123numbers"},
		{"with@special#chars!", "withspecialchars"},
		{"multiple!!!special@@@chars", "multiplespecialchars"},
		{"", ""},
		{"only-valid-chars_123", "only-valid-chars_123"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := sanitize(tt.input)
			if result != tt.expected {
				t.Errorf("sanitize(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestConcurrentOptionApplication tests concurrent application of options
func TestConcurrentOptionApplication(t *testing.T) {
	const numGoroutines = 100

	var wg sync.WaitGroup
	results := make([]ServiceConfig, numGoroutines)

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()

			// Apply multiple options concurrently
			cfg := NewServiceConfig(
				"testuser",
				"testgroup",
				"/usr/bin/test",
				"/var/log/test",
				WithWatchdog("30s"),
				WithJournal(),
				WithLogrotate(),
				WithStream("app", "app.log"),
			)

			results[id] = cfg
		}(i)
	}

	wg.Wait()

	// Verify all results are consistent
	expected := results[0]
	for i := 1; i < numGoroutines; i++ {
		if !configsEqual(&results[i], &expected) {
			t.Errorf("Configuration mismatch at index %d", i)
		}
	}
}

// TestRaceConditionsInStreams tests race conditions when modifying streams
func TestRaceConditionsInStreams(t *testing.T) {
	const numGoroutines = 50

	var wg sync.WaitGroup

	// Test concurrent creation of configs with streams (proper usage pattern)
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			// Each goroutine creates its own config - this is the intended usage
			cfg := NewServiceConfig(
				"testuser",
				"testgroup",
				"/usr/bin/test",
				"/var/log/test",
				WithStream(fmt.Sprintf("stream%d", id), fmt.Sprintf("file%d.log", id)),
			)

			// Verify the stream was set correctly
			if cfg.Streams == nil || cfg.Streams[fmt.Sprintf("stream%d", id)] != fmt.Sprintf("file%d.log", id) {
				t.Errorf("Stream not set correctly for goroutine %d", id)
			}
		}(i)
	}

	wg.Wait()
}

// TestConcurrentSanitize tests the sanitize function under concurrent load
func TestConcurrentSanitize(t *testing.T) {
	const numGoroutines = 100

	testCases := []string{
		"simple",
		"with@special#chars!",
		"multiple!!!special@@@chars",
		"with-dashes_and_underscores123",
	}

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				for _, input := range testCases {
					result := sanitize(input)
					// Basic sanity check - result should not contain special chars
					for _, char := range result {
						if !((char >= 'a' && char <= 'z') ||
							(char >= 'A' && char <= 'Z') ||
							(char >= '0' && char <= '9') ||
							char == '-' || char == '_') {
							t.Errorf("sanitize produced invalid character: %c in %s", char, result)
						}
					}
				}
			}
		}()
	}

	wg.Wait()
}

// Helper function to compare ServiceConfigs
func configsEqual(a, b *ServiceConfig) bool {
	return a.User == b.User &&
		a.Group == b.Group &&
		a.UniqueName == b.UniqueName &&
		a.ServiceName == b.ServiceName &&
		a.BinaryPath == b.BinaryPath &&
		a.LogDir == b.LogDir &&
		a.SystemdFile == b.SystemdFile &&
		a.MakeLogrotate == b.MakeLogrotate &&
		reflect.DeepEqual(a.ServiceLines, b.ServiceLines) &&
		reflect.DeepEqual(a.Streams, b.Streams)
}

// Benchmark tests for performance analysis
func BenchmarkNewServiceConfig(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NewServiceConfig("user", "group", "/usr/bin/test", "/var/log")
	}
}

func BenchmarkSanitize(b *testing.B) {
	input := "with@special#chars!and_more"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sanitize(input)
	}
}

func BenchmarkWithStream(b *testing.B) {
	cfg := ServiceConfig{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		WithStream("app", "app.log")(&cfg)
	}
}
