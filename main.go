package systemd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// ServiceConfig holds settings for the systemd service.
type ServiceConfig struct {
	// Mandatory
	User        string // system user that owns the process
	Group       string // primary group
	UniqueName  string // slug to identify files (no spaces)
	ServiceName string // e.g. my‑app.service
	BinaryPath  string // absolute path to executable

	// Optional
	LogDir      string // if empty, rsyslog/logrotate files are skipped
	SystemdFile string // defaults to /etc/systemd/system/<ServiceName>

	// Customisation
	ServiceLines  []string          // raw lines appended to [Service]
	MakeLogrotate bool              // generate logrotate for core log
	Streams       map[string]string // map of stream names to log file names
}

// Manager controls installation and uninstallation of a systemd service.
type Manager struct {
	cfg      ServiceConfig
	errChan  chan<- error
	infoChan chan<- string
}

// Option customizes the Manager.
type Option func(*Manager)

// WithErrorChan sets a channel to receive errors.
func WithErrorChan(ch chan<- error) Option {
	return func(m *Manager) { m.errChan = ch }
}

// WithInfoChan sets a channel to receive informational messages.
func WithInfoChan(ch chan<- string) Option {
	return func(m *Manager) { m.infoChan = ch }
}

// NewManager creates a Manager. If cfg.SystemdFile is empty it defaults to
// /etc/systemd/system/<ServiceName>. Optional generation flags are set to
// sensible defaults.
func NewManager(cfg ServiceConfig, opts ...Option) *Manager {
	if cfg.SystemdFile == "" {
		cfg.SystemdFile = fmt.Sprintf("/etc/systemd/system/%s", cfg.ServiceName)
	}

	if cfg.MakeLogrotate && cfg.LogDir == "" {
		cfg.MakeLogrotate = false
	}

	m := &Manager{cfg: cfg}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

/* --------------------------------------------------------------------
   Public API
-------------------------------------------------------------------- */

// Install performs service installation.
func (m *Manager) Install() error {
	c := m.cfg
	m.info("installing service: %s", c.ServiceName)

	if err := ensureServiceUser(c.User, c.Group); err != nil {
		return m.fail(err)
	}
	m.info("service user/group ensured")

	if c.LogDir != "" {
		if err := writeRsyslogConf(c); err != nil {
			return m.fail(err)
		}
		m.info("rsyslog config written")

		if c.MakeLogrotate {
			if err := writeLogrotateConfs(c); err != nil {
				return m.fail(err)
			}
			m.info("logrotate configs written")
		}
	}

	if err := writeSystemdUnit(c); err != nil {
		return m.fail(err)
	}
	m.info("systemd unit file written")

	if err := execCommand("systemctl", "daemon-reload"); err != nil {
		return m.fail(err)
	}
	m.info("daemons reloaded")

	if err := execCommand("systemctl", "enable", "--now", c.ServiceName); err != nil {
		return m.fail(err)
	}
	m.info("service enabled and started")

	return nil
}

// Uninstall removes the installed service.
func (m *Manager) Uninstall() error {
	c := m.cfg
	m.info("uninstalling service: %s", c.ServiceName)

	_ = execCommand("systemctl", "disable", c.ServiceName)
	_ = execCommand("systemctl", "stop", c.ServiceName)

	paths := []string{
		c.SystemdFile,
		rsyslogPath(c),
		logrotateCorePath(c) + "-*",
	}
	for _, p := range paths {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			m.error(err)
		} else {
			m.info("removed %s", p)
		}
	}

	if err := execCommand("systemctl", "daemon-reload"); err != nil {
		return m.fail(err)
	}
	m.info("daemons reloaded")

	return nil
}

// --- internal helpers -------------------------------------------------------

func (m *Manager) info(format string, v ...interface{}) {
	if m.infoChan == nil {
		return
	}
	msg := fmt.Sprintf(format, v...)
	select {
	case m.infoChan <- msg:
	default:
	}
}

func (m *Manager) error(err error) {
	if m.errChan == nil || err == nil {
		return
	}
	select {
	case m.errChan <- err:
	default:
	}
}

func (m *Manager) fail(err error) error {
	m.error(err)
	return err
}

/* ------------------- OS‑level helpers ------------------------------------ */

func ensureServiceUser(user, group string) error {
	if _, err := execOutput("id", "-u", user); err != nil {
		if err := execCommand("useradd", "--system", "--no-create-home",
			"--shell", "/usr/sbin/nologin", user); err != nil {
			return err
		}
	}
	if _, err := execOutput("getent", "group", group); err != nil {
		if err := execCommand("groupadd", "--system", group); err != nil {
			return err
		}
	}
	return nil
}

func writeSystemdUnit(c ServiceConfig) error {
	extra := strings.Join(c.ServiceLines, "\n") + "\n"

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
`, c.UniqueName, c.BinaryPath, c.User, c.Group, extra)

	return os.WriteFile(c.SystemdFile, []byte(unit), 0644)
}

func writeRsyslogConf(c ServiceConfig) error {
	configs := []string{}
	for name, file := range c.Streams {
		configs = append(configs, fmt.Sprintf(`if $msg contains 'stream=%s' then {
			action(type="omfile" file="%s/%s" template="%s")
			stop
		}
		`, name, c.LogDir, file, c.UniqueName))
	}
	if len(configs) == 0 {
		return nil // no streams to configure
	}
	conf := fmt.Sprintf(`module(load="imuxsock")
module(load="imklog")
module(load="omfile")
template(name="%s" type="string"
		  string="%%msg%%\n")
%s`, c.UniqueName, strings.Join(configs, "\n"))

	return os.WriteFile(rsyslogPath(c), []byte(conf), 0640)
}

func writeLogrotateConfs(c ServiceConfig) error {
	if !c.MakeLogrotate {
		return nil
	}

	if c.Streams == nil {
		return nil
	}

	for name, file := range c.Streams {
		log := fmt.Sprintf(`%s/%s {
	weekly
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
	}
	`, c.LogDir, file, c.User, c.Group)
		if err := os.WriteFile(
			logrotateCorePath(c)+"-"+name, []byte(log), 0640); err != nil {
			return err
		}
	}

	return nil
}

func rsyslogPath(c ServiceConfig) string { return fmt.Sprintf("/etc/rsyslog.d/%s.conf", c.UniqueName) }
func logrotateCorePath(c ServiceConfig) string {
	return fmt.Sprintf("/etc/logrotate.d/%s", c.UniqueName)
}

func execOutput(cmd string, args ...string) ([]byte, error) {
	return exec.Command(cmd, args...).CombinedOutput()
}

func execCommand(cmd string, args ...string) error {
	out, err := execOutput(cmd, args...)
	if err != nil {
		return fmt.Errorf("%s failed: %v\noutput: %s", cmd, err, string(out))
	}
	return nil
}
