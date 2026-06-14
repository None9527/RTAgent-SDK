package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	rtagent "github.com/None9527/RTAgent-SDK/pkg/rtagent"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	if strings.TrimSpace(os.Getenv("OPENAI_API_KEY")) == "" {
		fmt.Println("set OPENAI_API_KEY to run this example")
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	contextWindowTokens := 128000
	if raw := strings.TrimSpace(os.Getenv("OPENAI_CONTEXT_WINDOW_TOKENS")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			return fmt.Errorf("OPENAI_CONTEXT_WINDOW_TOKENS must be an integer: %w", err)
		}
		contextWindowTokens = parsed
	}

	provider, err := rtagent.NewOpenAICompatibleProvider(rtagent.OpenAICompatibleProviderConfig{
		BaseURL:             firstNonEmpty(os.Getenv("OPENAI_BASE_URL"), rtagent.DefaultOpenAICompatibleBaseURL),
		APIKey:              os.Getenv("OPENAI_API_KEY"),
		Model:               firstNonEmpty(os.Getenv("OPENAI_MODEL"), "gpt-4o"),
		ContextWindowTokens: contextWindowTokens,
	})
	if err != nil {
		return err
	}
	tmpDir, err := os.MkdirTemp("", "rtagent-openai-compatible-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	rt, err := rtagent.Open(ctx, rtagent.Config{
		Runtime: rtagent.RuntimeConfig{
			DSN:     filepath.Join(tmpDir, "runtime.db"),
			WorkDir: tmpDir,
		},
		Host: rtagent.HostPorts{Model: provider},
	})
	if err != nil {
		return err
	}
	defer rt.Close()

	projection, err := rt.SubmitRun(ctx, rtagent.SubmitRunRequest{
		RunID:     "example-openai-compatible-run",
		SessionID: "example-openai-compatible-session",
		Input:     "Reply in one sentence: RTAgent Runtime SDK example completed.",
	}, rtagent.Identity{ActorID: "example"})
	if err != nil {
		return err
	}
	fmt.Println(projection.Output)
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
