package systemd

import (
	"fmt"
	"maps"
	"path/filepath"
	"regexp"
)

// A ServiceOpt mutates the ServiceConfig in-place.
type ServiceOpt func(*ServiceConfig)

// WithWatchdog sets WatchdogSec.
func WithWatchdog(sec string) ServiceOpt {
	return func(c *ServiceConfig) {
		c.ServiceLines = append(c.ServiceLines,
			fmt.Sprintf("WatchdogSec=%s", sec))
	}
}

// WithServiceLine appends an arbitrary line to the [Service] block.
func WithServiceLine(line string) ServiceOpt {
	return func(c *ServiceConfig) { c.ServiceLines = append(c.ServiceLines, line) }
}

// WithJournal routes stdio to journald (StandardOutput/StandardError).
func WithJournal() ServiceOpt {
	return func(c *ServiceConfig) {
		c.ServiceLines = append(c.ServiceLines,
			"StandardOutput=journal",
			"StandardError=journal")
	}
}

func WithUMask(umask string) ServiceOpt {
	return func(c *ServiceConfig) {
		c.ServiceLines = append(c.ServiceLines, fmt.Sprintf("UMask=%s", umask))
	}
}

func WithLimitNOFILE(limit string) ServiceOpt {
	return func(c *ServiceConfig) {
		c.ServiceLines = append(c.ServiceLines, fmt.Sprintf("LimitNOFILE=%s", limit))
	}
}

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

func WithNotifyAccess() ServiceOpt {
	return func(c *ServiceConfig) {
		c.ServiceLines = append(c.ServiceLines, "NotifyAccess=main")
	}
}

// WithLogrotate enables logrotate for the core log.
func WithLogrotate() ServiceOpt {
	return func(c *ServiceConfig) {
		c.MakeLogrotate = true
	}
}

func WithStream(name, file string) ServiceOpt {
	return func(c *ServiceConfig) {
		if c.Streams == nil {
			c.Streams = make(map[string]string)
		}
		c.Streams[name] = file
	}
}

func WithStreams(streams map[string]string) ServiceOpt {
	return func(c *ServiceConfig) {
		if c.Streams == nil {
			c.Streams = make(map[string]string)
		}
		maps.Copy(c.Streams, streams)
	}
}

// NewServiceConfig builds the core struct, applies opts, and
// auto‑fills derived fields (UniqueName, ServiceName, SystemdFile).
func NewServiceConfig(
	user, group, bin, logDir string,
	opts ...ServiceOpt,
) ServiceConfig {
	base := sanitize(filepath.Base(bin))
	dir := sanitize(filepath.Base(filepath.Dir(bin)))
	unique := fmt.Sprintf("%s-%s", dir, base)

	c := ServiceConfig{
		User:        user,
		Group:       group,
		BinaryPath:  bin,
		LogDir:      logDir,
		UniqueName:  unique,
		ServiceName: unique + ".service",
	}
	for _, o := range opts {
		o(&c)
	}
	if c.SystemdFile == "" {
		c.SystemdFile = "/etc/systemd/system/" + c.ServiceName
	}
	return c
}

func sanitize(s string) string {
	// Allow only letters, numbers, dashes, and underscores.
	return regexp.MustCompile(`[^a-zA-Z0-9_-]`).ReplaceAllString(s, "")
}
