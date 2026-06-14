package rtagent

import "strings"

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
