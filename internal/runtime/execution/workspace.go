package execution

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"rtagent/internal/domain/persistence"
)

type ManagedWorkspace struct {
	workDir string
	store   persistence.Bundle
}

func NewManagedWorkspace(workDir string, store persistence.Bundle) *ManagedWorkspace {
	return &ManagedWorkspace{
		workDir: filepath.Clean(workDir),
		store:   store,
	}
}

// WriteFile performs an audited write to the workspace with SHA-256 tamper checks
func (w *ManagedWorkspace) WriteFile(ctx context.Context, relativePath string, content []byte, activeActivityID string, runID string) (persistence.ArtifactRecord, error) {
	// 1. Resolve and clean the target path
	cleanRel := filepath.Clean(relativePath)
	if filepath.IsAbs(cleanRel) || strings.HasPrefix(cleanRel, "..") {
		return persistence.ArtifactRecord{}, fmt.Errorf("security violation: path %s is absolute or points outside the workspace", relativePath)
	}

	absPath := filepath.Join(w.workDir, cleanRel)
	if !strings.HasPrefix(absPath, w.workDir) {
		return persistence.ArtifactRecord{}, fmt.Errorf("security violation: path %s escapes workspace root", cleanRel)
	}

	// 2. Perform SHA-256 tamper verification if the file already exists physically
	if _, err := os.Stat(absPath); err == nil {
		physicalHash, hashErr := w.calculateSHA256(absPath)
		if hashErr != nil {
			return persistence.ArtifactRecord{}, fmt.Errorf("calculate physical file hash: %w", hashErr)
		}

		// Retrieve latest registered artifact from the Truth store
		dbArtifact, dbErr := w.store.GetLatestArtifactByPath(ctx, cleanRel)
		if dbErr == nil && dbArtifact.ArtifactID != "" {
			if dbArtifact.SHA256 != physicalHash {
				return persistence.ArtifactRecord{}, fmt.Errorf("tamper detected: file %s SHA-256 mismatch (physical: %s, database: %s)", cleanRel, physicalHash, dbArtifact.SHA256)
			}
		}
	}

	// 3. Ensure parent directory structure exists
	parentDir := filepath.Dir(absPath)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return persistence.ArtifactRecord{}, fmt.Errorf("create parent directories: %w", err)
	}

	// 4. Perform physical write
	if err := os.WriteFile(absPath, content, 0644); err != nil {
		return persistence.ArtifactRecord{}, fmt.Errorf("write physical file: %w", err)
	}

	// 5. Calculate new hash of written file
	newHash, err := w.calculateSHA256(absPath)
	if err != nil {
		return persistence.ArtifactRecord{}, fmt.Errorf("calculate new file hash: %w", err)
	}

	preview := string(content)
	if len(preview) > 1000 {
		preview = preview[:1000]
	}

	// 6. Generate and register versioned ArtifactRecord
	artID := fmt.Sprintf("art_%s_%d", filepath.Base(cleanRel), time.Now().UnixNano())
	artRec := persistence.ArtifactRecord{
		ArtifactID: artID,
		Source: persistence.SourceRef{
			Kind:     persistence.SourceArtifact,
			RunID:    runID,
			RecordID: activeActivityID,
		},
		Kind:      "file",
		Path:      cleanRel,
		SHA256:    newHash,
		ByteSize:  len(content),
		Preview:   preview,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}

	if err := w.store.PutArtifact(ctx, artRec); err != nil {
		return persistence.ArtifactRecord{}, fmt.Errorf("audit artifact registration failed: %w", err)
	}

	return artRec, nil
}

func (w *ManagedWorkspace) calculateSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
