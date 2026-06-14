package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	rtagent "github.com/None9527/RTAgent/pkg/rtagent"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	if strings.TrimSpace(os.Getenv("DASHSCOPE_API_KEY")) == "" {
		fmt.Println("set DASHSCOPE_API_KEY to run this example")
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	provider, err := rtagent.NewDashScopeQwen37PlusProviderFromEnv()
	if err != nil {
		return err
	}
	tmpDir, err := os.MkdirTemp("", "rtagent-dashscope-qwen-*")
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
		RunID:     "example-dashscope-run",
		SessionID: "example-dashscope-session",
		Input:     "请用一句中文回复：RTAgent Runtime SDK 示例运行成功。",
	}, rtagent.Identity{ActorID: "example"})
	if err != nil {
		return err
	}
	fmt.Println(projection.Output)
	return nil
}
