package rtagent

import (
	"context"
	"fmt"
	"time"
)

func (r *Runtime) buildContextPacket(ctx context.Context, cmd RuntimeCommand) (ContextPacket, error) {
	events, err := r.ListEvents(ctx, EventQuery{RunID: cmd.Scope.RunID})
	if err != nil {
		return ContextPacket{}, err
	}
	var specs []ToolSpec
	if r.toolProvider != nil {
		specs, err = r.toolProvider.ToolSpecs(ctx, cmd.Scope)
		if err != nil {
			return ContextPacket{}, fmt.Errorf("load tool specs: %w", err)
		}
	}
	contextPacketID := "ctx:" + cmd.Scope.RunID + ":" + fmt.Sprintf("%d", time.Now().UTC().UnixNano())
	toolSchemaSnapshotID := ""
	toolSchemaHash := ""
	if len(specs) > 0 {
		specs, toolSchemaSnapshotID, toolSchemaHash, err = r.persistToolSchemaSnapshot(ctx, cmd.Scope, contextPacketID, specs)
		if err != nil {
			return ContextPacket{}, err
		}
	}
	input := firstPayloadString(cmd.Payload, "objective", "input", "message")
	return ContextPacket{
		ID:                   contextPacketID,
		RunID:                cmd.Scope.RunID,
		SessionID:            cmd.Scope.SessionID,
		Scope:                cmd.Scope,
		Input:                input,
		Payload:              clonePayload(cmd.Payload),
		Events:               append([]RuntimeEventEnvelope(nil), events...),
		ToolSpecs:            append([]ToolSpec(nil), specs...),
		ToolSchemaSnapshotID: toolSchemaSnapshotID,
		ToolSchemaHash:       toolSchemaHash,
		GeneratedAt:          time.Now().UTC(),
	}, nil
}
