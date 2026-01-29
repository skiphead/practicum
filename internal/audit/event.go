package audit

// Event represents an audit event for tracking user actions in the system.
// Audit events are used for monitoring, security, and analytics purposes.
type Event struct {
	Timestamp int64  `json:"ts"`      // Unix timestamp of the event (seconds since epoch)
	Action    string `json:"action"`  // Action type: "shorten" or "follow"
	UserID    string `json:"user_id"` // User identifier (empty if user is not authenticated)
	URL       string `json:"url"`     // Original URL involved in the action
}
