package rtagent

import "strings"

// deriveContextMessageBudget computes a message-count window from the model
// provider's declared context window. It returns 0 (no trimming) when the
// provider does not declare capabilities or declares an unknown window.
//
// Heuristic: reserve ~25% of the window for system prompt, tool schemas, and
// output, and assume an average of ~500 tokens per conversation message
// (covering assistant text, tool calls, and tool observations). This is a
// conservative default; hosts wanting precise control should set
// RuntimeConfig.MaxContextMessages explicitly, which overrides this derivation.
func deriveContextMessageBudget(provider ModelProvider) int {
	if provider == nil {
		return 0
	}
	capsProvider, ok := provider.(ModelCapabilityProvider)
	if !ok {
		return 0
	}
	caps := capsProvider.Capabilities()
	if caps.ContextWindowTokens <= 0 {
		return 0
	}
	const (
		reservedFraction    = 0.25 // 25% for system/tools/output
		avgTokensPerMessage = 500
		minDerivedBudget    = 4
	)
	usable := float64(caps.ContextWindowTokens) * (1.0 - reservedFraction)
	budget := int(usable / float64(avgTokensPerMessage))
	if budget < minDerivedBudget {
		return minDerivedBudget
	}
	return budget
}

// trimMessagesToWindow bounds a conversation message slice to at most max
// entries while preserving task context. It is a pure function with no
// dependencies on Runtime state, making it cheap to test exhaustively.
//
// Policy:
//   - If max <= 0, trimming is disabled; the input is returned unchanged.
//   - If len(messages) <= max, nothing exceeds the window; returned unchanged.
//   - Otherwise the result is: the first role:"user" message (the task context
//     — losing it would make the model forget the objective), followed by the
//     most recent (max-1) messages. Middle messages are dropped.
//
// The first-user-message search skips leading non-user messages (e.g. a system
// message) but anchors on the earliest user message, which is the original task
// instruction. If no user message exists, the tail-only window is used.
//
// The returned slice is always a fresh allocation when trimming occurs, so the
// caller can safely mutate it without aliasing the input.
func trimMessagesToWindow(messages []ModelMessage, max int) []ModelMessage {
	if max <= 0 || len(messages) <= max {
		return messages
	}

	firstUserIdx := -1
	for i, msg := range messages {
		if strings.TrimSpace(msg.Role) == "user" {
			firstUserIdx = i
			break
		}
	}

	tailCount := max
	result := make([]ModelMessage, 0, max+1)

	if firstUserIdx >= 0 && firstUserIdx < len(messages)-tailCount {
		// Preserve the first user message as task context, then take the tail.
		result = append(result, messages[firstUserIdx])
		tailCount = max - 1
		if tailCount < 1 {
			tailCount = 1
		}
	}

	start := len(messages) - tailCount
	if start < 0 {
		start = 0
	}
	// Avoid duplicating the first user message if it falls inside the tail.
	if firstUserIdx >= 0 && start <= firstUserIdx && len(result) > 0 {
		start = firstUserIdx + 1
	}
	result = append(result, messages[start:]...)
	return result
}

// trimMessagesIfConfigured applies trimMessagesToWindow using the Runtime's
// configured MaxContextMessages. It returns the (possibly trimmed) messages and
// the number of messages dropped, so callers can emit an observability event.
// When trimming is disabled (max <= 0), it is a no-op returning 0 dropped.
func (r *Runtime) trimMessagesIfConfigured(messages []ModelMessage) ([]ModelMessage, int) {
	if r.maxContextMessages <= 0 {
		return messages, 0
	}
	before := len(messages)
	trimmed := trimMessagesToWindow(messages, r.maxContextMessages)
	return trimmed, before - len(trimmed)
}
