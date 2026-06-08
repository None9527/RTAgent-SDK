package execution

import (
	"context"
	"errors"
	"fmt"

	"github.com/xeipuuv/gojsonschema"
)

type AgentInput struct {
	RunID                string                 `json:"run_id"`
	ActivityID           string                 `json:"activity_id"`
	Objective            string                 `json:"objective"`
	WorldStateSnapshot   map[string]interface{} `json:"world_state_snapshot"`
	ContextHandles       []interface{}          `json:"context_handles"`
	MaterializedContext  []interface{}          `json:"materialized_context"`
	AvailableCapabilities []string               `json:"available_capabilities"`
}

type Claim struct {
	ClaimType    string   `json:"claim_type"` // observed_fact, inference, hypothesis, recommendation
	Statement    string   `json:"statement"`
	EvidenceRefs []string `json:"evidence_refs,omitempty"`
}

type RequestedAction struct {
	ActionID string                 `json:"action_id"`
	Kind     string                 `json:"kind"`
	Arguments map[string]interface{} `json:"arguments"`
}

type AgentOutput struct {
	ReasoningSummary string            `json:"reasoning_summary"`
	ProposedPlan     []string          `json:"proposed_plan"`
	RequestedActions []RequestedAction `json:"requested_actions"`
	Claims           []Claim           `json:"claims"`
	CompletionStatus string            `json:"completion_status"` // in_progress, completed, failed, blocked
}

type LLMClient interface {
	Call(ctx context.Context, prompt string) (string, error)
}

type GovernedExecutor struct {
	outputSchemaLoader gojsonschema.JSONLoader
	llmProvider        LLMClient
}

func NewGovernedExecutor(schemaJSON string, llm LLMClient) *GovernedExecutor {
	return &GovernedExecutor{
		outputSchemaLoader: gojsonschema.NewStringLoader(schemaJSON),
		llmProvider:        llm,
	}
}

func (e *GovernedExecutor) ExecuteCycle(ctx context.Context, inputJSON string) (string, error) {
	if e.llmProvider == nil {
		return "", errors.New("llm provider not configured")
	}

	llmResponse, err := e.llmProvider.Call(ctx, inputJSON)
	if err != nil {
		return "", fmt.Errorf("llm invocation: %w", err)
	}

	if err := e.validateOutputSchema(llmResponse); err != nil {
		fmt.Printf("[Warning] AgentOutput schema validation failed: %v. Triggering repair loop...\n", err)
		return e.triggerRepairLoop(ctx, inputJSON, llmResponse, err.Error())
	}

	return llmResponse, nil
}

func (e *GovernedExecutor) validateOutputSchema(response string) error {
	documentLoader := gojsonschema.NewStringLoader(response)
	result, err := gojsonschema.Validate(e.outputSchemaLoader, documentLoader)
	if err != nil {
		return err
	}

	if !result.Valid() {
		var errMsgs string
		for _, desc := range result.Errors() {
			errMsgs += fmt.Sprintf("- %s\n", desc.String())
		}
		return fmt.Errorf("schema violation errors:\n%s", errMsgs)
	}
	return nil
}

func (e *GovernedExecutor) triggerRepairLoop(ctx context.Context, originalInput string, failedOutput string, errorMsg string) (string, error) {
	repairPrompt := fmt.Sprintf(
		"%s\n\n[SYSTEM DIRECTIVE: AUTO-REPAIR]\nYour previous output violated the AgentOutput JSON schema.\n"+
			"FAILED_OUTPUT:\n%s\n\nERROR_MESSAGE:\n%s\n\nPlease fix the JSON format and ensure all fields are present.",
		originalInput, failedOutput, errorMsg,
	)

	repairedResponse, err := e.llmProvider.Call(ctx, repairPrompt)
	if err != nil {
		return "", fmt.Errorf("repair llm invocation: %w", err)
	}

	if err := e.validateOutputSchema(repairedResponse); err != nil {
		return "", fmt.Errorf("repair failed again, aborting cycle: %w", err)
	}

	return repairedResponse, nil
}
