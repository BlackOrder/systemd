package systemd

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// TestNewManager tests the creation of a new Manager
func TestNewManager(t *testing.T) {
	cfg := ServiceConfig{
		User:        "testuser",
		Group:       "testgroup",
		UniqueName:  "test-service",
		ServiceName: "test-service.service",
		BinaryPath:  "/usr/bin/test",
	}

	// Test with default SystemdFile
	m := NewManager(&cfg)
	expectedPath := "/etc/systemd/system/test-service.service"
	if m.cfg.SystemdFile != expectedPath {
		t.Errorf("Expected SystemdFile %s, got %s", expectedPath, m.cfg.SystemdFile)
	}

	// Test with custom SystemdFile
	cfg.SystemdFile = "/custom/path/test.service"
	m = NewManager(&cfg)
	if m.cfg.SystemdFile != "/custom/path/test.service" {
		t.Errorf("Expected SystemdFile %s, got %s", "/custom/path/test.service", m.cfg.SystemdFile)
	}

	// Test MakeLogrotate auto-disable when LogDir is empty
	cfg.MakeLogrotate = true
	cfg.LogDir = ""
	m = NewManager(&cfg)
	if m.cfg.MakeLogrotate {
		t.Error("Expected MakeLogrotate to be disabled when LogDir is empty")
	}
}

// TestNewManagerWithOptions tests Manager creation with options
func TestNewManagerWithOptions(t *testing.T) {
	cfg := ServiceConfig{
		User:        "testuser",
		Group:       "testgroup",
		UniqueName:  "test-service",
		ServiceName: "test-service.service",
		BinaryPath:  "/usr/bin/test",
	}

	errChan := make(chan error, 10)
	infoChan := make(chan string, 10)

	m := NewManager(&cfg, WithErrorChan(errChan), WithInfoChan(infoChan))

	// Verify channels are set
	if m.errChan == nil {
		t.Error("Expected errChan to be set")
	}
	if m.infoChan == nil {
		t.Error("Expected infoChan to be set")
	}
}

// TestManagerChannelCommunication tests channel communication
func TestManagerChannelCommunication(t *testing.T) {
	cfg := ServiceConfig{
		User:        "testuser",
		Group:       "testgroup",
		UniqueName:  "test-service",
		ServiceName: "test-service.service",
		BinaryPath:  "/usr/bin/test",
	}

	errChan := make(chan error, 10)
	infoChan := make(chan string, 10)

	m := NewManager(&cfg, WithErrorChan(errChan), WithInfoChan(infoChan))

	// Test infof method
	m.infof("test message %s", "arg")
	select {
	case msg := <-infoChan:
		if msg != "test message arg" {
			t.Errorf("Expected 'test message arg', got '%s'", msg)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Expected to receive info message")
	}

	// Test error method
	testErr := fmt.Errorf("test error")
	m.error(testErr)
	select {
	case err := <-errChan:
		if err.Error() != "test error" {
			t.Errorf("Expected 'test error', got '%s'", err.Error())
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Expected to receive error message")
	}
}

// TestManagerChannelNonBlocking tests that channels don't block when full
func TestManagerChannelNonBlocking(t *testing.T) {
	cfg := ServiceConfig{
		User:        "testuser",
		Group:       "testgroup",
		UniqueName:  "test-service",
		ServiceName: "test-service.service",
		BinaryPath:  "/usr/bin/test",
	}

	// Create channels with no buffer
	errChan := make(chan error)
	infoChan := make(chan string)

	m := NewManager(&cfg, WithErrorChan(errChan), WithInfoChan(infoChan))

	// These should not block even though no one is reading from channels
	done := make(chan bool, 2)

	go func() {
		m.infof("test message")
		done <- true
	}()

	go func() {
		m.error(fmt.Errorf("test error"))
		done <- true
	}()

	// Wait for both goroutines to complete with timeout
	timeout := time.After(100 * time.Millisecond)
	count := 0
	for count < 2 {
		select {
		case <-done:
			count++
		case <-timeout:
			t.Error("Channel communication blocked unexpectedly")
			return
		}
	}
}

// TestWriteSystemdUnit tests systemd unit file generation
func TestWriteSystemdUnit(t *testing.T) {
	tempDir := t.TempDir()
	unitFile := filepath.Join(tempDir, "test.service")

	cfg := ServiceConfig{
		User:         "testuser",
		Group:        "testgroup",
		UniqueName:   "test-service",
		ServiceName:  "test-service.service",
		BinaryPath:   "/usr/bin/test",
		SystemdFile:  unitFile,
		ServiceLines: []string{"Environment=TEST=1", "TimeoutStopSec=30"},
	}

	err := writeSystemdUnit(&cfg)
	if err != nil {
		t.Fatalf("Failed to write systemd unit: %v", err)
	}

	// Read and verify file contents
	content, err := os.ReadFile(unitFile)
	if err != nil {
		t.Fatalf("Failed to read unit file: %v", err)
	}

	expected := "[Unit]\nDescription=test-service\nAfter=network.target\n\n[Service]\n" +
		"Type=notify\nExecStart=/usr/bin/test\nRestart=on-failure\nUser=testuser\n" +
		"Group=testgroup\nEnvironment=TEST=1\nTimeoutStopSec=30\n[Install]\n" +
		"WantedBy=multi-user.target\n"
	if string(content) != expected {
		t.Errorf("Unit file content mismatch.\nExpected:\n%s\nGot:\n%s", expected, string(content))
	}
}

// TestWriteRsyslogConf tests rsyslog configuration generation
func TestWriteRsyslogConf(t *testing.T) {
	cfg := ServiceConfig{
		User:       "testuser",
		Group:      "testgroup",
		UniqueName: "test-service",
		LogDir:     "/var/log/test",
		Streams: map[string]string{
			"app":   "app.log",
			"error": "error.log",
		},
	}

	// Test with empty streams
	emptyCfg := cfg
	emptyCfg.Streams = nil
	err := writeRsyslogConf(&emptyCfg)
	if err != nil {
		t.Fatalf("Failed to write rsyslog config with empty streams: %v", err)
	}

	// Note: Since rsyslogPath writes to /etc/rsyslog.d/, we can't easily test
	// the actual file writing in unit tests without root privileges.
	// This test validates the function doesn't crash with valid input.
}

// TestWriteLogrotateConfs tests logrotate configuration generation
func TestWriteLogrotateConfs(t *testing.T) {
	cfg := ServiceConfig{
		User:          "testuser",
		Group:         "testgroup",
		UniqueName:    "test-service",
		LogDir:        "/var/log/test",
		MakeLogrotate: true,
		Streams: map[string]string{
			"app":   "app.log",
			"error": "error.log",
		},
	}

	// Test with MakeLogrotate disabled
	disabledCfg := cfg
	disabledCfg.MakeLogrotate = false
	err := writeLogrotateConfs(&disabledCfg)
	if err != nil {
		t.Fatalf("Failed with MakeLogrotate disabled: %v", err)
	}

	// Test with nil streams
	nilStreamsCfg := cfg
	nilStreamsCfg.Streams = nil
	err = writeLogrotateConfs(&nilStreamsCfg)
	if err != nil {
		t.Fatalf("Failed with nil streams: %v", err)
	}

	// Note: Since logrotateCorePath writes to /etc/logrotate.d/, we can't easily test
	// the actual file writing in unit tests without root privileges.
	// This test validates the function doesn't crash with valid input.
}

// TestRaceConditions tests for race conditions in concurrent usage
func TestRaceConditions(t *testing.T) {
	const numGoroutines = 100
	const numOperations = 10

	cfg := ServiceConfig{
		User:        "testuser",
		Group:       "testgroup",
		UniqueName:  "test-service",
		ServiceName: "test-service.service",
		BinaryPath:  "/usr/bin/test",
	}

	errChan := make(chan error, numGoroutines*numOperations)
	infoChan := make(chan string, numGoroutines*numOperations)

	var wg sync.WaitGroup

	// Test concurrent Manager creation
	t.Run("ConcurrentManagerCreation", func(t *testing.T) {
		wg.Add(numGoroutines)
		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer wg.Done()
				for j := 0; j < numOperations; j++ {
					testCfg := cfg
					testCfg.UniqueName = fmt.Sprintf("test-service-%d-%d", id, j)
					testCfg.ServiceName = fmt.Sprintf("test-service-%d-%d.service", id, j)

					m := NewManager(&testCfg, WithErrorChan(errChan), WithInfoChan(infoChan))
					if m == nil {
						t.Errorf("NewManager returned nil for goroutine %d, operation %d", id, j)
					}
				}
			}(i)
		}
		wg.Wait()
	})

	// Test concurrent channel operations
	t.Run("ConcurrentChannelOperations", func(t *testing.T) {
		m := NewManager(&cfg, WithErrorChan(errChan), WithInfoChan(infoChan))

		wg.Add(numGoroutines * 2) // *2 for info and error operations

		// Concurrent info operations
		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer wg.Done()
				for j := 0; j < numOperations; j++ {
					m.infof("test message %d-%d", id, j)
				}
			}(i)
		}

		// Concurrent error operations
		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer wg.Done()
				for j := 0; j < numOperations; j++ {
					m.error(fmt.Errorf("test error %d-%d", id, j))
				}
			}(i)
		}

		wg.Wait()

		// Drain channels to verify all messages were sent
		close(errChan)
		close(infoChan)

		errorCount := 0
		for range errChan {
			errorCount++
		}

		infoCount := 0
		for range infoChan {
			infoCount++
		}

		expectedCount := numGoroutines * numOperations
		if errorCount != expectedCount {
			t.Errorf("Expected %d error messages, got %d", expectedCount, errorCount)
		}
		if infoCount != expectedCount {
			t.Errorf("Expected %d info messages, got %d", expectedCount, infoCount)
		}
	})
}

// TestConcurrentFileOperations tests concurrent file writing operations
func TestConcurrentFileOperations(t *testing.T) {
	const numGoroutines = 50
	tempDir := t.TempDir()

	cfg := ServiceConfig{
		User:        "testuser",
		Group:       "testgroup",
		UniqueName:  "test-service",
		ServiceName: "test-service.service",
		BinaryPath:  "/usr/bin/test",
		LogDir:      "/var/log/test",
		Streams: map[string]string{
			"app": "app.log",
		},
	}

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Test concurrent systemd unit file writing
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			testCfg := cfg
			testCfg.SystemdFile = filepath.Join(tempDir, fmt.Sprintf("test-%d.service", id))

			err := writeSystemdUnit(&testCfg)
			if err != nil {
				t.Errorf("Failed to write systemd unit for goroutine %d: %v", id, err)
			}
		}(i)
	}

	wg.Wait()

	// Verify all files were created
	for i := 0; i < numGoroutines; i++ {
		filename := filepath.Join(tempDir, fmt.Sprintf("test-%d.service", i))
		if _, err := os.Stat(filename); os.IsNotExist(err) {
			t.Errorf("Service file %s was not created", filename)
		}
	}
}

// Benchmark tests for performance analysis
func BenchmarkNewManager(b *testing.B) {
	cfg := ServiceConfig{
		User:        "testuser",
		Group:       "testgroup",
		UniqueName:  "test-service",
		ServiceName: "test-service.service",
		BinaryPath:  "/usr/bin/test",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NewManager(&cfg)
	}
}

func BenchmarkManagerInfo(b *testing.B) {
	cfg := ServiceConfig{
		User:        "testuser",
		Group:       "testgroup",
		UniqueName:  "test-service",
		ServiceName: "test-service.service",
		BinaryPath:  "/usr/bin/test",
	}

	infoChan := make(chan string, 1000)
	m := NewManager(&cfg, WithInfoChan(infoChan))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.infof("test message %d", i)
	}
}
