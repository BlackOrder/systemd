# Integration Guide

This guide shows how to integrate the systemd package into your Go applications for service management.

## Table of Contents
- [Basic Integration](#basic-integration)
- [Advanced Patterns](#advanced-patterns)
- [Real-World Examples](#real-world-examples)
- [Best Practices](#best-practices)
- [Troubleshooting](#troubleshooting)

## Basic Integration

### 1. Simple Command-Line Integration

The most common pattern is to add service management flags to your application:

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
    // Service management flags
    install := flag.Bool("install", false, "Install systemd service")
    uninstall := flag.Bool("uninstall", false, "Uninstall systemd service")
    flag.Parse()

    // Handle service management
    if *install || *uninstall {
        handleServiceManagement(*install, *uninstall)
        return
    }

    // Your normal application logic
    runApplication()
}

func handleServiceManagement(install, uninstall bool) {
    // Get the current executable path
    bin, err := os.Executable()
    if err != nil {
        log.Fatalf("Failed to get executable path: %v", err)
    }
    bin, _ = filepath.Abs(bin)

    // Create service configuration
    cfg := smd.NewServiceConfig(
        "myapp",                // service user
        "myapp",                // service group
        bin,                    // binary path
        "/var/log/myapp",       // log directory
        smd.WithWatchdog("30s"),
        smd.WithJournal(),
        smd.WithLogrotate(),
    )

    mgr := smd.NewManager(&cfg)

    if install {
        log.Println("Installing systemd service...")
        if err := mgr.Install(); err != nil {
            log.Fatalf("Failed to install service: %v", err)
        }
        log.Printf("Service %s installed successfully", cfg.ServiceName)
    } else if uninstall {
        log.Println("Uninstalling systemd service...")
        if err := mgr.Uninstall(); err != nil {
            log.Fatalf("Failed to uninstall service: %v", err)
        }
        log.Printf("Service %s uninstalled successfully", cfg.ServiceName)
    }
}

func runApplication() {
    log.Println("Starting application...")
    // Your application logic here
}
```

## Advanced Patterns

### 1. Singleton Configuration with External Config

For applications with complex configuration systems:

```go
package service

import (
    "os"
    "path/filepath"
    "sync"
    
    smd "github.com/blackorder/systemd"
    "your-app/config" // Your app's config package
)

var (
    cfg  smd.ServiceConfig
    once sync.Once
)

// GetServiceConfig returns the singleton ServiceConfig instance
func GetServiceConfig() *smd.ServiceConfig {
    once.Do(func() {
        cfg = buildServiceConfig()
    })
    return &cfg
}

func buildServiceConfig() smd.ServiceConfig {
    // Get current executable path
    bin, _ := os.Executable()
    bin, _ = filepath.Abs(bin)

    // Load your application configuration
    appCfg := config.Load()
    
    // Build service options based on your config
    opts := []smd.ServiceOpt{
        smd.WithWatchdog("30s"),
        smd.WithJournal(),
        smd.WithNotifyAccess(),
    }

    // Add production-specific options
    if appCfg.Environment == "production" {
        opts = append(opts,
            smd.WithUMask("0027"),
            smd.WithLimitNOFILE("65535"),
            smd.WithLogrotate(),
        )
    }

    // Add log streams based on your app's logging setup
    if appCfg.Logging.FileLogging {
        streams := make(map[string]string)
        for _, stream := range appCfg.Logging.Streams {
            streams[stream.Name] = stream.File
        }
        opts = append(opts, smd.WithStreams(streams))
    }

    // Add environment variables
    for key, value := range appCfg.Service.Environment {
        opts = append(opts, smd.WithServiceLine(fmt.Sprintf("Environment=%s=%s", key, value)))
    }

    return smd.NewServiceConfig(
        appCfg.Service.User,
        appCfg.Service.Group,
        bin,
        appCfg.Logging.Directory,
        opts...,
    )
}
```

### 2. Integration with Structured Logging

Using zerolog for structured logging:

```go
package main

import (
    "flag"
    "os"
    
    "github.com/rs/zerolog"
    "github.com/rs/zerolog/log"
    smd "github.com/blackorder/systemd"
)

func main() {
    // Configure logging
    log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
    
    install := flag.Bool("install", false, "Install systemd service")
    uninstall := flag.Bool("uninstall", false, "Uninstall systemd service")
    flag.Parse()

    if *install || *uninstall {
        handleServiceWithLogging(*install, *uninstall)
        return
    }

    runApplication()
}

func handleServiceWithLogging(install, uninstall bool) {
    cfg := service.GetServiceConfig()

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
            log.Error().Err(err).Msg("Service management error")
        }
    }()

    mgr := smd.NewManager(*cfg, 
        smd.WithInfoChan(infoCh),
        smd.WithErrorChan(errCh),
    )

    if install {
        log.Info().Str("service", cfg.ServiceName).Msg("Installing systemd service")
        if err := mgr.Install(); err != nil {
            log.Fatal().Err(err).Msg("Failed to install service")
        }
        log.Info().Str("service", cfg.ServiceName).Msg("Service installed successfully")
    } else if uninstall {
        log.Info().Str("service", cfg.ServiceName).Msg("Uninstalling systemd service")
        if err := mgr.Uninstall(); err != nil {
            log.Fatal().Err(err).Msg("Failed to uninstall service")
        }
        log.Info().Str("service", cfg.ServiceName).Msg("Service uninstalled successfully")
    }

    // Close channels
    close(infoCh)
    close(errCh)
}
```

### 3. Integration with Cobra CLI

For applications using the Cobra CLI framework:

```go
package cmd

import (
    "github.com/spf13/cobra"
    "github.com/rs/zerolog/log"
    smd "github.com/blackorder/systemd"
)

var serviceCmd = &cobra.Command{
    Use:   "service",
    Short: "Manage systemd service",
}

var installCmd = &cobra.Command{
    Use:   "install",
    Short: "Install systemd service",
    Long:  "Install the application as a systemd service",
    Run: func(cmd *cobra.Command, args []string) {
        cfg := service.GetServiceConfig()
        mgr := smd.NewManager(*cfg)
        
        if err := mgr.Install(); err != nil {
            log.Fatal().Err(err).Msg("Failed to install service")
        }
        
        log.Info().Str("service", cfg.ServiceName).Msg("Service installed successfully")
        cmd.Printf("Service %s installed successfully\n", cfg.ServiceName)
        cmd.Printf("Start with: systemctl start %s\n", cfg.ServiceName)
        cmd.Printf("Check status: systemctl status %s\n", cfg.ServiceName)
    },
}

var uninstallCmd = &cobra.Command{
    Use:   "uninstall",
    Short: "Uninstall systemd service",
    Long:  "Remove the systemd service and clean up configuration files",
    Run: func(cmd *cobra.Command, args []string) {
        cfg := service.GetServiceConfig()
        mgr := smd.NewManager(*cfg)
        
        if err := mgr.Uninstall(); err != nil {
            log.Fatal().Err(err).Msg("Failed to uninstall service")
        }
        
        log.Info().Str("service", cfg.ServiceName).Msg("Service uninstalled successfully")
        cmd.Printf("Service %s uninstalled successfully\n", cfg.ServiceName)
    },
}

var statusCmd = &cobra.Command{
    Use:   "status",
    Short: "Show service status",
    Long:  "Display the current status of the systemd service",
    Run: func(cmd *cobra.Command, args []string) {
        cfg := service.GetServiceConfig()
        cmd.Printf("Service: %s\n", cfg.ServiceName)
        cmd.Printf("User: %s\n", cfg.User)
        cmd.Printf("Binary: %s\n", cfg.BinaryPath)
        cmd.Printf("Logs: %s\n", cfg.LogDir)
        cmd.Printf("\nCheck status with: systemctl status %s\n", cfg.ServiceName)
    },
}

func init() {
    serviceCmd.AddCommand(installCmd)
    serviceCmd.AddCommand(uninstallCmd)
    serviceCmd.AddCommand(statusCmd)
}
```

## Real-World Examples

### Example 1: Web Application with Multiple Log Streams

```go
func buildWebAppConfig() smd.ServiceConfig {
    bin, _ := os.Executable()
    bin, _ = filepath.Abs(bin)

    return smd.NewServiceConfig(
        "webapp",
        "webapp",
        bin,
        "/var/log/webapp",
        // Service configuration
        smd.WithWatchdog("30s"),
        smd.WithJournal(),
        smd.WithUMask("0027"),
        smd.WithLimitNOFILE("65535"),
        smd.WithExecReload("5s", "30", "30"),
        smd.WithNotifyAccess(),
        smd.WithLogrotate(),
        
        // Environment variables
        smd.WithServiceLine("Environment=NODE_ENV=production"),
        smd.WithServiceLine("Environment=PORT=8080"),
        smd.WithServiceLine("WorkingDirectory=/opt/webapp"),
        
        // Security settings
        smd.WithServiceLine("PrivateTmp=true"),
        smd.WithServiceLine("NoNewPrivileges=true"),
        smd.WithServiceLine("ProtectKernelTunables=true"),
        smd.WithServiceLine("ProtectControlGroups=true"),
        
        // Log streams
        smd.WithStreams(map[string]string{
            "CORE":        "core.log",
            "HTTP-ACCESS": "http_access.log",
            "HTTP-ERROR":  "http_error.log",
            "AUDIT":       "audit.log",
        }),
    )
}
```

### Example 2: Database Application with Resource Limits

```go
func buildDatabaseConfig() smd.ServiceConfig {
    bin, _ := os.Executable()
    bin, _ = filepath.Abs(bin)

    return smd.NewServiceConfig(
        "mydb",
        "mydb",
        bin,
        "/var/log/mydb",
        // Database-specific configuration
        smd.WithWatchdog("60s"), // Longer watchdog for DB
        smd.WithJournal(),
        smd.WithUMask("0027"),
        smd.WithLimitNOFILE("100000"), // High file descriptor limit
        smd.WithExecReload("10s", "60", "30"), // Longer start timeout
        smd.WithNotifyAccess(),
        smd.WithLogrotate(),
        
        // Resource limits
        smd.WithServiceLine("LimitMEMLOCK=infinity"),
        smd.WithServiceLine("LimitNOFILE=100000"),
        smd.WithServiceLine("LimitCORE=infinity"),
        
        // Environment
        smd.WithServiceLine("Environment=DB_CONFIG=/etc/mydb/config.toml"),
        smd.WithServiceLine("WorkingDirectory=/var/lib/mydb"),
        
        // Security for database
        smd.WithServiceLine("PrivateTmp=true"),
        smd.WithServiceLine("ProtectHome=true"),
        smd.WithServiceLine("ProtectSystem=strict"),
        smd.WithServiceLine("ReadWritePaths=/var/lib/mydb"),
        
        // Log streams
        smd.WithStreams(map[string]string{
            "DATABASE": "database.log",
            "QUERY":    "query.log",
            "ERROR":    "error.log",
            "AUDIT":    "audit.log",
        }),
    )
}
```

## Best Practices

### 1. Configuration Management

- **Use singleton pattern** for service configuration to ensure consistency
- **Determine binary path dynamically** using `os.Executable()`
- **Integrate with your existing config system** rather than hardcoding values
- **Use environment-specific configurations** (dev vs prod)

### 2. Security

```go
// Production security settings
smd.WithUMask("0027"),                    // Restrictive umask
smd.WithServiceLine("PrivateTmp=true"),   // Private /tmp
smd.WithServiceLine("NoNewPrivileges=true"), // Prevent privilege escalation
smd.WithServiceLine("ProtectKernelTunables=true"), // Protect kernel parameters
smd.WithServiceLine("ProtectControlGroups=true"),  // Protect cgroups
smd.WithServiceLine("ProtectSystem=strict"),       // Read-only system
```

### 3. Resource Management

```go
// Appropriate resource limits
smd.WithLimitNOFILE("65535"),            // File descriptor limit
smd.WithExecReload("5s", "30", "30"),    // Timeouts: restart, start, stop
smd.WithWatchdog("30s"),                 // Health check interval
```

### 4. Logging

```go
// Structured logging setup
smd.WithJournal(),                       // Use systemd journal
smd.WithLogrotate(),                     // Enable log rotation
smd.WithStreams(map[string]string{       // Separate log streams
    "APP":   "application.log",
    "ERROR": "error.log",
    "AUDIT": "audit.log",
}),
```

### 5. Error Handling

```go
// Monitor installation progress
infoCh := make(chan string, 10)
errCh := make(chan error, 10)

go func() {
    for msg := range infoCh {
        log.Info().Msg(msg)
    }
}()

go func() {
    for err := range errCh {
        log.Error().Err(err).Msg("Installation error")
    }
}()

mgr := smd.NewManager(&cfg, smd.WithInfoChan(infoCh), smd.WithErrorChan(errCh))
```

## Troubleshooting

### Common Issues

1. **Permission Denied**
   ```bash
   sudo ./myapp --install  # Service installation requires root
   ```

2. **Service User Doesn't Exist**
   - The package automatically creates system users and groups
   - Ensure the user/group names are valid system identifiers

3. **Binary Path Issues**
   - Always use `os.Executable()` to get the current binary path
   - Ensure the binary is in a stable location for production

4. **Log Directory Permissions**
   - The package sets up proper permissions automatically
   - Ensure the parent directory exists and is writable

### Validation

```bash
# Check service status
systemctl status myapp

# View service logs
journalctl -u myapp -f

# Check service configuration
systemctl cat myapp

# Verify log files
ls -la /var/log/myapp/
```

### Development Testing

```bash
# Test without actual installation
./myapp --help

# Dry run with config validation
go run main.go --install --dry-run  # if you implement dry-run flag
```

This integration guide should help you implement the systemd package effectively in your applications. The patterns shown here are based on real-world usage and provide a solid foundation for service management in Go applications.
