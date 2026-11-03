package main

import "strings"

// gpuApps are programs that are likely holding the GPU; rag checks them before using it.
var gpuApps = []struct {
	what  string
	match func(name, cmd string) bool
}{
	{"LoRA training (AI Toolkit)", func(n, c string) bool {
		return strings.Contains(c, "ai-toolkit") || strings.Contains(c, "ai_toolkit") || (strings.Contains(c, `\toolkit\`) || strings.Contains(c, "/toolkit/")) && strings.Contains(c, "run.py")
	}},
	{"ComfyUI", func(n, c string) bool { return strings.Contains(n, "comfyui") || strings.Contains(c, "comfyui") }},
}
