package llm

import (
	"testing"
)

func TestRecommendBackendGPULarge(t *testing.T) {
	hw := HardwareProfile{
		GPU:      GPUInfo{Available: true, Name: "RTX 3090", VRAMMiB: 24576},
		RAMMiB:   32768,
		CPUCores: 16,
	}
	rec := RecommendBackend(hw)
	if rec.Backend != BackendLlamaCPP {
		t.Errorf("expected llama.cpp for 24GB GPU, got %v", rec.Backend)
	}
	if rec.ModelName != "Qwen2.5-Coder-14B-Instruct-Q4_K_M.gguf" {
		t.Errorf("expected 14B model for 24GB, got %s", rec.ModelName)
	}
}

func TestRecommendBackendGPUMedium(t *testing.T) {
	hw := HardwareProfile{
		GPU:      GPUInfo{Available: true, Name: "RTX 2060", VRAMMiB: 6144},
		RAMMiB:   16384,
		CPUCores: 8,
	}
	rec := RecommendBackend(hw)
	if rec.Backend != BackendLlamaCPP {
		t.Errorf("expected llama.cpp for 6GB GPU, got %v", rec.Backend)
	}
	if rec.ModelName != "qwen2.5-coder-7b-instruct-q4_k_m.gguf" {
		t.Errorf("expected 7B model for 6GB, got %s", rec.ModelName)
	}
}

func TestRecommendBackendGPUTooSmall(t *testing.T) {
	hw := HardwareProfile{
		GPU:      GPUInfo{Available: true, Name: "GTX 1050", VRAMMiB: 2048},
		RAMMiB:   8192,
		CPUCores: 4,
	}
	rec := RecommendBackend(hw)
	if rec.Backend != BackendBitNet {
		t.Errorf("expected BitNet for 2GB GPU, got %v", rec.Backend)
	}
}

func TestRecommendBackendNoGPU(t *testing.T) {
	hw := HardwareProfile{
		GPU:      GPUInfo{Available: false},
		RAMMiB:   16384,
		CPUCores: 8,
	}
	rec := RecommendBackend(hw)
	if rec.Backend != BackendBitNet {
		t.Errorf("expected BitNet for no GPU, got %v", rec.Backend)
	}
}

func TestDetectHardware(t *testing.T) {
	// Smoke test — just make sure it doesn't crash
	hw := DetectHardware()
	if hw.CPUCores <= 0 {
		t.Error("expected positive CPU core count")
	}
	if hw.OS == "" {
		t.Error("expected non-empty OS")
	}
}
