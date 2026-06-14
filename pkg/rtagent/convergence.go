package rtagent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

// Convergence thresholds. These bound loop behavior so a run converges or
// terminates gracefully instead of spinning until the hard iteration limit.
// They are conservative defaults ported from production agent runtimes; hosts
// do not need to tune them for typical use.
const (
	// repeatedToolInteractionThreshold is the number of times the same
	// tool-call + observation signature must repeat in a single run before the
	// controller forces a replan steering message.
	repeatedToolInteractionThreshold = 3
	// softNoProgressIterationFloor is the earliest iteration at which the
	// no-progress streak can trigger a replan. Below this the loop is allowed
	// to explore freely.
	softNoProgressIterationFloor = 12
	// noProgressFinalizationThreshold is the number of consecutive tool turns
	// that produce no novel observation signature required to trigger replan.
	noProgressFinalizationThreshold = 3
)

// convergenceDecision is the outcome of observing one tool turn. At most one
// of ShouldFinalize / ShouldReplan is set; both can be false (continue normally).
type convergenceDecision struct {
	ShouldFinalize bool
	ShouldReplan   bool
	Reason         string
	Detail         string
}

// finalizing marks that the next model request must produce a final answer
// (tools stripped). replan injects a one-shot steering message but keeps tools.
func (d convergenceDecision) isActive() bool {
	return d.ShouldFinalize || d.ShouldReplan
}

// convergenceController tracks tool-interaction signatures across a run to
// detect repetition and lack of progress. It is created fresh per run (not per
// Runtime instance) and is not safe to share across runs.
type convergenceController struct {
	signatures       map[string]int // signature -> times seen
	noProgressStreak int            // consecutive turns with no novel signature
	observations     int            // total observations observed
	replans          map[string]int // reason -> times emitted (dedupe)
}

func newConvergenceController() *convergenceController {
	return &convergenceController{
		signatures: make(map[string]int),
		replans:    make(map[string]int),
	}
}

// observe evaluates one tool turn (the calls the model made and the observations
// those calls produced) and returns a convergence decision.
//
// iteration is the current 1-based iteration (matches state.Iteration+1 at the
// call site, since observe runs after tool execution but before the increment).
// hardIterationLimit is the maxToolIterations budget; the controller fires a
// pre-flush finalize one iteration before the limit so the finalization turn
// still fits inside the budget.
func (c *convergenceController) observe(iteration, hardIterationLimit int, calls []ToolCall, observations []ToolObservation) convergenceDecision {
	c.observations += len(observations)

	maxRepeat := 0
	novel := false
	for idx, call := range calls {
		var observation ToolObservation
		if idx < len(observations) {
			observation = observations[idx]
		}
		sig := convergenceToolSignature(call, observation)
		if c.signatures[sig] == 0 {
			novel = true
		}
		c.signatures[sig]++
		if c.signatures[sig] > maxRepeat {
			maxRepeat = c.signatures[sig]
		}
	}

	if len(calls) == 0 || !novel {
		c.noProgressStreak++
	} else {
		c.noProgressStreak = 0
	}

	// Hard-budget pre-flush: finalize one iteration before the limit so the
	// finalization turn (tools stripped) still runs inside the budget. This is
	// checked first so it overrides replan when the budget is about to run out.
	if hardIterationLimit > 0 && iteration >= hardIterationLimit-1 {
		return convergenceDecision{
			ShouldFinalize: true,
			Reason:         "hard_budget_preflush",
			Detail:         fmt.Sprintf("reached iteration %d of hard budget %d; finalizing before the limit", iteration, hardIterationLimit),
		}
	}

	if maxRepeat >= repeatedToolInteractionThreshold {
		return c.replanDecision("repeated_tool_interaction", fmt.Sprintf("same tool interaction repeated %d times", maxRepeat))
	}

	if iteration >= softNoProgressIterationFloor && c.noProgressStreak >= noProgressFinalizationThreshold && c.observations > 0 {
		return c.replanDecision("no_new_observation", fmt.Sprintf("%d consecutive tool turns added no new observation signature", c.noProgressStreak))
	}

	return convergenceDecision{}
}

// replanDecision emits a replan for a reason at most once. Subsequent triggers
// of the same reason return an empty decision (no-op), so the loop falls
// through to the hard-budget pre-flush if the model keeps stalling.
func (c *convergenceController) replanDecision(reason, detail string) convergenceDecision {
	c.replans[reason]++
	if c.replans[reason] > 1 {
		return convergenceDecision{}
	}
	return convergenceDecision{ShouldReplan: true, Reason: reason, Detail: detail}
}

// convergenceToolSignature produces a stable hash of a tool call combined with
// its observation, so two identical calls returning different results are
// treated as novel (not repetitive). The signature covers name + arguments +
// observation status + summary.
func convergenceToolSignature(call ToolCall, observation ToolObservation) string {
	payload := map[string]any{
		"name":      call.Name,
		"arguments": call.Arguments,
	}
	if observation.ToolCallID != "" {
		payload["status"] = observation.Status
		summary := strings.TrimSpace(observation.ModelVisibleSummary)
		if summary == "" {
			summary = strings.TrimSpace(observation.UserVisibleSummary)
		}
		payload["summary"] = summary
	}
	data, _ := json.Marshal(payload)
	sum := sha256.Sum256(data)
	return "convergence-tool-" + hex.EncodeToString(sum[:])
}

// convergenceFinalizationMessage is the steering user message injected when the
// controller decides the loop should produce a final answer. It instructs the
// model to stop calling tools and answer from the observations already gathered.
func convergenceFinalizationMessage(decision convergenceDecision) string {
	detail := strings.TrimSpace(decision.Detail)
	if detail == "" {
		detail = "the runtime reached a convergence checkpoint"
	}
	return "Runtime convergence checkpoint: " + detail + ". Stop calling tools now. Based only on the observations already available, return the user's final answer. Include confirmed facts, actions completed, and remaining unknowns when relevant. Do not mention this checkpoint unless it is necessary to explain partial coverage."
}

// convergenceReplanMessage is the steering user message injected when the
// controller detects low-yield exploration. Tools remain available; the model
// is asked to compress evidence and make one precise non-repetitive observation.
func convergenceReplanMessage(decision convergenceDecision) string {
	detail := strings.TrimSpace(decision.Detail)
	if detail == "" {
		detail = "the runtime observed low-yield exploration"
	}
	return "Runtime observation checkpoint: " + detail + ". Keep the current task context and do not restart. Compress the confirmed evidence into a short working summary, name the single most important missing fact if one still blocks the answer, then either answer now or make one precise non-repetitive observation. Tools remain available when a specific missing fact blocks the answer."
}

// convergenceToolSpecs returns the tool specs to pass to the model. In
// finalization mode tools are stripped (nil) so the model must produce a text
// answer. Otherwise the full tool spec set is forwarded.
func convergenceToolSpecs(finalizing bool, specs []ToolSpec) []ToolSpec {
	if finalizing {
		return nil
	}
	return append([]ToolSpec(nil), specs...)
}
