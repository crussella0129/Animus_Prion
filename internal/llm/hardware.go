package llm

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

// GPUInfo holds detected GPU capabilities.
type GPUInfo struct {
	Available bool
	Name      string
	VRAMMiB   int // total VRAM in MiB
}

// HardwareProfile represents the detected system capabilities.
type HardwareProfile struct {
	GPU      GPUInfo
	RAMMiB   int
	CPUCores int
	OS       string
	Arch     string
}

// InferenceBackend is the recommended inference approach.
type InferenceBackend int

const (
	BackendBitNet    InferenceBackend = iota // CPU-optimized 1-bit inference
	BackendLlamaCPP                          // GPU-accelerated GGUF inference
)

func (b InferenceBackend) String() string {
	switch b {
	case BackendBitNet:
		return "bitnet"
	case BackendLlamaCPP:
		return "llama.cpp"
	default:
		return "unknown"
	}
}

// ModelRecommendation is the auto-selected model and backend.
type ModelRecommendation struct {
	Backend   InferenceBackend
	ModelName string // filename to look for or download
	ModelSize string // human-readable size
	Reason    string // why this was chosen
}

// DetectHardware probes the system for GPU, RAM, and CPU info.
func DetectHardware() HardwareProfile {
	return HardwareProfile{
		GPU:      detectGPU(),
		RAMMiB:   detectRAM(),
		CPUCores: runtime.NumCPU(),
		OS:       runtime.GOOS,
		Arch:     runtime.GOARCH,
	}
}

// RecommendBackend selects the best inference backend based on hardware.
//
// Decision logic:
//   - GPU with >=10GB VRAM → llama.cpp with 14B Q4 GGUF
//   - GPU with >=6GB VRAM  → llama.cpp with 7B Q4 GGUF
//   - GPU with <6GB / none → BitNet 1-bit on CPU
//
// The 6GB threshold: Qwen 7B Q4_K_M needs ~5.5GB VRAM. Below that,
// GPU can't run a useful Q4 model, so BitNet CPU is better.
func RecommendBackend(hw HardwareProfile) ModelRecommendation {
	if hw.GPU.Available && hw.GPU.VRAMMiB >= 6144 {
		if hw.GPU.VRAMMiB >= 10240 {
			return ModelRecommendation{
				Backend:   BackendLlamaCPP,
				ModelName: "Qwen2.5-Coder-14B-Instruct-Q4_K_M.gguf",
				ModelSize: "8.4 GB",
				Reason:    fmt.Sprintf("GPU: %s (%d MB VRAM) — using 14B Q4", hw.GPU.Name, hw.GPU.VRAMMiB),
			}
		}
		return ModelRecommendation{
			Backend:   BackendLlamaCPP,
			ModelName: "qwen2.5-coder-7b-instruct-q4_k_m.gguf",
			ModelSize: "4.4 GB",
			Reason:    fmt.Sprintf("GPU: %s (%d MB VRAM) — using 7B Q4", hw.GPU.Name, hw.GPU.VRAMMiB),
		}
	}

	// CPU path: BitNet
	if hw.GPU.Available {
		return ModelRecommendation{
			Backend:   BackendBitNet,
			ModelName: "bitnet-b1.58-2B-4T.gguf",
			ModelSize: "~500 MB",
			Reason:    fmt.Sprintf("GPU: %s (%d MB) too small for Q4 — using BitNet CPU", hw.GPU.Name, hw.GPU.VRAMMiB),
		}
	}
	return ModelRecommendation{
		Backend:   BackendBitNet,
		ModelName: "bitnet-b1.58-2B-4T.gguf",
		ModelSize: "~500 MB",
		Reason:    "No GPU detected — using BitNet CPU inference",
	}
}

// detectGPU finds an NVIDIA GPU and its VRAM via nvidia-smi.
func detectGPU() GPUInfo {
	out, err := exec.Command("nvidia-smi",
		"--query-gpu=name,memory.total",
		"--format=csv,noheader,nounits",
	).Output()
	if err != nil {
		return GPUInfo{Available: false}
	}

	line := strings.TrimSpace(string(out))
	if line == "" {
		return GPUInfo{Available: false}
	}

	// Parse "NVIDIA GeForce RTX 2080 Ti, 11264"
	lines := strings.SplitN(line, "\n", 2)
	parts := strings.SplitN(strings.TrimSpace(lines[0]), ", ", 2)
	if len(parts) < 2 {
		return GPUInfo{Available: true, Name: line}
	}

	vram, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
	return GPUInfo{
		Available: true,
		Name:      strings.TrimSpace(parts[0]),
		VRAMMiB:   vram,
	}
}

// detectRAM returns total system RAM in MiB.
func detectRAM() int {
	switch runtime.GOOS {
	case "windows":
		out, err := exec.Command("wmic", "ComputerSystem", "get", "TotalPhysicalMemory").Output()
		if err != nil {
			return 0
		}
		re := regexp.MustCompile(`(\d{9,})`)
		m := re.FindString(string(out))
		if m == "" {
			return 0
		}
		b, _ := strconv.ParseInt(m, 10, 64)
		return int(b / 1024 / 1024)

	case "linux":
		out, err := exec.Command("grep", "MemTotal", "/proc/meminfo").Output()
		if err != nil {
			return 0
		}
		re := regexp.MustCompile(`(\d+)`)
		m := re.FindString(string(out))
		kb, _ := strconv.ParseInt(m, 10, 64)
		return int(kb / 1024)

	case "darwin":
		out, err := exec.Command("sysctl", "-n", "hw.memsize").Output()
		if err != nil {
			return 0
		}
		b, _ := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
		return int(b / 1024 / 1024)
	}
	return 0
}

// FindBitNetServer locates the BitNet-compiled server binary.
func FindBitNetServer() (string, error) {
	if p := os.Getenv("PRION_BITNET_SERVER"); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}

	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}

	home := userHome()
	candidates := []string{
		home + "/.animus_prion/bin/bitnet-server" + suffix,
		home + "/.animus/bin/bitnet-server" + suffix,
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}

	if p, err := exec.LookPath("bitnet-server" + suffix); err == nil {
		return p, nil
	}

	return "", fmt.Errorf("bitnet-server not found")
}
