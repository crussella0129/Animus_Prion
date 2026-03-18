// Prion is a local-first LLM agent with plan-then-execute architecture.
// Built as a Go rewrite of the Python Animus agent for lightning speed.
package main

import (
	"bufio"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/crussella0129/Animus_Prion/internal/agent"
	"github.com/crussella0129/Animus_Prion/internal/config"
	"github.com/crussella0129/Animus_Prion/internal/core"
	"github.com/crussella0129/Animus_Prion/internal/llm"
	"github.com/crussella0129/Animus_Prion/internal/permission"
	"github.com/crussella0129/Animus_Prion/internal/planner"
	"github.com/crussella0129/Animus_Prion/internal/tools"
	"github.com/spf13/cobra"
)

var (
	cfgFile   string
	workspace string
	version   = "0.1.0"
)

var greetings = []string{
	"What are we building today?",
	"What can I help you with?",
	"Ready when you are.",
	"What are we working on?",
	"Let's get to work.",
	"What's on the agenda?",
	"Point me at the problem.",
	"What needs doing?",
	"Standing by for orders.",
	"Awaiting instructions.",
}

func randomGreeting() string {
	return greetings[rand.Intn(len(greetings))]
}

func printBanner() {
	fmt.Println()
	fmt.Println("  ╔═══════════════════════════════════════╗")
	fmt.Println("  ║       ANIMUS PRION  ·  Activated      ║")
	fmt.Printf("  ║  v%-6s  %s/%-6s  Qwen 7B  ║\n", version, runtime.GOOS, runtime.GOARCH)
	fmt.Println("  ╚═══════════════════════════════════════╝")
	fmt.Println()
	fmt.Printf("  %s\n\n", randomGreeting())
}

func main() {
	rootCmd := &cobra.Command{
		Use:   "prion",
		Short: "Prion — local-first LLM agent",
		Long:  "Prion is a local-first LLM agent with plan-then-execute architecture, built for lightning speed.",
		// Default action: launch interactive mode
		RunE: func(cmd *cobra.Command, args []string) error {
			return interactiveSession()
		},
	}

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: ~/.animus_prion/config.yaml)")
	rootCmd.PersistentFlags().StringVar(&workspace, "workspace", ".", "workspace root directory")

	rootCmd.AddCommand(runCmd())
	rootCmd.AddCommand(versionCmd())
	rootCmd.AddCommand(configCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// ensureServer checks if llama-server is reachable; if not, tries to start it.
func ensureServer() error {
	cfg, err := config.Load(mustConfigPath())
	if err != nil {
		return nil // non-fatal, will fail later on generate
	}

	baseURL := cfg.Model.BaseURL
	if baseURL == "" {
		baseURL = "http://127.0.0.1:8090/v1"
	}

	// Strip /v1 to get health endpoint
	healthURL := strings.TrimSuffix(baseURL, "/v1") + "/health"

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(healthURL)
	if err == nil {
		resp.Body.Close()
		if resp.StatusCode == 200 {
			return nil // server is already running
		}
	}

	// Server not running — try to start it
	fmt.Println("  Starting llama-server...")

	modelPath := cfg.Model.ModelName
	// Check common model locations
	candidates := []string{
		filepath.Join(os.Getenv("USERPROFILE"), ".animus", "models", modelPath),
		filepath.Join(os.Getenv("HOME"), ".animus", "models", modelPath),
		modelPath, // absolute path
	}

	var foundModel string
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			foundModel = c
			break
		}
	}

	if foundModel == "" {
		return fmt.Errorf("model not found: %s\n  Start llama-server manually:\n  llama-server --model <path-to-gguf> --port 8090", modelPath)
	}

	serverBin := findLlamaServer()
	if serverBin == "" {
		return fmt.Errorf("llama-server not found.\n  Start it manually:\n  llama-server --model %s --port 8090", foundModel)
	}

	// Start server in background
	cmd := exec.Command(serverBin,
		"--model", foundModel,
		"--ctx-size", "4096",
		"--n-gpu-layers", "-1",
		"--port", "8090",
		"--host", "127.0.0.1",
	)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start llama-server: %w", err)
	}

	// Wait for it to become healthy
	for i := 0; i < 30; i++ {
		time.Sleep(1 * time.Second)
		resp, err := client.Get(healthURL)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				fmt.Println("  llama-server ready.")
				return nil
			}
		}
	}

	return fmt.Errorf("llama-server started but didn't become healthy within 30s")
}

func findLlamaServer() string {
	// Check common locations
	candidates := []string{
		filepath.Join(os.Getenv("USERPROFILE"), ".animus", "bin", "llama-server.exe"),
		filepath.Join(os.Getenv("HOME"), ".animus", "bin", "llama-server.exe"),
		filepath.Join(os.Getenv("USERPROFILE"), ".animus", "bin", "llama-server"),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	// Check PATH
	if p, err := exec.LookPath("llama-server"); err == nil {
		return p
	}
	return ""
}

func mustConfigPath() string {
	p, _ := configPath()
	return p
}

// interactiveSession is the main entry point — banner + REPL.
func interactiveSession() error {
	printBanner()

	if err := ensureServer(); err != nil {
		fmt.Fprintf(os.Stderr, "  Warning: %v\n\n", err)
	}

	ag, err := setupAgent()
	if err != nil {
		return fmt.Errorf("setup failed: %w", err)
	}

	fmt.Println("  Type your task, or /help for commands, /quit to exit.")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)
	// Allow long inputs (default is 64KB which is fine, but be explicit)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for {
		fmt.Print("prion> ")
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		// Slash commands
		if strings.HasPrefix(input, "/") {
			if input == "/quit" || input == "/exit" {
				fmt.Println("Shutting down. Until next time.")
				return nil
			}
			if handleSlashCommand(input, ag) {
				continue
			}
		}

		start := time.Now()

		// Route: complex tasks → planner, simple → agent
		var response string
		if planner.IsSimpleTask(input) {
			response, err = ag.Run(input)
		} else {
			e, envErr := setupEnv()
			if envErr != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", envErr)
				continue
			}
			pe := planner.NewPlanExecutor(e.provider, e.registry, e.workspace)
			result, planErr := pe.Execute(input)
			if planErr != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", planErr)
				continue
			}
			response = result.Summary
			if !result.Success {
				response += "\n(Warning: some steps had issues)"
			}
			err = nil
		}

		elapsed := time.Since(start)

		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n\n", err)
			continue
		}

		if response != "" {
			fmt.Println()
			fmt.Println(response)
		}
		fmt.Printf("\n[%.1fs]\n\n", elapsed.Seconds())
	}

	return nil
}

// runCmd executes a single task and exits, OR launches interactive if no args given.
func runCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "run [task]",
		Short: "Execute a task (or start interactive mode with no args)",
		RunE: func(cmd *cobra.Command, args []string) error {
			// No args → interactive mode
			if len(args) == 0 {
				return interactiveSession()
			}

			task := strings.Join(args, " ")

			if err := ensureServer(); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
			}

			// Simple tasks → agent loop (fast path)
			if planner.IsSimpleTask(task) {
				ag, err := setupAgent()
				if err != nil {
					return err
				}
				response, err := ag.Run(task)
				if err != nil {
					return err
				}
				fmt.Println(response)
				return nil
			}

			// Complex tasks → plan-then-execute
			e, err := setupEnv()
			if err != nil {
				return err
			}
			pe := planner.NewPlanExecutor(e.provider, e.registry, e.workspace)
			result, err := pe.Execute(task)
			if err != nil {
				return err
			}

			if result.Summary != "" {
				fmt.Println(result.Summary)
			}
			if !result.Success {
				fmt.Fprintf(os.Stderr, "Warning: some steps failed\n")
			}
			return nil
		},
	}
}

// versionCmd prints the version.
func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Prion v%s\n", version)
		},
	}
}

// configCmd manages configuration.
func configCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage configuration",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "init",
		Short: "Create default configuration file",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := configPath()
			if err != nil {
				return err
			}
			cfg := config.DefaultConfig()
			if err := cfg.Save(path); err != nil {
				return err
			}
			fmt.Printf("Configuration written to %s\n", path)
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "show",
		Short: "Show current configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := configPath()
			if err != nil {
				return err
			}
			data, err := os.ReadFile(path)
			if err != nil {
				if os.IsNotExist(err) {
					fmt.Println("No configuration file found. Run 'prion config init' to create one.")
					return nil
				}
				return err
			}
			fmt.Println(string(data))
			return nil
		},
	})

	return cmd
}

// env holds the shared components needed by both the agent and planner.
type env struct {
	provider  llm.Provider
	registry  *tools.Registry
	workspace *core.Workspace
	cfg       *config.Config
}

// setupEnv creates the shared provider, registry, and workspace.
func setupEnv() (*env, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}

	cfg, err := config.Load(path)
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}

	if workspace != "" {
		cfg.Agent.WorkspaceRoot = workspace
	}
	if cfg.Agent.WorkspaceRoot == "" {
		cwd, _ := os.Getwd()
		cfg.Agent.WorkspaceRoot = cwd
	}

	ws, err := core.NewWorkspace(cfg.Agent.WorkspaceRoot)
	if err != nil {
		return nil, fmt.Errorf("creating workspace: %w", err)
	}

	checker := permission.NewChecker(false)
	registry := tools.NewRegistry()
	budget := tools.NewExecutionBudget(300 * time.Second)

	registry.Register(tools.NewReadFileTool(ws, checker))
	registry.Register(tools.NewWriteFileTool(ws, checker))
	registry.Register(tools.NewListFilesTool(ws))
	registry.Register(tools.NewShellTool(ws, checker, budget))
	registry.Register(tools.NewGitInitTool(ws))
	registry.Register(tools.NewGitStatusTool(ws))
	registry.Register(tools.NewGitDiffTool(ws))
	registry.Register(tools.NewGitLogTool(ws))
	registry.Register(tools.NewGitAddTool(ws))
	registry.Register(tools.NewGitCommitTool(ws))
	registry.Register(tools.NewGitBranchTool(ws))
	registry.Register(tools.NewGitCheckoutTool(ws))

	provider, err := llm.NewProvider(cfg)
	if err != nil {
		return nil, fmt.Errorf("creating LLM provider: %w", err)
	}

	return &env{provider: provider, registry: registry, workspace: ws, cfg: cfg}, nil
}

// setupAgent creates a fully configured agent from the shared environment.
func setupAgent() (*agent.Agent, error) {
	e, err := setupEnv()
	if err != nil {
		return nil, err
	}

	ag := agent.New(agent.Config{
		Provider:      e.provider,
		Registry:      e.registry,
		Workspace:     e.workspace,
		MaxTurns:      e.cfg.Agent.MaxTurns,
		SizeTier:      e.cfg.Model.SizeTier,
		ContextLength: e.cfg.Model.ContextLength,
	})

	return ag, nil
}

func configPath() (string, error) {
	if cfgFile != "" {
		return cfgFile, nil
	}
	dir, err := config.ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

// handleSlashCommand processes in-session commands.
func handleSlashCommand(input string, ag *agent.Agent) bool {
	switch {
	case input == "/help":
		fmt.Println()
		fmt.Println("  Commands:")
		fmt.Println("    /help    Show this help")
		fmt.Println("    /reset   Clear conversation history")
		fmt.Println("    /quit    Exit Prion")
		fmt.Println()
		fmt.Println("  Usage:")
		fmt.Println("    Type a task and press Enter.")
		fmt.Println("    Simple questions go through the fast agent loop.")
		fmt.Println("    Complex multi-file tasks use the planner with auto-verify.")
		fmt.Println()
		return true

	case input == "/reset":
		ag.Reset()
		fmt.Println("Conversation reset.")
		return true

	default:
		fmt.Printf("Unknown command: %s (type /help)\n", input)
		return true
	}
}
