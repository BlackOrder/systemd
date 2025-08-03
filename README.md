# systemd - Go Package for SystemD Service Management

[![Go Version](https://img.shields.io/badge/go-%3E%3D1.24-blue.svg)](https://golang.org/dl/)
[![Go Report Card](https://goreportcard.com/badge/github.com/blackorder/systemd)](https://goreportcard.com/report/github.com/blackorder/systemd)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

A Go package for programmatically installing and managing systemd services on Linux systems. This package provides a clean, type-safe API for creating systemd unit files, configuring logging with rsyslog, and setting up log rotation.

## Features

- **Service Installation & Management**: Install, enable, start, stop, and uninstall systemd services
- **Configuration Generation**: Generate systemd unit files with customizable service options
- **Logging Integration**: Automatic rsyslog configuration for structured logging
- **Log Rotation**: Optional logrotate configuration for log management
- **Type Safety**: Strongly typed configuration with builder pattern
- **Concurrent Safe**: Designed for safe concurrent usage
- **Flexible Options**: Extensible service configuration through functional options

## Installation

```bash
go get github.com/blackorder/systemd
```

## Real-World Usage Patterns

### Pattern 1: Singleton Service Configuration

For applications that need a consistent service configuration throughout their lifecycle:

```go
package systemd

import (
	"os"
	"path/filepath"
	"sync"

	smd "github.com/blackorder/systemd"
)

var (
	cfg  smd.ServiceConfig
	once sync.Once
)

// GetServiceConfig returns the singleton ServiceConfig instance,
// initializing it only on the first call.
func GetServiceConfig() *smd.ServiceConfig {
	once.Do(func() {
		cfg = makeServiceConfig()
	})
	return &cfg
}

func makeServiceConfig() smd.ServiceConfig {
	// Determine the absolute path of the binary
	bin, _ := os.Executable()
	bin, _ = filepath.Abs(bin)

	// Your application config (replace with your own config system)
	svc := config.GetService() // your own app-level config
	logDir := config.GetLog().FilePath

	return smd.NewServiceConfig(
		svc.User,
		svc.Group,
		bin,
		logDir,
		smd.WithWatchdog("30s"),
		smd.WithJournal(),
		smd.WithUMask("0027"),
		smd.WithLimitNOFILE("65535"),
		smd.WithExecReload("5s", "30", "30"),
		smd.WithNotifyAccess(),
		smd.WithLogrotate(),
		smd.WithStreams(map[string]string{
			"CORE":        "core.log",
			"HTTP-ACCESS": "http_access.log",
			"HTTP-ERROR":  "http_error.log",
		}),
	)
}
```

### Pattern 2: Command-Line Service Management

Integrate service management into your application's CLI:

```go
package main

import (
	"flag"
	"log"

	smd "github.com/blackorder/systemd"
)

func main() {
	cfg := systemd.GetServiceConfig()

	installService := flag.Bool("install", false, "Install or update systemd service for "+cfg.ServiceName)
	uninstallService := flag.Bool("uninstall", false, "Uninstall systemd service for "+cfg.ServiceName)
	flag.Parse()

	if *installService {
		infoCh := make(chan string, 10)
		go func() {
			for msg := range infoCh {
				log.Printf("INFO: %s", msg)
			}
		}()

		mgr := smd.NewManager(*cfg, smd.WithInfoChan(infoCh))

		if err := mgr.Install(); err != nil {
			log.Fatalf("Failed to install systemd service: %v", err)
		}
		log.Printf("Service %s installed successfully", cfg.ServiceName)
		return
	}

	if *uninstallService {
		infoCh := make(chan string, 10)
		go func() {
			for msg := range infoCh {
				log.Printf("INFO: %s", msg)
			}
		}()

		mgr := smd.NewManager(*cfg, smd.WithInfoChan(infoCh))
		if err := mgr.Uninstall(); err != nil {
			log.Fatalf("Failed to uninstall systemd service: %v", err)
		}
		log.Printf("Service %s uninstalled successfully", cfg.ServiceName)
		return
	}

	// Your main application logic here
	runApplication()
}

func runApplication() {
	// Your application's main logic
	log.Println("Application running...")
}
```

### Pattern 3: Integration with Structured Logging

For applications using structured logging (like zerolog, logrus, etc.):

```go
package main

import (
	"flag"
	
	"github.com/rs/zerolog/log"
	smd "github.com/blackorder/systemd"
)

func main() {
	cfg := systemd.GetServiceConfig()

	installService := flag.Bool("install", false, "Install systemd service")
	uninstallService := flag.Bool("uninstall", false, "Uninstall systemd service")
	flag.Parse()

	if *installService {
		// Create channels for monitoring
		infoCh := make(chan string, 10)
		errCh := make(chan error, 10)

		// Monitor info messages
		go func() {
			for msg := range infoCh {
				log.Info().Msg(msg)
			}
		}()

		// Monitor error messages
		go func() {
			for err := range errCh {
				log.Error().Err(err).Msg("Installation error")
			}
		}()

		mgr := smd.NewManager(*cfg, 
			smd.WithInfoChan(infoCh),
			smd.WithErrorChan(errCh),
		)

		if err := mgr.Install(); err != nil {
			log.Fatal().Err(err).Msg("Failed to install systemd service")
		}
		
		// Close channels
		close(infoCh)
		close(errCh)
		
		log.Info().Str("service", cfg.ServiceName).Msg("Service installed successfully")
		return
	}

	if *uninstallService {
		mgr := smd.NewManager(*cfg)
		if err := mgr.Uninstall(); err != nil {
			log.Fatal().Err(err).Msg("Failed to uninstall systemd service")
		}
		log.Info().Str("service", cfg.ServiceName).Msg("Service uninstalled successfully")
		return
	}

	// Start your application
	log.Info().Msg("Starting application...")
	runApplication()
}
```
```

## Best Practices

### 1. Service Configuration Management

**Use Singleton Pattern**: For applications that need consistent service configuration, use the singleton pattern to ensure the same configuration is used throughout your application lifecycle.

**Dynamic Binary Path**: Always determine the binary path dynamically using `os.Executable()` to ensure the service points to the correct executable regardless of where it's installed.

```go
bin, _ := os.Executable()
bin, _ = filepath.Abs(bin)
```

**Configuration Integration**: Integrate with your existing configuration system rather than hardcoding values.

### 2. Error Handling

**Monitor Installation Progress**: Use the info and error channels to provide feedback during installation:

```go
infoCh := make(chan string, 10)
errCh := make(chan error, 10)
mgr := smd.NewManager(&cfg, smd.WithInfoChan(infoCh), smd.WithErrorChan(errCh))
```

**Graceful Degradation**: Handle cases where service installation fails without crashing your application.

### 3. Security Considerations

**Appropriate Umask**: Set restrictive umask for better security:
```go
smd.WithUMask("0027") // Owner: rwx, Group: r-x, Others: none
```

**Resource Limits**: Set appropriate resource limits:
```go
smd.WithLimitNOFILE("65535") // Reasonable file descriptor limit
```

**User Isolation**: Always run services with dedicated users, never as root.

### 4. Logging Best Practices

**Structured Logging**: Use structured log streams for different types of logs:
```go
smd.WithStreams(map[string]string{
    "CORE":        "core.log",        // Application core logs
    "HTTP-ACCESS": "http_access.log", // HTTP access logs
    "HTTP-ERROR":  "http_error.log",  // HTTP error logs
    "AUDIT":       "audit.log",       // Security/audit logs
})
```

**Log Rotation**: Always enable log rotation for production services:
```go
smd.WithLogrotate()
```

### 5. Service Lifecycle

**Installation Flags**: Provide command-line flags for service management:
```bash
./myapp --install    # Install the service
./myapp --uninstall  # Uninstall the service
./myapp              # Run normally
```

**Validation**: Validate that the service can be installed before attempting:
```go
if os.Geteuid() != 0 {
    log.Fatal("Service installation requires root privileges")
}
```

### 6. Integration Patterns

#### With Cobra CLI

```go
var installCmd = &cobra.Command{
    Use:   "install",
    Short: "Install systemd service",
    Run: func(cmd *cobra.Command, args []string) {
        cfg := systemd.GetServiceConfig()
        mgr := smd.NewManager(*cfg)
        if err := mgr.Install(); err != nil {
            log.Fatal(err)
        }
        fmt.Println("Service installed successfully")
    },
}
```

#### With Environment Variables

```go
func makeServiceConfig() smd.ServiceConfig {
    user := getEnvOr("SERVICE_USER", "myapp")
    group := getEnvOr("SERVICE_GROUP", "myapp")
    logDir := getEnvOr("LOG_DIR", "/var/log/myapp")
    
    return smd.NewServiceConfig(user, group, bin, logDir, /* options */)
}
```

### 7. Development vs Production

**Environment Detection**: Use different configurations for development and production:

```go
func makeServiceConfig() smd.ServiceConfig {
    opts := []smd.ServiceOpt{
        smd.WithWatchdog("30s"),
        smd.WithJournal(),
    }
    
    if isProduction() {
        opts = append(opts,
            smd.WithUMask("0027"),
            smd.WithLimitNOFILE("65535"),
            smd.WithLogrotate(),
        )
    }
    
    return smd.NewServiceConfig(user, group, bin, logDir, opts...)
}
```

## Quick Start

For a simple integration, here's the minimal setup:

```go
package main

import (
    "flag"
    "log"
    "os"
    "path/filepath"
    
    smd "github.com/blackorder/systemd"
)

func main() {
    install := flag.Bool("install", false, "Install systemd service")
    uninstall := flag.Bool("uninstall", false, "Uninstall systemd service")
    flag.Parse()

    if *install || *uninstall {
        bin, _ := os.Executable()
        bin, _ = filepath.Abs(bin)
        
        cfg := smd.NewServiceConfig(
            "myapp", "myapp", bin, "/var/log/myapp",
            smd.WithWatchdog("30s"),
            smd.WithJournal(),
        )
        
        mgr := smd.NewManager(&cfg)
        
        if *install {
            if err := mgr.Install(); err != nil {
                log.Fatal(err)
            }
            log.Println("Service installed")
        } else {
            if err := mgr.Uninstall(); err != nil {
                log.Fatal(err)
            }
            log.Println("Service uninstalled")
        }
        return
    }

    // Your application logic here
    log.Println("Application running...")
}

### Core Types

#### ServiceConfig

The main configuration struct for defining a systemd service:

```go
type ServiceConfig struct {
    // Mandatory fields
    User        string // system user that owns the process
    Group       string // primary group
    UniqueName  string // slug to identify files (no spaces)
    ServiceName string // e.g. my-app.service
    BinaryPath  string // absolute path to executable

    // Optional fields
    LogDir      string // if empty, rsyslog/logrotate files are skipped
    SystemdFile string // defaults to /etc/systemd/system/<ServiceName>

    // Customization
    ServiceLines  []string          // raw lines appended to [Service]
    MakeLogrotate bool              // generate logrotate for core log
    Streams       map[string]string // map of stream names to log file names
}
```

#### Manager

The main interface for service management:

```go
type Manager struct {
    // private fields
}

func NewManager(cfg *ServiceConfig, opts ...Option) *Manager
func (m *Manager) Install() error
func (m *Manager) Uninstall() error
```

### Configuration Functions

#### NewServiceConfig

Creates a new service configuration with sensible defaults:

```go
func NewServiceConfig(user, group, bin, logDir string, opts ...ServiceOpt) ServiceConfig
```

**Parameters:**
- `user`: System user to run the service as
- `group`: Primary group for the service
- `bin`: Absolute path to the executable
- `logDir`: Directory for log files (can be empty to skip logging setup)
- `opts`: Functional options for additional configuration

**Example:**
```go
cfg := systemd.NewServiceConfig(
    "webapp", 
    "webapp", 
    "/opt/webapp/bin/webapp",
    "/var/log/webapp",
    systemd.WithWatchdog("30s"),
)
```

### Service Options (ServiceOpt)

#### WithWatchdog
Enables systemd watchdog with the specified timeout:
```go
systemd.WithWatchdog("30s") // Sets WatchdogSec=30s
```

#### WithJournal
Routes stdout/stderr to systemd journal:
```go
systemd.WithJournal() // Sets StandardOutput=journal, StandardError=journal
```

#### WithServiceLine
Adds custom lines to the [Service] section:
```go
systemd.WithServiceLine("Environment=DEBUG=1")
```

#### WithUMask
Sets the umask for the service:
```go
systemd.WithUMask("0022")
```

#### WithLimitNOFILE
Sets file descriptor limits:
```go
systemd.WithLimitNOFILE("65536")
```

#### WithExecReload
Configures reload behavior:
```go
systemd.WithExecReload("5", "30", "30") // restart, start timeout, stop timeout
```

#### WithNotifyAccess
Enables systemd notify protocol:
```go
systemd.WithNotifyAccess() // Sets NotifyAccess=main
```

#### WithLogrotate
Enables logrotate configuration generation:
```go
systemd.WithLogrotate()
```

#### WithStream
Adds a single log stream:
```go
systemd.WithStream("app", "application.log")
```

#### WithStreams
Adds multiple log streams:
```go
streams := map[string]string{
    "app":   "app.log",
    "error": "error.log",
    "audit": "audit.log",
}
systemd.WithStreams(streams)
```

### Manager Options

#### WithErrorChan
Sets a channel to receive error messages:
```go
errChan := make(chan error, 10)
manager := systemd.NewManager(&cfg, systemd.WithErrorChan(errChan))
```

#### WithInfoChan
Sets a channel to receive informational messages:
```go
infoChan := make(chan string, 10)
manager := systemd.NewManager(&cfg, systemd.WithInfoChan(infoChan))
```

## Examples

### Basic Service Installation

```go
package main

import (
    "github.com/blackorder/systemd"
    "log"
)

func main() {
    cfg := systemd.NewServiceConfig(
        "myservice",
        "myservice", 
        "/usr/local/bin/myservice",
        "", // no logging
    )
    
    manager := systemd.NewManager(&cfg)
    if err := manager.Install(); err != nil {
        log.Fatal(err)
    }
    
    log.Println("Service installed successfully")
}
```

### Advanced Service with Logging

```go
package main

import (
    "github.com/blackorder/systemd"
    "log"
)

func main() {
    cfg := systemd.NewServiceConfig(
        "webapp",
        "webapp",
        "/opt/webapp/bin/webapp",
        "/var/log/webapp",
        systemd.WithWatchdog("30s"),
        systemd.WithJournal(),
        systemd.WithLogrotate(),
        systemd.WithUMask("0022"),
        systemd.WithLimitNOFILE("65536"),
        systemd.WithStream("access", "access.log"),
        systemd.WithStream("error", "error.log"),
        systemd.WithServiceLine("Environment=NODE_ENV=production"),
        systemd.WithServiceLine("Environment=PORT=8080"),
    )
    
    errChan := make(chan error, 10)
    infoChan := make(chan string, 10)
    
    manager := systemd.NewManager(&cfg,
        systemd.WithErrorChan(errChan),
        systemd.WithInfoChan(infoChan),
    )
    
    // Monitor progress
    go func() {
        for msg := range infoChan {
            log.Printf("INFO: %s", msg)
        }
    }()
    
    go func() {
        for err := range errChan {
            log.Printf("ERROR: %v", err)
        }
    }()
    
    if err := manager.Install(); err != nil {
        log.Fatal(err)
    }
    
    log.Println("Service installed successfully")
}
```

### Service Uninstallation

```go
cfg := systemd.NewServiceConfig(
    "myservice",
    "myservice",
    "/usr/local/bin/myservice",
    "/var/log/myservice",
)

manager := systemd.NewManager(&cfg)
if err := manager.Uninstall(); err != nil {
    log.Printf("Failed to uninstall: %v", err)
} else {
    log.Println("Service uninstalled successfully")
}
```

## Generated Files

When installing a service, this package generates the following files:

### SystemD Unit File
Location: `/etc/systemd/system/<service-name>.service`

Example:
```ini
[Unit]
Description=webapp
After=network.target

[Service]
Type=notify
ExecStart=/opt/webapp/bin/webapp
Restart=on-failure
User=webapp
Group=webapp
WatchdogSec=30s
StandardOutput=journal
StandardError=journal
Environment=NODE_ENV=production

[Install]
WantedBy=multi-user.target
```

### Rsyslog Configuration
Location: `/etc/rsyslog.d/<unique-name>.conf`

Example:
```
module(load="imuxsock")
module(load="imklog")
module(load="omfile")
template(name="webapp" type="string" string="%msg%\n")

if $msg contains 'stream=access' then {
    action(type="omfile" file="/var/log/webapp/access.log" template="webapp"
         dirCreateMode="0750" dirOwner="webapp" dirGroup="webapp"
         fileCreateMode="0640" fileOwner="webapp" fileGroup="webapp")
    stop
}

if $msg contains 'stream=error' then {
    action(type="omfile" file="/var/log/webapp/error.log" template="webapp"
         dirCreateMode="0750" dirOwner="webapp" dirGroup="webapp"
         fileCreateMode="0640" fileOwner="webapp" fileGroup="webapp")
    stop
}
```

### Logrotate Configuration
Location: `/etc/logrotate.d/<unique-name>-<stream>`

Example:
```
/var/log/webapp/access.log {
    weekly
    rotate 8
    size 100M
    compress
    delaycompress
    missingok
    notifempty
    create 0640 webapp webapp
    sharedscripts
    postrotate
        systemctl kill -s HUP rsyslog.service
    endscript
}
```

## Requirements

- Linux system with systemd
- Go 1.24.5 or later
- Root privileges for service installation (typically)
- `systemctl` command available
- `useradd`, `groupadd` commands for user management
- Optional: `rsyslog` for logging features

## Error Handling

The package provides detailed error information for common failure scenarios:

- **Permission errors**: When running without sufficient privileges
- **User/group creation failures**: When system user management fails
- **File write errors**: When configuration files cannot be written
- **Service management errors**: When systemctl commands fail

All errors include context about what operation failed and the underlying system error.

## Thread Safety

This package is designed to be thread-safe for its intended usage patterns:

- **NewManager()**: Safe for concurrent calls
- **NewServiceConfig()**: Safe for concurrent calls  
- **Manager methods**: Safe to call on different Manager instances concurrently
- **Option functions**: Safe when applied during config creation
- **Channel operations**: Non-blocking and safe for concurrent use

**Note**: The same ServiceConfig instance should not be modified concurrently by multiple goroutines after creation.

## Testing

Run the test suite:

```bash
# Run all tests
go test ./...

# Run tests with race detection
go test -race ./...

# Run tests with verbose output
go test -v ./...

# Run benchmarks
go test -bench=. ./...
```

The package includes comprehensive tests covering:
- Unit tests for all public functions
- Integration tests for service installation/uninstallation
- Race condition detection tests
- Benchmark tests for performance analysis

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests for new functionality
5. Ensure all tests pass with race detection
6. Submit a pull request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Changelog

### v1.0.0
- Initial release
- Core service management functionality
- Rsyslog and logrotate integration
- Comprehensive test suite
- Thread-safe design

## Support

For issues, questions, or contributions, please use the GitHub issue tracker at:
https://github.com/blackorder/systemd/issues
