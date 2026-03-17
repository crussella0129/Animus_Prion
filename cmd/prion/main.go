// Prion is a local-first LLM agent with plan-then-execute architecture.
// Built as a Go rewrite of the Python Animus agent for lightning speed.
package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
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

func main() {
	rootCmd := &cobra.Command{
		Use:   "prion",
		Short: "Prion — local-first LLM agent",
		Long:  "Prion is a local-first LLM agent with plan-then-execute architecture, built for lightning speed.",
	}

	// Persistent flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: ~/.animus_prion/config.yaml)")
	rootCmd.PersistentFlags().StringVar(&workspace, "workspace", ".", "workspace root directory")

	// Subcommands
	rootCmd.AddCommand(chatCmd())
	rootCmd.AddCommand(runCmd())
	rootCmd.AddCommand(versionCmd())
	rootCmd.AddCommand(configCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// chatCmd starts an interactive REPL session.
func chatCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "chat",
		Short: "Start an interactive chat session",
		RunE: func(cmd *cobra.Command, args []string) error {
			ag, err := setupAgent()
			if err != nil {
				return err
			}

			fmt.Println("Prion v" + version + " — type /help for commands, /quit to exit")
			fmt.Println()

			scanner := bufio.NewScanner(os.Stdin)
			for {
				fmt.Print(">>> ")
				if !scanner.Scan() {
					break
				}

				input := strings.TrimSpace(scanner.Text())
				if input == "" {
					continue
				}

				// Slash commands
				if strings.HasPrefix(input, "/") {
					if handleSlashCommand(input, ag) {
						continue
					}
					if input == "/quit" || input == "/exit" {
						fmt.Println("Goodbye!")
						return nil
					}
				}

				start := time.Now()
				response, err := ag.Run(input)
				elapsed := time.Since(start)

				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					continue
				}

				fmt.Println()
				fmt.Println(response)
				fmt.Printf("\n[%.1fs]\n\n", elapsed.Seconds())
			}

			return nil
		},
	}
}

// runCmd executes a single task and exits.
// Routes complex tasks through PlanExecutor (with verify/repair and completeness checks).
// Simple tasks go through the raw agent loop for speed.
func runCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "run [task]",
		Short: "Execute a single task",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			task := strings.Join(args, " ")

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

			// Complex tasks → plan-then-execute (with verify + completeness)
			env, err := setupEnv()
			if err != nil {
				return err
			}
			pe := planner.NewPlanExecutor(env.provider, env.registry, env.workspace)
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
		fmt.Println("Commands:")
		fmt.Println("  /help    — Show this help")
		fmt.Println("  /tools   — List available tools")
		fmt.Println("  /reset   — Clear conversation history")
		fmt.Println("  /quit    — Exit")
		return true

	case input == "/tools":
		fmt.Println("Use /help for available commands")
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
