//go:build linux

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScriptAddrShell(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "start-qwen.sh")
	os.WriteFile(p, []byte("#!/bin/bash\nPORT=8091\n./llama-server -m model.gguf --host 127.0.0.1 --port $PORT -ngl 99\n"), 0o755)
	if h, port := scriptAddr(p); h != "127.0.0.1" || port != 8091 {
		t.Fatalf("got %s:%d", h, port)
	}
	os.WriteFile(p, []byte("./llama-server -m m.gguf --port 8099\n"), 0o755)
	if _, port := scriptAddr(p); port != 8099 {
		t.Fatalf("got port %d", port)
	}
}

func TestLinuxDefaultAIScript(t *testing.T) {
	h := t.TempDir()
	t.Setenv("HOME", h)
	if s := linuxDefaultAIScript(); s != "" {
		t.Fatalf("found %q in an empty home", s)
	}
	os.MkdirAll(filepath.Join(h, "llama.cpp"), 0o755)
	for _, n := range []string{"start-z.sh", "start-a.sh", "notes.sh"} {
		os.WriteFile(filepath.Join(h, "llama.cpp", n), []byte("#!/bin/sh\n"), 0o755)
	}
	if s := linuxDefaultAIScript(); filepath.Base(s) != "start-a.sh" {
		t.Fatalf("picked %q", s)
	}
}
