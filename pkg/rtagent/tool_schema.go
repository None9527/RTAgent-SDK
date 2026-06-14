package rtagent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/None9527/RTAgent/internal/domain/persistence"
)

type toolSchemaSnapshotPayload struct {
	Specs []ToolSpec `json:"specs"`
}

func (r *Runtime) persistToolSchemaSnapshot(ctx context.Context, scope ExecutionScope, contextPacketID string, specs []ToolSpec) ([]ToolSpec, string, string, error) {
	normalized, err := normalizeToolSpecsForSnapshot(specs)
	if err != nil {
		return nil, "", "", err
	}
	if len(normalized) == 0 {
		return normalized, "", "", nil
	}
	hash, snapshotJSON, err := toolSchemaSnapshotHash(normalized)
	if err != nil {
		return nil, "", "", err
	}
	snapshotID := "tool_schema:" + scope.RunID + ":" + shortHash(contextPacketID+"|"+hash)
	if err := r.kernel.store.PutToolSchemaSnapshot(ctx, persistence.ToolSchemaSnapshotRecord{
		SnapshotID:      snapshotID,
		RunID:           scope.RunID,
		ContextPacketID: contextPacketID,
		SchemaHash:      hash,
		ToolCount:       len(normalized),
		SnapshotJSON:    snapshotJSON,
		CreatedAt:       time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		return nil, "", "", fmt.Errorf("put tool schema snapshot: %w", err)
	}
	return normalized, snapshotID, hash, nil
}

func normalizeToolSpecsForSnapshot(specs []ToolSpec) ([]ToolSpec, error) {
	out := make([]ToolSpec, 0, len(specs))
	seen := map[string]struct{}{}
	for _, spec := range specs {
		spec = cloneToolSpec(spec)
		spec.Name = strings.TrimSpace(spec.Name)
		if spec.Name == "" {
			continue
		}
		if _, ok := seen[spec.Name]; ok {
			return nil, fmt.Errorf("duplicate tool spec name %q", spec.Name)
		}
		seen[spec.Name] = struct{}{}
		if strings.TrimSpace(spec.SchemaHash) == "" {
			hash, err := toolSpecHash(spec)
			if err != nil {
				return nil, err
			}
			spec.SchemaHash = hash
		}
		if strings.TrimSpace(spec.Epoch) == "" {
			spec.Epoch = firstNonEmpty(spec.Version, spec.SchemaHash)
		}
		out = append(out, spec)
	}
	return out, nil
}

func validateToolCallAgainstSpec(call ToolCall, spec *ToolSpec) error {
	if call.EpochClosed {
		return errors.New("tool call references a closed epoch")
	}
	if spec == nil {
		if strings.TrimSpace(call.SchemaHash) != "" || strings.TrimSpace(call.Epoch) != "" {
			return fmt.Errorf("tool %q has schema metadata but no current tool spec", call.Name)
		}
		return nil
	}
	if callHash := strings.TrimSpace(call.SchemaHash); callHash != "" {
		specHash := strings.TrimSpace(spec.SchemaHash)
		if specHash == "" {
			return fmt.Errorf("tool %q has schema hash %q but current spec has no schema hash", call.Name, callHash)
		}
		if callHash != specHash {
			return fmt.Errorf("tool %q schema hash mismatch: call=%s current=%s", call.Name, callHash, specHash)
		}
	}
	if callEpoch := strings.TrimSpace(call.Epoch); callEpoch != "" {
		specEpoch := firstNonEmpty(spec.Epoch, spec.Version, spec.SchemaHash)
		if specEpoch == "" {
			return fmt.Errorf("tool %q has epoch %q but current spec has no epoch", call.Name, callEpoch)
		}
		if callEpoch != specEpoch {
			return fmt.Errorf("tool %q epoch mismatch: call=%s current=%s", call.Name, callEpoch, specEpoch)
		}
	}
	return nil
}

func toolSchemaSnapshotHash(specs []ToolSpec) (string, string, error) {
	sorted := append([]ToolSpec(nil), specs...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].Name < sorted[j].Name
	})
	body, err := json.Marshal(toolSchemaSnapshotPayload{Specs: sorted})
	if err != nil {
		return "", "", fmt.Errorf("marshal tool schema snapshot: %w", err)
	}
	return sha256Hex(body), string(body), nil
}

func toolSpecHash(spec ToolSpec) (string, error) {
	spec = cloneToolSpec(spec)
	spec.SchemaHash = ""
	body, err := json.Marshal(spec)
	if err != nil {
		return "", fmt.Errorf("marshal tool spec %q: %w", spec.Name, err)
	}
	return sha256Hex(body), nil
}

func sha256Hex(body []byte) string {
	sum := sha256.Sum256(body)
	return "sha256:" + hex.EncodeToString(sum[:])
}
