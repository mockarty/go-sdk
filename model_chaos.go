// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

package mockarty

import "time"

// ---------------------------------------------------------------------------
// Experiment Status & Fault/Target Enumerations
// ---------------------------------------------------------------------------

// ExperimentStatus represents the current state of a chaos experiment.
type ExperimentStatus string

const (
	ExperimentStatusPending   ExperimentStatus = "pending"
	ExperimentStatusQueued    ExperimentStatus = "queued"
	ExperimentStatusWarmup    ExperimentStatus = "warmup"
	ExperimentStatusActive    ExperimentStatus = "active"
	ExperimentStatusCooldown  ExperimentStatus = "cooldown"
	ExperimentStatusCompleted ExperimentStatus = "completed"
	ExperimentStatusAborted   ExperimentStatus = "aborted"
	ExperimentStatusFailed    ExperimentStatus = "failed"
)

// FaultType describes the kind of fault to inject.
type FaultType string

const (
	FaultPodKill           FaultType = "pod_kill"
	FaultPodKillRandom     FaultType = "pod_kill_random"
	FaultNetworkPartition  FaultType = "network_partition"
	FaultNetworkDelay      FaultType = "network_delay"
	FaultNetworkLoss       FaultType = "network_loss"
	FaultDeploymentRestart FaultType = "deployment_restart"
	FaultScale             FaultType = "scale"
	FaultNodeDrain         FaultType = "node_drain"
	FaultResourceStress    FaultType = "resource_stress"
	FaultDNSDisruption     FaultType = "dns_disruption"
	FaultIOChaos           FaultType = "io_chaos"
	FaultTimeChaos         FaultType = "time_chaos"
)

// TargetMode defines how targets are selected for fault injection.
type TargetMode string

const (
	TargetModeOne          TargetMode = "one"
	TargetModeRandomOne    TargetMode = "random_one"
	TargetModeAll          TargetMode = "all"
	TargetModeFixedPercent TargetMode = "fixed_percent"
)

// ---------------------------------------------------------------------------
// Core Experiment Model
// ---------------------------------------------------------------------------

// ChaosExperiment is the top-level entity describing a chaos engineering experiment.
// Fields and JSON tags match the server model in internal/chaos/models.go exactly.
type ChaosExperiment struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Namespace   string           `json:"namespace"`
	Status      ExperimentStatus `json:"status"`
	PresetName  string           `json:"presetName,omitempty"`

	Faults      []FaultConfig  `json:"faults"`
	Target      TargetConfig   `json:"target"`
	SteadyState *SteadyState   `json:"steadyState,omitempty"`
	Schedule    ScheduleConfig `json:"schedule"`
	Safety      SafetyConfig   `json:"safety"`

	Results *ChaosResults `json:"results,omitempty"`

	DurationSec int    `json:"durationSec"`
	WarmupSec   int    `json:"warmupSec,omitempty"`
	CooldownSec int    `json:"cooldownSec,omitempty"`
	CreatedBy   string `json:"createdBy,omitempty"`

	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	StartedAt *time.Time `json:"startedAt,omitempty"`
	EndedAt   *time.Time `json:"endedAt,omitempty"`
}

// ---------------------------------------------------------------------------
// Fault Configuration
// ---------------------------------------------------------------------------

// FaultConfig describes a single fault injection action within an experiment.
type FaultConfig struct {
	Type       FaultType              `json:"type"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`

	// Fields used by specific fault types
	GracePeriodSec int    `json:"gracePeriodSec,omitempty"`
	Replicas       *int   `json:"replicas,omitempty"`
	Duration       string `json:"duration,omitempty"`
	IntervalSec    int    `json:"intervalSec,omitempty"`

	// Network-specific
	LatencyMs      int `json:"latencyMs,omitempty"`
	LossPercent    int `json:"lossPercent,omitempty"`
	JitterMs       int `json:"jitterMs,omitempty"`
	CorruptPercent int `json:"corruptPercent,omitempty"`

	// Resource stress-specific
	CPUCores   int    `json:"cpuCores,omitempty"`
	MemoryMB   int    `json:"memoryMB,omitempty"`
	StressType string `json:"stressType,omitempty"` // cpu, memory, io, all

	// DNS-specific
	TargetDomain string `json:"targetDomain,omitempty"`
	SpoofIP      string `json:"spoofIP,omitempty"`

	// IO chaos-specific
	IOLatencyMs  int    `json:"ioLatencyMs,omitempty"`
	IOErrPercent int    `json:"ioErrPercent,omitempty"`
	IOPath       string `json:"ioPath,omitempty"`

	// Time chaos-specific
	TimeOffsetSec int `json:"timeOffsetSec,omitempty"`
}

// ---------------------------------------------------------------------------
// Target Configuration
// ---------------------------------------------------------------------------

// TargetConfig specifies which resources are targeted by the experiment.
type TargetConfig struct {
	Mode       TargetMode        `json:"mode"`
	Selector   map[string]string `json:"selector,omitempty"`   // Label selector key=value pairs
	Deployment string            `json:"deployment,omitempty"` // Target deployment name
	Namespace  string            `json:"namespace,omitempty"`  // Target namespace (if different from experiment)
	PodNames   []string          `json:"podNames,omitempty"`   // Specific pod names
	NodeName   string            `json:"nodeName,omitempty"`   // Specific node name
	Percentage int               `json:"percentage,omitempty"` // For fixed_percent mode (1-100)
}

// ---------------------------------------------------------------------------
// Steady State
// ---------------------------------------------------------------------------

// SteadyState defines expected baseline conditions and the checks used to verify them.
type SteadyState struct {
	Checks []SteadyStateCheck `json:"checks"`
}

// SteadyStateCheck is a single verification of a steady-state hypothesis.
type SteadyStateCheck struct {
	Name        string            `json:"name"`
	Type        string            `json:"type"`                  // http, prometheus, pod_count, custom
	Endpoint    string            `json:"endpoint,omitempty"`    // URL to check
	Method      string            `json:"method,omitempty"`      // HTTP method
	Expected    interface{}       `json:"expected,omitempty"`    // Expected value or status code
	Tolerance   float64           `json:"tolerance,omitempty"`   // Acceptable deviation (e.g. 0.05 = 5%)
	TimeoutSec  int               `json:"timeoutSec,omitempty"`  // Check timeout
	IntervalSec int               `json:"intervalSec,omitempty"` // Check interval during experiment
	Query       string            `json:"query,omitempty"`       // PromQL query or JSONPath expression
	Headers     map[string]string `json:"headers,omitempty"`
}

// ---------------------------------------------------------------------------
// Schedule Configuration
// ---------------------------------------------------------------------------

// ScheduleConfig controls the scheduling behavior of the experiment.
type ScheduleConfig struct {
	CronExpr    string `json:"cronExpr,omitempty"`    // Cron expression for recurring experiments
	RepeatCount int    `json:"repeatCount,omitempty"` // Number of repetitions (0 = infinite if cron)
	Jitter      int    `json:"jitter,omitempty"`      // Random jitter in seconds added to start time
}

// ---------------------------------------------------------------------------
// Safety Configuration
// ---------------------------------------------------------------------------

// SafetyConfig defines guardrails to prevent chaos from causing unacceptable damage.
type SafetyConfig struct {
	DenyNamespaces     []string `json:"denyNamespaces,omitempty"`     // Namespaces that must never be targeted
	AllowNamespaces    []string `json:"allowNamespaces,omitempty"`    // Only allow these namespaces (if set, deny is ignored)
	MaxConcurrent      int      `json:"maxConcurrent,omitempty"`      // Max concurrent experiments
	MaxPodsAffected    int      `json:"maxPodsAffected,omitempty"`    // Max pods that can be killed/affected at once
	MinReplicasAlive   int      `json:"minReplicasAlive,omitempty"`   // Minimum replicas that must remain alive
	AutoRollback       bool     `json:"autoRollback,omitempty"`       // Automatically rollback on steady-state violation
	HaltOnSteadyFail   bool     `json:"haltOnSteadyFail,omitempty"`   // Halt experiment if steady-state check fails
	RequireApproval    bool     `json:"requireApproval,omitempty"`    // Require manual approval before execution
	BlastRadiusPercent int      `json:"blastRadiusPercent,omitempty"` // Max % of pods that can be affected (0 = no limit)
}

// ---------------------------------------------------------------------------
// Results
// ---------------------------------------------------------------------------

// ChaosResults holds aggregated results from a completed experiment.
type ChaosResults struct {
	SteadyStateBefore bool               `json:"steadyStateBefore"`
	SteadyStateAfter  bool               `json:"steadyStateAfter"`
	SteadyStateDuring bool               `json:"steadyStateDuring"`
	Phases            []PhaseMetrics     `json:"phases,omitempty"`
	Timeline          []TimelineEvent    `json:"timeline,omitempty"`
	AffectedResources []AffectedResource `json:"affectedResources,omitempty"`
	TotalDurationMs   int64              `json:"totalDurationMs"`
	ErrorCount        int                `json:"errorCount"`
	RecoveryTimeMs    int64              `json:"recoveryTimeMs,omitempty"`
	ResilienceScore   int                `json:"resilienceScore"`
	Summary           string             `json:"summary,omitempty"`
}

// PhaseMetrics contains metrics collected during a specific phase of the experiment.
type PhaseMetrics struct {
	Phase       string    `json:"phase"` // warmup, active, cooldown
	StartTime   time.Time `json:"startTime"`
	EndTime     time.Time `json:"endTime"`
	Latency     MinMaxAvg `json:"latency,omitempty"`
	ErrorRate   float64   `json:"errorRate"`
	Throughput  float64   `json:"throughput,omitempty"` // Requests per second
	PodRestarts int       `json:"podRestarts,omitempty"`
}

// MinMaxAvg holds min/max/avg statistics for a numeric metric.
type MinMaxAvg struct {
	Min float64 `json:"min"`
	Max float64 `json:"max"`
	Avg float64 `json:"avg"`
}

// TimelineEvent represents a discrete event that occurred during the experiment.
type TimelineEvent struct {
	Timestamp time.Time   `json:"timestamp"`
	Type      string      `json:"type"` // fault_injected, fault_removed, pod_killed, pod_restarted, etc.
	Message   string      `json:"message"`
	Details   interface{} `json:"details,omitempty"`
	Severity  string      `json:"severity,omitempty"` // info, warning, error
}

// AffectedResource describes a resource that was impacted by the experiment.
type AffectedResource struct {
	Kind      string `json:"kind"` // Pod, Deployment, Node, NetworkPolicy
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Action    string `json:"action"` // killed, restarted, scaled, drained, partitioned
	Recovered bool   `json:"recovered"`
}

// ---------------------------------------------------------------------------
// Profiles (Cluster Connections) — maps to InfraProfile on the server
// ---------------------------------------------------------------------------

// ChaosProfile represents a Kubernetes cluster connection profile
// (InfraProfile on the server side).
type ChaosProfile struct {
	ID             string `json:"id"`
	NamespaceID    string `json:"namespaceId"`
	Name           string `json:"name"`
	KubeconfigPath string `json:"kubeconfigPath,omitempty"`
	KubeconfigData string `json:"kubeconfigData,omitempty"` // Base64-encoded kubeconfig content
	Context        string `json:"context,omitempty"`        // kubectl context name
	InCluster      bool   `json:"inCluster,omitempty"`      // Use in-cluster config
	DefaultNS      string `json:"defaultNamespace,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// ChaosConnectionResult holds the result of a cluster connectivity test.
type ChaosConnectionResult struct {
	Connected    bool        `json:"connected"`
	Error        string      `json:"error,omitempty"`
	ProfileID    string      `json:"profileId,omitempty"`
	ProfileName  string      `json:"profileName,omitempty"`
	Capabilities interface{} `json:"capabilities,omitempty"`
	Warning      string      `json:"warning,omitempty"`
	Message      string      `json:"message,omitempty"`
}

// ---------------------------------------------------------------------------
// Presets
// ---------------------------------------------------------------------------

// ChaosPreset represents a predefined chaos experiment template (PresetInfo on the server).
type ChaosPreset struct {
	Name        string   `json:"name"`
	DisplayName string   `json:"displayName"`
	Description string   `json:"description,omitempty"`
	FaultTypes  []string `json:"faultTypes,omitempty"`
	RiskLevel   string   `json:"riskLevel,omitempty"` // low, medium, high, critical
}

// ---------------------------------------------------------------------------
// Operator
// ---------------------------------------------------------------------------

// ChaosOperatorStatus holds the status of the chaos operator in a cluster.
type ChaosOperatorStatus struct {
	Installed     bool     `json:"installed"`
	Healthy       bool     `json:"healthy"`
	Replicas      int      `json:"replicas,omitempty"`
	ReadyReplicas int      `json:"readyReplicas,omitempty"`
	Namespace     string   `json:"namespace,omitempty"`
	Message       string   `json:"message,omitempty"`
	SetupSteps    []string `json:"setupSteps,omitempty"`
}

// ---------------------------------------------------------------------------
// Metrics & Events
// ---------------------------------------------------------------------------

// MetricsSnapshot captures a point-in-time snapshot of cluster/service metrics.
type MetricsSnapshot struct {
	Timestamp     time.Time          `json:"timestamp"`
	PodCount      int                `json:"podCount"`
	ReadyPodCount int                `json:"readyPodCount"`
	RestartCount  int                `json:"restartCount"`
	LatencyP50Ms  float64            `json:"latencyP50Ms,omitempty"`
	LatencyP95Ms  float64            `json:"latencyP95Ms,omitempty"`
	LatencyP99Ms  float64            `json:"latencyP99Ms,omitempty"`
	ErrorRate     float64            `json:"errorRate,omitempty"`
	CustomMetrics map[string]float64 `json:"customMetrics,omitempty"`
}

// HealthCheckResult represents the outcome of a single health check execution.
type HealthCheckResult struct {
	CheckName     string      `json:"checkName"`
	Passed        bool        `json:"passed"`
	ActualValue   interface{} `json:"actualValue,omitempty"`
	ExpectedValue interface{} `json:"expectedValue,omitempty"`
	Message       string      `json:"message,omitempty"`
	Timestamp     time.Time   `json:"timestamp"`
	DurationMs    int64       `json:"durationMs"`
}

// ---------------------------------------------------------------------------
// List Response Wrappers (match server JSON envelope shapes)
// ---------------------------------------------------------------------------

// chaosExperimentListResponse is the envelope returned by GET /api/v1/chaos/experiments.
type chaosExperimentListResponse struct {
	Experiments []ChaosExperiment `json:"experiments"`
	Total       int               `json:"total"`
	Limit       int               `json:"limit"`
	Offset      int               `json:"offset"`
}

// chaosPresetListResponse is the envelope returned by GET /api/v1/chaos/presets.
type chaosPresetListResponse struct {
	Presets []ChaosPreset `json:"presets"`
	Count   int           `json:"count"`
}

// chaosProfileListResponse is the envelope returned by GET /api/v1/chaos/profiles.
type chaosProfileListResponse struct {
	Profiles []ChaosProfile `json:"profiles"`
	Count    int            `json:"count"`
}

// chaosMetricsResponse is the envelope returned by GET /api/v1/chaos/experiments/:id/metrics.
type chaosMetricsResponse struct {
	ExperimentID string            `json:"experimentId"`
	Snapshots    []MetricsSnapshot `json:"snapshots"`
	Count        int               `json:"count"`
}

// chaosEventsResponse is the envelope returned by GET /api/v1/chaos/experiments/:id/events.
type chaosEventsResponse struct {
	ExperimentID string          `json:"experimentId"`
	Events       []TimelineEvent `json:"events"`
	Count        int             `json:"count"`
}

// ---------------------------------------------------------------------------
// List Options
// ---------------------------------------------------------------------------

// ChaosListOptions specifies filters for listing experiments.
type ChaosListOptions struct {
	Namespace string
	Status    string
	Limit     int
	Offset    int
}
