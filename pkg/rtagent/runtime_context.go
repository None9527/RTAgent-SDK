package rtagent

import (
	"context"
	"strings"

	"github.com/None9527/RTAgent-SDK/internal/domain/contextual"
)

func (r *Runtime) RegisterContextHandle(ctx context.Context, handle ContextHandle) error {
	if err := r.ensureReady(); err != nil {
		return err
	}
	handle.RunID = strings.TrimSpace(handle.RunID)
	if err := r.ensureRunExists(ctx, handle.RunID); err != nil {
		return err
	}
	return r.kernel.contextRegistry.Register(ctx, contextual.ContextHandle{
		HandleID:              handle.HandleID,
		RunID:                 handle.RunID,
		Kind:                  contextual.HandleKind(handle.Kind),
		Title:                 handle.Title,
		Summary:               handle.Summary,
		SourceRef:             handle.SourceRef,
		TokenEstimate:         handle.TokenEstimate,
		Freshness:             handle.Freshness,
		MaterializationPolicy: handle.MaterializationPolicy,
		EvidenceRefs:          append([]string(nil), handle.EvidenceRefs...),
	})
}

func (r *Runtime) MaterializeContext(ctx context.Context, handleID string) (string, error) {
	if err := r.ensureReady(); err != nil {
		return "", err
	}
	return r.kernel.materializer.Materialize(ctx, handleID)
}
