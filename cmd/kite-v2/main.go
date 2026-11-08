package main

import (
	"os"
	"strings"

	gemini "kite/internal/adapter/gemini"
	tui "kite/internal/adapter/tui"
	core "kite/internal/core"
)

func main() {
	// A missing .env is fine. FromEnv reports any variable that is still missing.
	loadEnv(".env")

	geminiClient, err := gemini.FromEnv()
	if err != nil {
		panic(err)
	}
	//here it satisfied as geminiClient has implemented the core.LLM interface  as it has complete method
	kite := core.NewAgent(geminiClient)
	tui.Init(kite)
}

// loadEnv copies KEY=VALUE lines from path into the environment.
// Variables already set are left alone.
func loadEnv(path string) {
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "" || os.Getenv(key) != "" {
			continue
		}
		os.Setenv(key, strings.TrimSpace(val))
	}
}
