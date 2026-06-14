package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	rtagent "rtagent/pkg/rtagent"
)

type cliConfig struct {
	dbPath    string
	sessionID string
	resumeID  string
	input     string
	showGraph bool
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg := parseFlags()

	ctx := context.Background()
	rt, err := rtagent.Open(ctx, rtagent.Config{
		Runtime: rtagent.RuntimeConfig{
			DSN:     cfg.dbPath,
			WorkDir: os.TempDir(),
		},
	})
	if err != nil {
		return err
	}
	defer rt.Close()

	targetSession := cfg.sessionID
	if cfg.resumeID != "" {
		session, err := rt.InspectSession(ctx, rtagent.SessionQuery{SessionID: cfg.resumeID})
		if err != nil {
			return fmt.Errorf("inspect session %q: %w", cfg.resumeID, err)
		}
		if !session.CanResume || !session.ExternalResumeReady {
			return fmt.Errorf("session %q is not externally resumable", cfg.resumeID)
		}
		printSession("resume", session)
		targetSession = cfg.resumeID
	}

	runID := "example-host-run-" + fmt.Sprint(time.Now().UnixNano())
	projection, err := rt.SubmitRun(ctx, rtagent.SubmitRunRequest{
		RunID:     runID,
		SessionID: targetSession,
		Input:     cfg.input,
	}, rtagent.Identity{ActorID: "example"})
	if err != nil {
		return err
	}
	fmt.Printf("run id=%s session=%s status=%s output=%q\n", projection.RunID, projection.SessionID, projection.Status, projection.Output)

	session, err := rt.InspectSession(ctx, rtagent.SessionQuery{SessionID: targetSession})
	if err != nil {
		return fmt.Errorf("inspect updated session %q: %w", targetSession, err)
	}
	printSession("after", session)
	if cfg.showGraph {
		graph, err := rt.SessionGraph(ctx, rtagent.SessionGraphQuery{SessionID: targetSession})
		if err != nil {
			return fmt.Errorf("inspect session graph %q: %w", targetSession, err)
		}
		fmt.Printf("graph session=%s nodes=%d edges=%d latest_run=%s\n", graph.SessionID, len(graph.Nodes), len(graph.Edges), graph.LatestRunID)
	}
	return nil
}

func parseFlags() cliConfig {
	defaultDB := filepath.Join(os.TempDir(), "rtagent-host-resume-cli.db")
	dbPath := flag.String("db", defaultDB, "sqlite dsn/path used to persist host sessions")
	sessionID := flag.String("session", "example-host-session", "session id for a new or continued conversation")
	resumeID := flag.String("resume", "", "existing session id to inspect and continue")
	input := flag.String("input", "continue conversation", "user input for the next run")
	showGraph := flag.Bool("graph", false, "print the SDK session graph after the run")
	flag.Parse()

	return cliConfig{
		dbPath:    strings.TrimSpace(*dbPath),
		sessionID: strings.TrimSpace(*sessionID),
		resumeID:  strings.TrimSpace(*resumeID),
		input:     *input,
		showGraph: *showGraph,
	}
}

func printSession(label string, session rtagent.SessionSnapshot) {
	fmt.Printf(
		"session[%s] id=%s status=%s latest_run=%s runs=%d can_resume=%v hint=%q\n",
		label,
		session.SessionID,
		session.Status,
		session.LatestRunID,
		session.RunCount,
		session.CanResume,
		session.ResumeCommandHint,
	)
}
