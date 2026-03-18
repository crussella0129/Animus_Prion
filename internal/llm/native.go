package llm

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// NativeProvider manages a llama-server subprocess and delegates Generate()
// to a LocalProvider speaking the OpenAI-compatible protocol over localhost.
// This keeps the build pure Go (no CGo) while providing "just works" local inference.
type NativeProvider struct {
	process      *exec.Cmd
	inner        *LocalProvider
	port         int
	caps         ModelCapabilities
	shutdownOnce sync.Once
	shutdownErr  error
}

// NativeProviderConfig configures the native provider.
type NativeProviderConfig struct {
	ModelPath     string
	GPULayers     int
	ContextLength int
	SizeTier      string
	ServerPath    string // override for llama-server location
}

// NewNativeProvider launches llama-server as a child process and wraps it.
func NewNativeProvider(cfg NativeProviderConfig) (*NativeProvider, error) {
	// 1. Locate llama-server binary
	serverPath := cfg.ServerPath
	if serverPath == "" {
		var err error
		serverPath, err = FindLlamaServer()
		if err != nil {
			return nil, fmt.Errorf("llama-server not found: %w\n  Run 'prion setup' to download, or set $PRION_LLAMA_SERVER", err)
		}
	}

	// 2. Verify model file exists
	if _, err := os.Stat(cfg.ModelPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("model file not found: %s", cfg.ModelPath)
	}

	// 3. Pick a free port
	port, err := freePort()
	if err != nil {
		return nil, fmt.Errorf("finding free port: %w", err)
	}

	// 4. Build command
	gpuLayers := cfg.GPULayers
	if gpuLayers == 0 {
		gpuLayers = -1 // auto-detect
	}
	ctxSize := cfg.ContextLength
	if ctxSize == 0 {
		ctxSize = 4096
	}
	threads := runtime.NumCPU()
	if threads > 8 {
		threads = 8 // cap to avoid oversubscription
	}

	cmd := exec.Command(serverPath,
		"--model", cfg.ModelPath,
		"--port", strconv.Itoa(port),
		"--host", "127.0.0.1",
		"--n-gpu-layers", strconv.Itoa(gpuLayers),
		"--ctx-size", strconv.Itoa(ctxSize),
		"--threads", strconv.Itoa(threads),
	)
	// Suppress server logs by default (send to /dev/null)
	// Set PRION_SERVER_LOGS=1 to see llama-server output
	if os.Getenv("PRION_SERVER_LOGS") == "1" {
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting llama-server: %w", err)
	}

	// 5. Wait for server to become healthy
	baseURL := fmt.Sprintf("http://127.0.0.1:%d/v1", port)
	healthURL := fmt.Sprintf("http://127.0.0.1:%d/health", port)

	if err := waitForHealth(healthURL, 60*time.Second); err != nil {
		// Kill the process if it never became healthy
		cmd.Process.Kill()
		cmd.Wait()
		return nil, fmt.Errorf("llama-server failed to start: %w", err)
	}

	// 6. Create inner LocalProvider for actual HTTP calls
	sizeTier := cfg.SizeTier
	if sizeTier == "" {
		sizeTier = "medium"
	}

	inner := NewLocalProvider(LocalProviderConfig{
		BaseURL:       baseURL,
		APIKey:        "not-needed",
		Model:         filepath.Base(cfg.ModelPath),
		ContextLength: ctxSize,
		SizeTier:      sizeTier,
	})

	return &NativeProvider{
		process: cmd,
		inner:   inner,
		port:    port,
		caps: ModelCapabilities{
			ContextLength: ctxSize,
			SizeTier:      sizeTier,
			SupportsTools: false,
			SupportsJSON:  false,
		},
	}, nil
}

// Generate delegates to the inner LocalProvider.
func (p *NativeProvider) Generate(ctx context.Context, messages []Message, opts GenerateOptions) (string, error) {
	return p.inner.Generate(ctx, messages, opts)
}

// Available returns true if the managed server is healthy.
func (p *NativeProvider) Available() bool {
	return p.inner.Available()
}

// Capabilities returns the model capabilities.
func (p *NativeProvider) Capabilities() ModelCapabilities {
	return p.caps
}

// Shutdown gracefully stops the llama-server process.
// Idempotent — safe to call multiple times (deferred + explicit).
func (p *NativeProvider) Shutdown() error {
	p.shutdownOnce.Do(func() {
		if p.process == nil || p.process.Process == nil {
			return
		}

		done := make(chan error, 1)
		go func() { done <- p.process.Wait() }()

		if runtime.GOOS == "windows" {
			p.process.Process.Kill()
		} else {
			p.process.Process.Signal(os.Interrupt)
			select {
			case err := <-done:
				p.shutdownErr = err
				return
			case <-time.After(5 * time.Second):
				p.process.Process.Kill()
			}
		}

		p.shutdownErr = <-done
	})
	return p.shutdownErr
}

// Port returns the port the managed server is listening on.
func (p *NativeProvider) Port() int {
	return p.port
}

// --- Helper functions ---

// FindLlamaServer locates the llama-server binary.
// Search order: $PRION_LLAMA_SERVER → ~/.animus_prion/bin → ~/.animus/bin → $PATH
func FindLlamaServer() (string, error) {
	// 1. Environment variable override
	if p := os.Getenv("PRION_LLAMA_SERVER"); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}

	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}

	// 2. Prion's own bin directory
	home := userHome()
	candidates := []string{
		filepath.Join(home, ".animus_prion", "bin", "llama-server"+suffix),
		filepath.Join(home, ".animus", "bin", "llama-server"+suffix),
	}

	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}

	// 3. $PATH
	if p, err := exec.LookPath("llama-server" + suffix); err == nil {
		return p, nil
	}

	return "", fmt.Errorf("llama-server not found in $PRION_LLAMA_SERVER, ~/.animus_prion/bin, ~/.animus/bin, or $PATH")
}

// FindModel locates a GGUF model file.
// Search order: exact path → ~/.animus_prion/models/ → ~/.animus/models/
func FindModel(nameOrPath string) (string, error) {
	// Exact path
	if _, err := os.Stat(nameOrPath); err == nil {
		return nameOrPath, nil
	}

	home := userHome()
	candidates := []string{
		filepath.Join(home, ".animus_prion", "models", nameOrPath),
		filepath.Join(home, ".animus", "models", nameOrPath),
	}

	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
		// Try with .gguf suffix
		if !strings.HasSuffix(c, ".gguf") {
			withGGUF := c + ".gguf"
			if _, err := os.Stat(withGGUF); err == nil {
				return withGGUF, nil
			}
		}
	}

	return "", fmt.Errorf("model not found: %s (searched ~/.animus_prion/models/ and ~/.animus/models/)", nameOrPath)
}

// freePort asks the OS for an available TCP port.
func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// waitForHealth polls a health endpoint until it returns 200 or timeout.
func waitForHealth(url string, timeout time.Duration) error {
	client := &http.Client{Timeout: 2 * time.Second}
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				return nil
			}
		}
		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("server at %s did not become healthy within %v", url, timeout)
}

func userHome() string {
	if h := os.Getenv("USERPROFILE"); h != "" {
		return h // Windows
	}
	if h := os.Getenv("HOME"); h != "" {
		return h // Unix
	}
	h, _ := os.UserHomeDir()
	return h
}
