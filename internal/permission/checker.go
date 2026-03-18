// Package permission provides security checks for commands and paths.
// Implements deny-lists, injection detection, and network command blocking.
package permission

import (
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// DangerousDirectories that should never be accessed.
var DangerousDirectories = map[string]bool{
	"/etc":           true,
	"/sys":           true,
	"/proc":          true,
	"/boot":          true,
	"/dev":           true,
	"/root":          true,
	"C:\\Windows":    true,
	"C:\\System32":   true,
	"C:\\ProgramData": true,
}

// DangerousFiles that should never be read or written.
var DangerousFiles = map[string]bool{
	"/etc/passwd":        true,
	"/etc/shadow":        true,
	".ssh/id_rsa":        true,
	".ssh/id_ed25519":    true,
	".animus_prion/config.yaml": true,
}

// BlockedCommands that are always denied.
var BlockedCommands = map[string]bool{
	"rm -rf /":    true,
	"rm -rf /*":   true,
	"mkfs":        true,
	"dd":          true,
	":(){ :|:& };:": true, // fork bomb
}

// NetworkCommands that are blocked by default for safety.
var NetworkCommands = map[string]bool{
	"curl":      true,
	"wget":      true,
	"ssh":       true,
	"scp":       true,
	"rsync":     true,
	"nc":        true,
	"netcat":    true,
	"nmap":      true,
	"telnet":    true,
	"ftp":       true,
	"git push":  true,
	"git pull":  true,
	"git clone": true,
	"git fetch": true,
}

// DangerousCommands that require user confirmation.
var DangerousCommands = map[string]bool{
	"rm":       true,
	"sudo":     true,
	"shutdown": true,
	"reboot":   true,
	"kill":     true,
	"killall":  true,
	"chmod":    true,
	"chown":    true,
}

// injectionPattern detects shell injection attempts in commands.
// Matches: $(), backticks, ;, &&, ||, pipes, redirects
var injectionPattern = regexp.MustCompile(
	`\$\(` + // $(...)
		`|` + "`.+`" + // backtick execution
		`|;` + // command chaining
		`|&&` + // logical AND chaining
		`|\|\|` + // logical OR chaining
		`|\|` + // pipes
		`|>\s` + // output redirect
		`|<\s` + // input redirect
		`|>>`, // append redirect
)

// metacharPattern matches shell metacharacters that shouldn't appear in safe commands.
var metacharPattern = regexp.MustCompile(`[;|&<>` + "`" + `$()]`)

// Result represents the outcome of a permission check.
type Result struct {
	Allowed bool
	Reason  string
}

// Checker performs security validation on commands and paths.
type Checker struct {
	allowNetwork bool
}

// NewChecker creates a permission checker.
func NewChecker(allowNetwork bool) *Checker {
	return &Checker{allowNetwork: allowNetwork}
}

// HasInjectionPattern checks if a command string contains injection patterns.
func (c *Checker) HasInjectionPattern(command string) bool {
	return injectionPattern.MatchString(command)
}

// HasMetachars checks if a command string contains shell metacharacters.
func (c *Checker) HasMetachars(command string) bool {
	return metacharPattern.MatchString(command)
}

// IsPathSafe checks if a file path is safe to access.
// Follows symlinks and checks against deny lists.
func (c *Checker) IsPathSafe(path string) Result {
	// Normalize the path
	cleaned := filepath.Clean(path)

	// Resolve symlinks
	real, err := filepath.EvalSymlinks(cleaned)
	if err != nil {
		// Path doesn't exist yet — check the cleaned version
		real = cleaned
	}

	// Check against dangerous directories
	// Use dir + separator to avoid prefix collision (e.g. "/etcetera" matching "/etc")
	normalReal := strings.ToLower(filepath.Clean(real))
	for dir := range DangerousDirectories {
		normalDir := strings.ToLower(filepath.Clean(dir))
		dirWithSep := normalDir + string(filepath.Separator)
		if normalReal == normalDir || strings.HasPrefix(normalReal, dirWithSep) {
			return Result{Allowed: false, Reason: "path is in a dangerous directory: " + dir}
		}
	}

	// Check against dangerous files
	for file := range DangerousFiles {
		if strings.HasSuffix(strings.ToLower(real), strings.ToLower(file)) {
			return Result{Allowed: false, Reason: "path matches a dangerous file: " + file}
		}
	}

	return Result{Allowed: true}
}

// CheckCommand validates a command for safety.
// Returns a Result indicating whether the command is allowed.
func (c *Checker) CheckCommand(command string) Result {
	lower := strings.ToLower(strings.TrimSpace(command))

	// Check blocked commands (exact match)
	for blocked := range BlockedCommands {
		if strings.HasPrefix(lower, strings.ToLower(blocked)) {
			return Result{Allowed: false, Reason: "command is blocked: " + blocked}
		}
	}

	// Check injection patterns
	if c.HasInjectionPattern(command) {
		return Result{Allowed: false, Reason: "command contains injection pattern"}
	}

	// Check network commands (unless allowed)
	if !c.allowNetwork {
		for netCmd := range NetworkCommands {
			// Check if the command starts with or contains the network command
			parts := strings.Fields(lower)
			if len(parts) > 0 && parts[0] == strings.ToLower(netCmd) {
				return Result{Allowed: false, Reason: "network command blocked: " + netCmd}
			}
			// Two-word commands like "git push"
			if len(parts) > 1 && strings.ToLower(parts[0]+" "+parts[1]) == strings.ToLower(netCmd) {
				return Result{Allowed: false, Reason: "network command blocked: " + netCmd}
			}
		}
	}

	return Result{Allowed: true}
}

// IsDangerous checks if a command requires user confirmation.
func (c *Checker) IsDangerous(command string) bool {
	parts := strings.Fields(strings.TrimSpace(command))
	if len(parts) == 0 {
		return false
	}
	cmd := strings.ToLower(parts[0])

	// On Windows, strip .exe suffix
	if runtime.GOOS == "windows" {
		cmd = strings.TrimSuffix(cmd, ".exe")
	}

	return DangerousCommands[cmd]
}
