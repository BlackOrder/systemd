package systemd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const (
	// configFileMode defines standard permissions for system configuration files.
	// 0o644 allows read access for all users, write access for owner only.
	configFileMode = 0o644
)

// ServiceConfig holds the complete configuration for a systemd service.
// This struct defines all parameters needed to create, install, and manage
// a systemd service unit including logging and log rotation.
type ServiceConfig struct {
	// Required fields
	User        string // System user that owns the service process
	Group       string // Primary group for the service process
	UniqueName  string // Unique identifier for configuration files (no spaces)
	ServiceName string // Full systemd service name (e.g., "myapp.service")
	BinaryPath  string // Absolute path to the service executable

	// Optional fields
	LogDir      string // Directory for log files (empty to skip rsyslog/logrotate)
	SystemdFile string // Custom path for unit file (defaults to /etc/systemd/system/<ServiceName>)

	// Service customization
	ServiceLines  []string          // Additional lines to append to [Service] section
	MakeLogrotate bool              // Whether to generate logrotate configuration
	Streams       map[string]string // Map of stream names to log file names
}

// Manager handles installation and management of systemd services.
// It provides thread-safe operations for service lifecycle management
// with optional channel-based logging and error reporting.
type Manager struct {
	cfg      *ServiceConfig
	errChan  chan<- error
	infoChan chan<- string
}

// Option is a functional option for configuring Manager behavior.
type Option func(*Manager)

// WithErrorChan configures the Manager to send errors to the specified channel.
// Errors are sent non-blocking - if the channel is full, the error is dropped.
func WithErrorChan(ch chan<- error) Option {
	return func(m *Manager) { m.errChan = ch }
}

// WithInfoChan configures the Manager to send informational messages to the specified channel.
// Messages are sent non-blocking - if the channel is full, the message is dropped.
func WithInfoChan(ch chan<- string) Option {
	return func(m *Manager) { m.infoChan = ch }
}

// NewManager creates a new service Manager with the given configuration and options.
//
// If cfg.SystemdFile is empty, it defaults to /etc/systemd/system/<ServiceName>.
// If cfg.MakeLogrotate is true but cfg.LogDir is empty, MakeLogrotate is automatically disabled.
//
// The configuration is copied into the Manager, so subsequent modifications to the
// original ServiceConfig will not affect the Manager's behavior.
func NewManager(cfg *ServiceConfig, opts ...Option) *Manager {
	// Create a copy to avoid external modifications
	configCopy := *cfg

	// Set default SystemdFile path if not specified
	if configCopy.SystemdFile == "" {
		configCopy.SystemdFile = fmt.Sprintf("/etc/systemd/system/%s", configCopy.ServiceName)
	}

	// Disable logrotate if no log directory is specified
	if configCopy.MakeLogrotate && configCopy.LogDir == "" {
		configCopy.MakeLogrotate = false
	}

	m := &Manager{cfg: &configCopy}

	// Apply functional options
	for _, opt := range opts {
		opt(m)
	}

	return m
}

// Install performs complete service installation including user creation,
// configuration file generation, and service activation.
//
// The installation process:
//  1. Creates system user and group if they don't exist
//  2. Generates rsyslog configuration (if LogDir is specified)
//  3. Generates logrotate configuration (if MakeLogrotate is enabled)
//  4. Creates systemd unit file
//  5. Reloads systemd daemon configuration
//  6. Enables and starts the service
//
// Any failure during installation will halt the process and return an error.
// Partial installations may leave configuration files that should be cleaned
// up using Uninstall().
func (m *Manager) Install() error {
	c := m.cfg
	m.infof("Installing service: %s", c.ServiceName)

	// Ensure system user and group exist
	if err := ensureServiceUser(c.User, c.Group); err != nil {
		return m.fail(err)
	}
	m.infof("Service user and group ensured")

	// Configure logging if LogDir is specified
	if c.LogDir != "" {
		if err := writeRsyslogConf(c); err != nil {
			return m.fail(err)
		}
		m.infof("Rsyslog configuration written")

		if c.MakeLogrotate {
			if err := writeLogrotateConfs(c); err != nil {
				return m.fail(err)
			}
			m.infof("Logrotate configurations written")
		}
	}

	// Create systemd unit file
	if err := writeSystemdUnit(c); err != nil {
		return m.fail(err)
	}
	m.infof("Systemd unit file written")

	// Reload systemd configuration
	if err := execCommand("systemctl", "daemon-reload"); err != nil {
		return m.fail(err)
	}
	m.infof("Systemd daemon configuration reloaded")

	// Enable and start the service
	if err := execCommand("systemctl", "enable", "--now", c.ServiceName); err != nil {
		return m.fail(err)
	}
	m.infof("Service enabled and started successfully")

	return nil
}

// Uninstall removes the service and cleans up all associated configuration files.
//
// The uninstallation process:
//  1. Disables the service (ignores errors)
//  2. Stops the service (ignores errors)
//  3. Removes systemd unit file
//  4. Removes rsyslog configuration
//  5. Removes logrotate configuration files
//  6. Reloads systemd daemon configuration
//
// File removal operations are best-effort - missing files are ignored.
// Only the final daemon-reload operation can return an error.
func (m *Manager) Uninstall() error {
	c := m.cfg
	m.infof("Uninstalling service: %s", c.ServiceName)

	// Best-effort service shutdown
	_ = execCommand("systemctl", "disable", c.ServiceName)
	_ = execCommand("systemctl", "stop", c.ServiceName)

	// Clean up configuration files
	filesToRemove := []string{
		c.SystemdFile,
		rsyslogPath(c),
		logrotateCorePath(c) + "-*", // Glob pattern for logrotate files
	}

	for _, path := range filesToRemove {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			m.error(err)
		} else {
			m.infof("Removed: %s", path)
		}
	}

	// Reload systemd configuration
	if err := execCommand("systemctl", "daemon-reload"); err != nil {
		return m.fail(err)
	}
	m.infof("Systemd daemon configuration reloaded")

	return nil
}

// infof sends a formatted informational message to the info channel if configured.
// The send operation is non-blocking - if the channel is full, the message is dropped.
func (m *Manager) infof(format string, v ...interface{}) {
	if m.infoChan == nil {
		return
	}
	msg := fmt.Sprintf(format, v...)
	select {
	case m.infoChan <- msg:
	default: // Non-blocking send
	}
}

// error sends an error to the error channel if configured.
// The send operation is non-blocking - if the channel is full, the error is dropped.
func (m *Manager) error(err error) {
	if m.errChan == nil || err == nil {
		return
	}
	select {
	case m.errChan <- err:
	default: // Non-blocking send
	}
}

// fail sends an error to the error channel and returns it.
// This is a convenience function for error handling in installation/uninstallation.
func (m *Manager) fail(err error) error {
	m.error(err)
	return err
}

// ensureServiceUser creates the specified system user and group if they don't exist.
// Both user and group are created as system accounts with no home directory.
func ensureServiceUser(user, group string) error {
	// Check if user exists, create if not
	if _, err := execOutput("id", "-u", user); err != nil {
		if err := execCommand("useradd", "--system", "--no-create-home",
			"--shell", "/usr/sbin/nologin", user); err != nil {
			return fmt.Errorf("failed to create user %s: %w", user, err)
		}
	}

	// Check if group exists, create if not
	if _, err := execOutput("getent", "group", group); err != nil {
		if err := execCommand("groupadd", "--system", group); err != nil {
			return fmt.Errorf("failed to create group %s: %w", group, err)
		}
	}

	return nil
}

// writeSystemdUnit creates a systemd unit file with the service configuration.
// The generated unit file includes service description, dependencies, execution parameters,
// and any additional service lines specified in the configuration.
func writeSystemdUnit(c *ServiceConfig) error {
	// Prepare additional service configuration lines
	extraLines := ""
	if len(c.ServiceLines) > 0 {
		extraLines = strings.Join(c.ServiceLines, "\n") + "\n"
	}

	// Generate the complete unit file content
	unit := fmt.Sprintf(`[Unit]
Description=%s
After=network.target

[Service]
Type=notify
ExecStart=%s
Restart=on-failure
User=%s
Group=%s
%s[Install]
WantedBy=multi-user.target
`, c.UniqueName, c.BinaryPath, c.User, c.Group, extraLines)

	return os.WriteFile(c.SystemdFile, []byte(unit), configFileMode) // #nosec G306
}

// writeRsyslogConf creates an rsyslog configuration file for log stream routing.
// This configuration enables structured logging by routing messages containing
// 'stream=<name>' to specific log files with proper ownership and permissions.
func writeRsyslogConf(c *ServiceConfig) error {
	if len(c.Streams) == 0 {
		return nil // No streams configured
	}

	var configs []string
	for streamName, fileName := range c.Streams {
		streamConfig := fmt.Sprintf(`if $msg contains 'stream=%s' then {
	action(type="omfile" file="%s/%s" template="%s"
         dirCreateMode="0750" dirOwner="%s" dirGroup="%s"
		 fileCreateMode="0640" fileOwner="%s" fileGroup="%s")
	stop
}`, streamName, c.LogDir, fileName, c.UniqueName, c.User, c.Group, c.User, c.Group)
		configs = append(configs, streamConfig)
	}

	// Generate complete rsyslog configuration
	conf := fmt.Sprintf(`module(load="imuxsock")
module(load="imklog")
module(load="omfile")
template(name="%s" type="string" string="%%msg%%\n")
%s`, c.UniqueName, strings.Join(configs, "\n"))

	return os.WriteFile(rsyslogPath(c), []byte(conf), configFileMode) // #nosec G306
}

// writeLogrotateConfs creates logrotate configuration files for each log stream.
// Each stream gets its own logrotate configuration with weekly rotation,
// compression, and automatic cleanup of old log files.
func writeLogrotateConfs(c *ServiceConfig) error {
	if !c.MakeLogrotate || c.Streams == nil {
		return nil
	}

	for streamName, fileName := range c.Streams {
		logrotateConfig := fmt.Sprintf(`%s/%s {
	weekly
	rotate 8
	size 100M
	compress
	delaycompress
	missingok
	notifempty
	create 0640 %s %s
	sharedscripts
	postrotate
		systemctl kill -s HUP rsyslog.service
	endscript
}`, c.LogDir, fileName, c.User, c.Group)

		configPath := logrotateCorePath(c) + "-" + streamName
		if err := os.WriteFile(configPath, []byte(logrotateConfig), configFileMode); err != nil { // #nosec G306
			return fmt.Errorf("failed to write logrotate config for stream %s: %w", streamName, err)
		}
	}

	return nil
}

// rsyslogPath returns the file path for the rsyslog configuration.
func rsyslogPath(c *ServiceConfig) string {
	return fmt.Sprintf("/etc/rsyslog.d/%s.conf", c.UniqueName)
}

// logrotateCorePath returns the base file path for logrotate configurations.
// Individual stream configurations append "-{streamname}" to this path.
func logrotateCorePath(c *ServiceConfig) string {
	return fmt.Sprintf("/etc/logrotate.d/%s", c.UniqueName)
}

// execOutput executes a command and returns its combined stdout/stderr output.
func execOutput(cmd string, args ...string) ([]byte, error) {
	return exec.Command(cmd, args...).CombinedOutput()
}

// execCommand executes a command and returns an error if it fails.
// The error includes both the exit status and any output for debugging.
func execCommand(cmd string, args ...string) error {
	out, err := execOutput(cmd, args...)
	if err != nil {
		return fmt.Errorf("command '%s %s' failed: %w\nOutput: %s",
			cmd, strings.Join(args, " "), err, string(out))
	}
	return nil
}
