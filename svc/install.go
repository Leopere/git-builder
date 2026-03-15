package svc

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

const (
	installBin   = "/usr/local/bin/git-builder"
	installUnit  = "/etc/systemd/system/git-builder.service"
	launchdPlist = "/Library/LaunchDaemons/com.git-builder.plist"
)

const (
	unitContent = `[Unit]
Description=Git-builder SCM polling service
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/git-builder
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
`
)

// hasSystemd reports whether systemd is the running init (e.g. typical Linux).
// Prefer checking the directory to avoid spawning systemctl.
func hasSystemd() bool {
	info, err := os.Stat("/run/systemd/system")
	if err != nil {
		return false
	}
	return info.IsDir()
}

func launchdPlistContent(bin, configPath, runDir, keyDir, logPath string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>com.git-builder</string>
  <key>ProgramArguments</key>
  <array>
    <string>%s</string>
  </array>
  <key>EnvironmentVariables</key>
  <dict>
    <key>GIT_BUILDER_CONFIG</key>
    <string>%s</string>
    <key>GIT_BUILDER_RUNDIR</key>
    <string>%s</string>
    <key>GIT_BUILDER_KEY_DIR</key>
    <string>%s</string>
  </dict>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <true/>
  <key>StandardOutPath</key>
  <string>%s</string>
  <key>StandardErrorPath</key>
  <string>%s</string>
</dict>
</plist>
`, bin, configPath, runDir, keyDir, logPath, logPath)
}

func Install() error {
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("executable: %w", err)
	}
	if runtime.GOOS == "darwin" {
		return installLaunchdUser(self)
	}
	if !hasSystemd() {
		return fmt.Errorf("systemd not detected; install is supported on macOS (launchd) or Linux with systemd")
	}
	if err := copyFile(self, installBin); err != nil {
		return fmt.Errorf("copy binary: %w", err)
	}
	if err := os.WriteFile(installUnit, []byte(unitContent), 0644); err != nil {
		return fmt.Errorf("write unit: %w", err)
	}
	for _, c := range [][]string{
		{"systemctl", "daemon-reload"},
		{"systemctl", "enable", "git-builder"},
		{"systemctl", "start", "git-builder"},
	} {
		if err := run(c[0], c[1:]...); err != nil {
			return fmt.Errorf("%s: %w", c[0], err)
		}
	}
	fmt.Println("installed and started git-builder service")
	return nil
}

func installLaunchdUser(self string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("home dir: %w", err)
	}
	binPath := filepath.Join(home, ".local", "bin", "git-builder")
	configDir := filepath.Join(home, ".config", "git-builder")
	configPath := filepath.Join(configDir, "config.yaml")
	workdir := filepath.Join(home, ".local", "share", "git-builder", "repos")
	logDir := filepath.Join(home, ".local", "var", "log")
	logPath := filepath.Join(logDir, "git-builder.log")
	runDir := filepath.Join(home, ".local", "var", "run", "git-builder")
	plistPath := filepath.Join(home, "Library", "LaunchAgents", "com.git-builder.plist")

	for _, d := range []string{filepath.Dir(binPath), configDir, workdir, logDir, runDir} {
		if err := os.MkdirAll(d, 0755); err != nil {
			return fmt.Errorf("mkdir %s: %w", d, err)
		}
	}
	if err := copyFile(self, binPath); err != nil {
		return fmt.Errorf("copy binary: %w", err)
	}
	if err := ensureUserConfig(configPath, filepath.Dir(self), workdir); err != nil {
		return fmt.Errorf("config: %w", err)
	}
	keyDir := filepath.Join(home, ".ssh")
	plist := launchdPlistContent(binPath, configPath, runDir, keyDir, logPath)
	if err := os.WriteFile(plistPath, []byte(plist), 0644); err != nil {
		return fmt.Errorf("write plist: %w", err)
	}
	_ = run("launchctl", "unload", "-w", plistPath)
	if err := run("launchctl", "load", "-w", plistPath); err != nil {
		return fmt.Errorf("launchctl load: %w", err)
	}
	fmt.Println("installed and started git-builder service (launchd user)")
	fmt.Printf("  config: %s\n", configPath)
	fmt.Printf("  log: %s\n", logPath)
	fmt.Println("  (edit config, then service will hot-reload on save)")
	return nil
}

func ensureUserConfig(dst, exeDir, workdir string) error {
	if _, err := os.Stat(dst); err == nil {
		return nil // already exists, don't overwrite
	}
	example := filepath.Join(exeDir, "config.example.yaml")
	b, err := os.ReadFile(example)
	if err != nil {
		if os.IsNotExist(err) {
			return writeDefaultConfig(dst, workdir)
		}
		return err
	}
	b = replaceWorkdirInYAML(b, workdir)
	return os.WriteFile(dst, b, 0644)
}

func replaceWorkdirInYAML(b []byte, workdir string) []byte {
	const prefix = "workdir:"
	for i := 0; i < len(b); i++ {
		if (i == 0 || b[i-1] == '\n') && i+len(prefix) <= len(b) && string(b[i:i+len(prefix)]) == prefix {
			end := i + len(prefix)
			for end < len(b) && b[end] != '\n' {
				end++
			}
			newLine := fmt.Sprintf(" %s\n", workdir)
			return append(append(append(b[:i+len(prefix)], newLine...), b[end:]...))
		}
	}
	return b
}

func writeDefaultConfig(path, workdir string) error {
	content := fmt.Sprintf(`# git-builder user config (generated)
poll_interval_seconds: 300
workdir: %s
ssh_key: id_ed25519

repos: []
`, workdir)
	return os.WriteFile(path, []byte(content), 0644)
}

func Uninstall() error {
	if runtime.GOOS == "darwin" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("home dir: %w", err)
		}
		plistPath := filepath.Join(home, "Library", "LaunchAgents", "com.git-builder.plist")
		binPath := filepath.Join(home, ".local", "bin", "git-builder")
		_ = run("launchctl", "unload", "-w", plistPath)
		_ = os.Remove(plistPath)
		if err := os.Remove(binPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove binary: %w", err)
		}
		fmt.Println("uninstalled git-builder service")
		return nil
	}
	if hasSystemd() {
		_ = run("systemctl", "stop", "git-builder")
		_ = run("systemctl", "disable", "git-builder")
		if err := os.Remove(installUnit); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove unit: %w", err)
		}
		if err := run("systemctl", "daemon-reload"); err != nil {
			return err
		}
	} else {
		_ = os.Remove(installUnit)
	}
	if err := os.Remove(installBin); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove binary: %w", err)
	}
	fmt.Println("uninstalled git-builder service")
	return nil
}

func copyFile(src, dst string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, b, 0755)
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
