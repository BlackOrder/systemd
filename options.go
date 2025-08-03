package systemd

import (
	"fmt"
	"maps"
	"path/filepath"
	"regexp"
)

// ServiceOpt is a functional option that modifies a ServiceConfig.
// These options provide a flexible way to configure service parameters
// while maintaining immutable configuration objects.
type ServiceOpt func(*ServiceConfig)

// WithWatchdog configures the systemd watchdog timer for the service.
// The sec parameter should be a valid systemd time span (e.g., "30s", "2min").
func WithWatchdog(sec string) ServiceOpt {
	return func(c *ServiceConfig) {
		c.ServiceLines = append(c.ServiceLines, fmt.Sprintf("WatchdogSec=%s", sec))
	}
}

// WithServiceLine appends a custom line to the [Service] section of the unit file.
// This allows adding any systemd service directive not covered by specific options.
func WithServiceLine(line string) ServiceOpt {
	return func(c *ServiceConfig) {
		c.ServiceLines = append(c.ServiceLines, line)
	}
}

// WithJournal configures the service to route stdout/stderr to systemd journal.
// This sets StandardOutput=journal and StandardError=journal directives.
func WithJournal() ServiceOpt {
	return func(c *ServiceConfig) {
		c.ServiceLines = append(c.ServiceLines,
			"StandardOutput=journal",
			"StandardError=journal")
	}
}

// WithUMask sets the file mode creation mask for the service.
// The umask parameter should be an octal string (e.g., "0022").
func WithUMask(umask string) ServiceOpt {
	return func(c *ServiceConfig) {
		c.ServiceLines = append(c.ServiceLines, fmt.Sprintf("UMask=%s", umask))
	}
}

// WithLimitNOFILE sets the maximum number of open file descriptors for the service.
// The limit parameter can be a number or "infinity".
func WithLimitNOFILE(limit string) ServiceOpt {
	return func(c *ServiceConfig) {
		c.ServiceLines = append(c.ServiceLines, fmt.Sprintf("LimitNOFILE=%s", limit))
	}
}

// WithExecReload configures service reload and timeout behavior.
// Parameters: restart (RestartSec), start (TimeoutStartSec), stop (TimeoutStopSec).
func WithExecReload(restart, start, stop string) ServiceOpt {
	return func(c *ServiceConfig) {
		c.ServiceLines = append(c.ServiceLines,
			"ExecReload=/bin/kill -HUP $MAINPID",
			fmt.Sprintf("RestartSec=%s", restart),
			"KillSignal=SIGTERM",
			fmt.Sprintf("TimeoutStartSec=%s", start),
			fmt.Sprintf("TimeoutStopSec=%s", stop))
	}
}

// WithNotifyAccess configures systemd readiness notification access.
// Sets NotifyAccess=main to allow the main process to send readiness notifications.
func WithNotifyAccess() ServiceOpt {
	return func(c *ServiceConfig) {
		c.ServiceLines = append(c.ServiceLines, "NotifyAccess=main")
	}
}

// WithLogrotate enables automatic log rotation for service log files.
// This only has effect if LogDir is also specified in the ServiceConfig.
func WithLogrotate() ServiceOpt {
	return func(c *ServiceConfig) {
		c.MakeLogrotate = true
	}
}

// WithStream adds a named log stream that will be configured for rsyslog routing.
// The name parameter is used for filtering log messages, and file is the target log file name.
func WithStream(name, file string) ServiceOpt {
	return func(c *ServiceConfig) {
		if c.Streams == nil {
			c.Streams = make(map[string]string)
		}
		c.Streams[name] = file
	}
}

// WithStreams configures multiple log streams at once by copying from the provided map.
// This is equivalent to calling WithStream for each key-value pair.
func WithStreams(streams map[string]string) ServiceOpt {
	return func(c *ServiceConfig) {
		if c.Streams == nil {
			c.Streams = make(map[string]string)
		}
		maps.Copy(c.Streams, streams)
	}
}

// NewServiceConfig creates a ServiceConfig with reasonable defaults and applies the given options.
// It automatically generates UniqueName and ServiceName based on the binary path.
//
// Parameters:
//   - user: System user to run the service
//   - group: System group for the service
//   - bin: Absolute path to the service binary
//   - logDir: Directory for log files (empty to disable logging)
//   - opts: Additional service configuration options
//
// The UniqueName is generated as "{parent-dir}-{binary-name}" and ServiceName as "{UniqueName}.service".
func NewServiceConfig(user, group, bin, logDir string, opts ...ServiceOpt) ServiceConfig {
	// Generate unique identifiers from binary path
	baseName := sanitize(filepath.Base(bin))
	dirName := sanitize(filepath.Base(filepath.Dir(bin)))
	uniqueName := fmt.Sprintf("%s-%s", dirName, baseName)
	serviceName := uniqueName + ".service"

	// Create base configuration
	config := ServiceConfig{
		User:        user,
		Group:       group,
		BinaryPath:  bin,
		LogDir:      logDir,
		UniqueName:  uniqueName,
		ServiceName: serviceName,
		SystemdFile: "/etc/systemd/system/" + serviceName,
	}

	// Apply functional options
	for _, opt := range opts {
		opt(&config)
	}

	return config
}

// sanitize removes invalid characters from names to make them safe for use in file paths and service names.
// Only letters, numbers, dashes, and underscores are preserved.
func sanitize(s string) string {
	return regexp.MustCompile(`[^a-zA-Z0-9_-]`).ReplaceAllString(s, "")
}
