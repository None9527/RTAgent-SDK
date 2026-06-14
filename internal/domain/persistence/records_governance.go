package persistence

type CapabilityRecord struct {
	CapabilityID string `json:"capability_id,omitempty"`
	Subject      string `json:"subject,omitempty"`
	Scope        string `json:"scope,omitempty"`
	ExpiresAt    string `json:"expires_at,omitempty"`
	Authority    string `json:"authority,omitempty"`
	Policy       string `json:"policy,omitempty"`
}

type ToolSchemaSnapshotRecord struct {
	SnapshotID      string `json:"snapshot_id,omitempty"`
	RunID           string `json:"run_id,omitempty"`
	ContextPacketID string `json:"context_packet_id,omitempty"`
	SchemaHash      string `json:"schema_hash,omitempty"`
	ToolCount       int    `json:"tool_count,omitempty"`
	SnapshotJSON    string `json:"snapshot_json,omitempty"`
	CreatedAt       string `json:"created_at,omitempty"`
}

type PermissionRecord struct {
	PermissionID   string `json:"permission_id,omitempty"`
	RunID          string `json:"run_id,omitempty"`
	Subject        string `json:"subject,omitempty"`
	Scope          string `json:"scope,omitempty"`
	Granted        bool   `json:"granted,omitempty"`
	AuthorizedBy   string `json:"authorized_by,omitempty"`
	RequestedAt    string `json:"requested_at,omitempty"`
	ResolvedAt     string `json:"resolved_at,omitempty"`
	PolicyWarnings string `json:"policy_warnings,omitempty"`
}

type GrantRecord struct {
	GrantID      string `json:"grant_id,omitempty"`
	CapabilityID string `json:"capability_id,omitempty"`
	Grantee      string `json:"grantee,omitempty"`
	GrantedBy    string `json:"granted_by,omitempty"`
	GrantedAt    string `json:"granted_at,omitempty"`
	ExpiresAt    string `json:"expires_at,omitempty"`
}

type LeaseRecord struct {
	LeaseID          string `json:"lease_id,omitempty"`
	Resource         string `json:"resource,omitempty"`
	HolderActivityID string `json:"holder_activity_id,omitempty"`
	AcquiredAt       string `json:"acquired_at,omitempty"`
	ExpiresAt        string `json:"expires_at,omitempty"`
	ReleasedAt       string `json:"released_at,omitempty"`
}
