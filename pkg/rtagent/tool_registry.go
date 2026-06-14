package rtagent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
)

type ToolRegistry struct {
	mu        sync.RWMutex
	providers []ToolProvider
}

type registryTool struct {
	provider      ToolProvider
	providerIndex int
	originalName  string
	exposedName   string
	spec          ToolSpec
}

func NewToolRegistry(providers ...ToolProvider) *ToolRegistry {
	registry := &ToolRegistry{}
	for _, provider := range providers {
		if provider != nil {
			registry.providers = append(registry.providers, provider)
		}
	}
	return registry
}

func (r *ToolRegistry) Register(provider ToolProvider) error {
	if provider == nil {
		return errors.New("tool provider is nil")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers = append(r.providers, provider)
	return nil
}

func (r *ToolRegistry) ToolSpecs(ctx context.Context, scope ExecutionScope) ([]ToolSpec, error) {
	if r == nil {
		return nil, errors.New("tool registry is nil")
	}
	tools, err := r.registryTools(ctx, scope)
	if err != nil {
		return nil, err
	}
	specs := make([]ToolSpec, 0, len(tools))
	for _, tool := range tools {
		specs = append(specs, cloneToolSpec(tool.spec))
	}
	return specs, nil
}

func (r *ToolRegistry) ExecuteTool(ctx context.Context, scope ExecutionScope, call ToolCall) (ToolObservation, error) {
	if r == nil {
		return ToolObservation{}, errors.New("tool registry is nil")
	}
	name := strings.TrimSpace(call.Name)
	if name == "" {
		return ToolObservation{}, errors.New("tool call name is required")
	}
	tools, err := r.registryTools(ctx, scope)
	if err != nil {
		return ToolObservation{}, err
	}
	var matched *registryTool
	var ambiguous []string
	for _, tool := range tools {
		tool := tool
		if tool.exposedName == name {
			matched = &tool
			break
		}
		if tool.originalName == name && tool.exposedName != name {
			ambiguous = append(ambiguous, tool.exposedName)
		}
	}
	if matched == nil && len(ambiguous) > 0 {
		return ToolObservation{}, fmt.Errorf("tool %q is ambiguous; use one of: %s", name, strings.Join(ambiguous, ", "))
	}
	if matched == nil {
		return ToolObservation{}, fmt.Errorf("tool %q is not registered", name)
	}
	providerCall := call
	providerCall.Name = matched.originalName
	observation, err := matched.provider.ExecuteTool(ctx, scope, providerCall)
	if err != nil {
		return ToolObservation{}, err
	}
	if strings.TrimSpace(observation.Name) == "" || strings.TrimSpace(observation.Name) == matched.originalName {
		observation.Name = matched.exposedName
	}
	return observation, nil
}

func (r *ToolRegistry) snapshotProviders() []ToolProvider {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]ToolProvider(nil), r.providers...)
}

func configuredToolProvider(configured []ToolProvider) ToolProvider {
	providers := make([]ToolProvider, 0, len(configured))
	for _, provider := range configured {
		if provider != nil {
			providers = append(providers, provider)
		}
	}
	switch len(providers) {
	case 0:
		return nil
	case 1:
		return providers[0]
	default:
		return NewToolRegistry(providers...)
	}
}

func (r *ToolRegistry) registryTools(ctx context.Context, scope ExecutionScope) ([]registryTool, error) {
	providers := r.snapshotProviders()
	tools := make([]registryTool, 0)
	nameCounts := map[string]int{}
	for index, provider := range providers {
		providerSpecs, err := provider.ToolSpecs(ctx, scope)
		if err != nil {
			return nil, fmt.Errorf("load tool specs from provider %d: %w", index, err)
		}
		for _, spec := range providerSpecs {
			name := strings.TrimSpace(spec.Name)
			if name == "" {
				continue
			}
			cloned := cloneToolSpec(spec)
			cloned.Name = name
			tools = append(tools, registryTool{
				provider:      provider,
				providerIndex: index,
				originalName:  name,
				exposedName:   name,
				spec:          cloned,
			})
			nameCounts[name]++
		}
	}

	namespaceSeenByName := map[string]map[string]int{}
	exposedSeen := map[string]struct{}{}
	for i := range tools {
		tool := &tools[i]
		if nameCounts[tool.originalName] > 1 {
			base := registryToolNamespace(tool.spec, tool.providerIndex)
			if namespaceSeenByName[tool.originalName] == nil {
				namespaceSeenByName[tool.originalName] = map[string]int{}
			}
			namespaceSeenByName[tool.originalName][base]++
			count := namespaceSeenByName[tool.originalName][base]
			namespace := base
			if count > 1 {
				namespace = fmt.Sprintf("%s_%d", base, count)
			}
			tool.exposedName = namespace + "__" + tool.originalName
			tool.spec.Name = tool.exposedName
			if strings.TrimSpace(tool.spec.Namespace) == "" {
				tool.spec.Namespace = namespace
			}
		}
		if _, ok := exposedSeen[tool.exposedName]; ok {
			return nil, fmt.Errorf("duplicate tool spec name %q after namespace routing", tool.exposedName)
		}
		exposedSeen[tool.exposedName] = struct{}{}
	}
	return tools, nil
}

func registryToolNamespace(spec ToolSpec, providerIndex int) string {
	raw := firstNonEmpty(spec.Namespace, spec.ProviderName, fmt.Sprintf("provider_%d", providerIndex+1))
	var out strings.Builder
	for _, r := range strings.TrimSpace(raw) {
		switch {
		case r >= 'a' && r <= 'z':
			out.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			out.WriteRune(r)
		case r >= '0' && r <= '9':
			out.WriteRune(r)
		case r == '_' || r == '-':
			out.WriteRune(r)
		default:
			out.WriteByte('_')
		}
	}
	namespace := strings.Trim(out.String(), "_-")
	if namespace == "" {
		return fmt.Sprintf("provider_%d", providerIndex+1)
	}
	return namespace
}

func cloneToolSpec(spec ToolSpec) ToolSpec {
	out := spec
	out.Parameters = clonePayload(spec.Parameters)
	out.OutputSchema = clonePayload(spec.OutputSchema)
	out.ResourceLocks = append([]ResourceLock(nil), spec.ResourceLocks...)
	out.RequiredGrants = append([]ScopedPermissionGrant(nil), spec.RequiredGrants...)
	if len(spec.Examples) > 0 {
		out.Examples = make([]map[string]any, 0, len(spec.Examples))
		for _, example := range spec.Examples {
			out.Examples = append(out.Examples, clonePayload(example))
		}
	}
	out.ExecutionConstraints.FileScope = append([]string(nil), spec.ExecutionConstraints.FileScope...)
	return out
}
