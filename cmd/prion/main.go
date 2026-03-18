// Prion is a local-first LLM agent with plan-then-execute architecture.
// Built as a Go rewrite of the Python Animus agent for lightning speed.
package main

import (
	"bufio"
	"fmt"
	"math/rand"
	"os"
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
	version   = "0.2.0"
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
	fmt.Println("  +=========================================+")
	fmt.Println("  |       ANIMUS PRION  ·  Activated        |")
	fmt.Printf("  |  v%-6s  %s/%-6s              |\n", version, runtime.GOOS, runtime.GOARCH)
	fmt.Println("  +=========================================+")
	fmt.Println()
	fmt.Printf("  %s\n\n", randomGreeting())
}

func main() {
	rootCmd := &cobra.Command{
		Use:   "prion",
		Short: "Prion — local-first LLM agent",
		Long:  "Prion is a local-first LLM agent with plan-then-execute architecture, built for lightning speed.",
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

// interactiveSession is the main entry point — banner + REPL.
func interactiveSession() error {
	printBanner()

	e, err := setupEnv()
	if err != nil {
		return fmt.Errorf("setup failed: %w", err)
	}
	// Ensure managed providers are shut down on exit
	defer shutdownProvider(e.provider)

	ag := agent.New(agent.Config{
		Provider:      e.provider,
		Registry:      e.registry,
		Workspace:     e.workspace,
		MaxTurns:      e.cfg.Agent.MaxTurns,
		SizeTier:      e.cfg.Model.SizeTier,
		ContextLength: e.cfg.Model.ContextLength,
	})

	fmt.Println("  Type your task, or /help for commands, /quit to exit.")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)
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

		var response string
		if planner.IsSimpleTask(input) {
			response, err = ag.Run(input)
		} else {
			pe := planner.NewPlanExecutor(e.provider, e.registry, e.workspace)
			result, planErr := pe.Execute(input)
			if planErr != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n\n", planErr)
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
			if len(args) == 0 {
				return interactiveSession()
			}

			task := strings.Join(args, " ")

			e, err := setupEnv()
			if err != nil {
				return err
			}
			defer shutdownProvider(e.provider)

			if planner.IsSimpleTask(task) {
				ag := agent.New(agent.Config{
					Provider:      e.provider,
					Registry:      e.registry,
					Workspace:     e.workspace,
					MaxTurns:      e.cfg.Agent.MaxTurns,
					SizeTier:      e.cfg.Model.SizeTier,
					ContextLength: e.cfg.Model.ContextLength,
				})
				response, err := ag.Run(task)
				if err != nil {
					return err
				}
				fmt.Println(response)
				return nil
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

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Prion v%s\n", version)
		},
	}
}

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

// --- Environment setup ---

type env struct {
	provider  llm.Provider
	registry  *tools.Registry
	workspace *core.Workspace
	cfg       *config.Config
}

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

	fmt.Println("  Loading model...")
	provider, err := llm.NewProvider(cfg)
	if err != nil {
		return nil, fmt.Errorf("creating provider: %w", err)
	}
	fmt.Println("  Model ready.")
	fmt.Println()

	return &env{provider: provider, registry: registry, workspace: ws, cfg: cfg}, nil
}

// shutdownProvider cleans up managed providers (kills llama-server subprocess).
func shutdownProvider(p llm.Provider) {
	if s, ok := p.(llm.Shutdowner); ok {
		s.Shutdown()
	}
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
