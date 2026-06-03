// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import "encoding/json"

// Protocol represents the communication protocol of a mock.
type Protocol string

const (
	ProtocolHTTP     Protocol = "http"
	ProtocolGRPC     Protocol = "grpc"
	ProtocolMCP      Protocol = "mcp"
	ProtocolSocket   Protocol = "socket"
	ProtocolSOAP     Protocol = "soap"
	ProtocolGraphQL  Protocol = "graphql"
	ProtocolSSE      Protocol = "sse"
	ProtocolKafka    Protocol = "kafka"
	ProtocolRabbitMQ Protocol = "rabbitmq"
	ProtocolSMTP     Protocol = "smtp"
)

// AssertAction defines how a condition value is checked.
type AssertAction string

const (
	AssertEquals      AssertAction = "equals"
	AssertContains    AssertAction = "contains"
	AssertNotEquals   AssertAction = "not_equals"
	AssertNotContains AssertAction = "not_contains"
	AssertAny         AssertAction = "any"
	AssertNotEmpty    AssertAction = "notEmpty"
	AssertEmpty       AssertAction = "empty"
	AssertMatches     AssertAction = "matches"
)

// Decode represents a decoding strategy for payloads.
type Decode string

const (
	DecodeBase64 Decode = "base64"
)

// OneOfOrder represents the ordering strategy for OneOf responses.
type OneOfOrder string

const (
	OneOfOrderSequential OneOfOrder = "order"
	OneOfOrderRandom     OneOfOrder = "random"
)

// CallbackType defines the type of callback transport.
type CallbackType string

const (
	CallbackTypeHTTP     CallbackType = "http"
	CallbackTypeKafka    CallbackType = "kafka"
	CallbackTypeRabbitMQ CallbackType = "rabbitmq"
)

// CallbackTrigger defines when a callback fires.
type CallbackTrigger string

const (
	TriggerOnSuccess CallbackTrigger = "on_success"
	TriggerOnError   CallbackTrigger = "on_error"
	TriggerAlways    CallbackTrigger = "always"
)

// ---------------------------------------------------------------------------
// Condition
// ---------------------------------------------------------------------------

// Condition represents a single match condition for requests.
type Condition struct {
	Path           string       `json:"path,omitempty"`
	AssertAction   AssertAction `json:"assertAction,omitempty"`
	Decode         Decode       `json:"decode,omitempty"`
	ApplySortArray bool         `json:"sortArray,omitempty"`
	Value          any          `json:"value,omitempty"`
	ValueFromFile  string       `json:"valueFromFile,omitempty"`
}

// ---------------------------------------------------------------------------
// Request context types (per protocol)
// ---------------------------------------------------------------------------

// HttpRequestContext defines matching rules for HTTP requests.
type HttpRequestContext struct {
	Route          string       `json:"route,omitempty"`
	RoutePattern   string       `json:"routePattern,omitempty"`
	HttpMethod     string       `json:"httpMethod,omitempty"`
	Conditions     []*Condition `json:"conditions,omitempty"`
	QueryParams    []*Condition `json:"queryParams,omitempty"`
	Headers        []*Condition `json:"header,omitempty"`
	ApplySortArray bool         `json:"sortArray,omitempty"`
}

// GrpcRequestContext defines matching rules for gRPC requests.
type GrpcRequestContext struct {
	Conditions     []*Condition `json:"conditions,omitempty"`
	Meta           []*Condition `json:"meta,omitempty"`
	Service        string       `json:"service,omitempty"`
	Method         string       `json:"method,omitempty"`
	MethodType     string       `json:"methodType,omitempty"` // unary | server-stream | client-stream | bidirectional
	ApplySortArray bool         `json:"sortArray,omitempty"`
}

// MCPRequestContext defines matching rules for MCP requests.
type MCPRequestContext struct {
	Conditions     []*Condition `json:"conditions,omitempty"`
	Headers        []*Condition `json:"header,omitempty"`
	Method         string       `json:"method,omitempty"`
	Tool           string       `json:"tool,omitempty"`
	Resource       string       `json:"resource,omitempty"`
	Description    string       `json:"description,omitempty"`
	ApplySortArray bool         `json:"sortArray,omitempty"`
}

// SocketRequestContext defines matching rules for Socket requests.
type SocketRequestContext struct {
	Conditions     []*Condition `json:"conditions,omitempty"`
	ServerName     string       `json:"serverName,omitempty"`
	Event          string       `json:"event,omitempty"`
	Namespace      string       `json:"namespace,omitempty"`
	ApplySortArray bool         `json:"sortArray,omitempty"`
}

// SoapRequestContext defines matching rules for SOAP requests.
type SoapRequestContext struct {
	Conditions     []*Condition `json:"conditions,omitempty"`
	Headers        []*Condition `json:"header,omitempty"`
	Service        string       `json:"service,omitempty"`
	Method         string       `json:"method,omitempty"`
	Action         string       `json:"action,omitempty"`
	Path           string       `json:"path,omitempty"`
	ApplySortArray bool         `json:"sortArray,omitempty"`
}

// GraphQLRequestContext defines matching rules for GraphQL requests.
type GraphQLRequestContext struct {
	Conditions     []*Condition `json:"conditions,omitempty"`
	Headers        []*Condition `json:"header,omitempty"`
	Operation      string       `json:"operation,omitempty"` // query, mutation, subscription
	Field          string       `json:"field,omitempty"`
	Type           string       `json:"type,omitempty"`
	Path           string       `json:"path,omitempty"`
	ApplySortArray bool         `json:"sortArray,omitempty"`
}

// SSERequestContext defines matching rules for SSE requests.
type SSERequestContext struct {
	Conditions       []*Condition `json:"conditions,omitempty"`
	HeaderConditions []*Condition `json:"headerConditions,omitempty"`
	EventPath        string       `json:"eventPath,omitempty"`
	EventName        string       `json:"eventName,omitempty"`
	Description      string       `json:"description,omitempty"`
	ApplySortArray   bool         `json:"sortArray,omitempty"`
}

// KafkaRequestContext defines matching rules for Kafka messages.
type KafkaRequestContext struct {
	Conditions     []*Condition      `json:"conditions,omitempty"`
	Headers        []*Condition      `json:"headers,omitempty"`
	Topic          string            `json:"topic,omitempty"`
	ServerName     string            `json:"serverName,omitempty"`
	ConsumerGroup  string            `json:"consumerGroup,omitempty"`
	ApplySortArray bool              `json:"sortArray,omitempty"`
	OutputTopic    string            `json:"outputTopic,omitempty"`
	OutputBrokers  string            `json:"outputBrokers,omitempty"`
	OutputKey      string            `json:"outputKey,omitempty"`
	OutputHeaders  map[string]string `json:"outputHeaders,omitempty"`
}

// RabbitMQRequestContext defines matching rules for RabbitMQ messages.
type RabbitMQRequestContext struct {
	Conditions       []*Condition         `json:"conditions,omitempty"`
	Headers          []*Condition         `json:"headers,omitempty"`
	Queue            string               `json:"queue,omitempty"`
	Exchange         string               `json:"exchange,omitempty"`
	RoutingKey       string               `json:"routingKey,omitempty"`
	ServerName       string               `json:"serverName,omitempty"`
	ApplySortArray   bool                 `json:"sortArray,omitempty"`
	OutputURL        string               `json:"outputURL,omitempty"`
	OutputExchange   string               `json:"outputExchange,omitempty"`
	OutputRoutingKey string               `json:"outputRoutingKey,omitempty"`
	OutputQueue      string               `json:"outputQueue,omitempty"`
	OutputProps      *RabbitMQOutputProps `json:"outputProps,omitempty"`
}

// SmtpRequestContext defines matching rules for SMTP messages.
type SmtpRequestContext struct {
	ServerName          string       `json:"serverName,omitempty"`
	SenderConditions    []*Condition `json:"senderConditions,omitempty"`
	RecipientConditions []*Condition `json:"recipientConditions,omitempty"`
	SubjectConditions   []*Condition `json:"subjectConditions,omitempty"`
	BodyConditions      []*Condition `json:"bodyConditions,omitempty"`
	HeaderConditions    []*Condition `json:"headerConditions,omitempty"`
	ApplySortArray      bool         `json:"sortArray,omitempty"`
}

// S3RequestContext defines matching + declarative response rules for an S3
// (object-storage) mock. Mirrors the server's mock_data.s3 shape: top-level
// bucket/keyPrefix/endpoint selectors, optional condition/header/toxicity
// matchers, an ordered rule list, and dispatch defaults.
type S3RequestContext struct {
	Conditions     []*Condition      `json:"conditions,omitempty"`
	Headers        []*Condition      `json:"header,omitempty"`
	Toxicity       []ToxicityRule    `json:"toxicity,omitempty"`
	Rules          []S3OperationRule `json:"rules,omitempty"`
	Bucket         string            `json:"bucket,omitempty"`
	KeyPrefix      string            `json:"keyPrefix,omitempty"`
	Endpoint       string            `json:"endpoint,omitempty"`
	Defaults       *S3Defaults       `json:"defaults,omitempty"`
	ApplySortArray bool              `json:"sortArray,omitempty"`
}

// S3OperationRule is one declarative response rule. Rules are sorted by
// Priority DESC at dispatch and the first matching enabled rule wins.
type S3OperationRule struct {
	ID        string         `json:"id"`
	Operation string         `json:"operation"`
	Match     S3RuleMatch    `json:"match"`
	Response  S3RuleResponse `json:"response"`
	Priority  int            `json:"priority"`
	Enabled   bool           `json:"enabled"`
}

// S3RuleMatch narrows a rule by bucket / key. Exact fields take precedence;
// BucketPattern is a regex, KeyPattern a glob, KeyPrefix a HasPrefix check.
// Empty fields impose no filter on that axis.
type S3RuleMatch struct {
	Bucket        string `json:"bucket,omitempty"`
	BucketPattern string `json:"bucketPattern,omitempty"`
	Key           string `json:"key,omitempty"`
	KeyPattern    string `json:"keyPattern,omitempty"`
	KeyPrefix     string `json:"keyPrefix,omitempty"`
}

// S3RuleResponse is the response a matching rule synthesises. Type selects the
// variant: "file" (TemplatePath), "inline" (base64 InlineBody), "xmlError"
// (ErrorCode/ErrorMessage) or "passthrough" (fall back to the in-memory S3
// engine). UserMeta is emitted as x-amz-meta-* headers.
type S3RuleResponse struct {
	UserMeta       map[string]string `json:"userMeta,omitempty"`
	Type           string            `json:"type"`
	TemplatePath   string            `json:"templatePath,omitempty"`
	InlineBody     string            `json:"inlineBody,omitempty"`
	ContentType    string            `json:"contentType,omitempty"`
	ETag           string            `json:"etag,omitempty"`
	LastModified   string            `json:"lastModified,omitempty"`
	CacheControl   string            `json:"cacheControl,omitempty"`
	ErrorCode      string            `json:"errorCode,omitempty"`
	ErrorMessage   string            `json:"errorMessage,omitempty"`
	StatusOverride int               `json:"statusOverride,omitempty"`
	DelayMs        int               `json:"delayMs,omitempty"`
}

// S3Defaults controls the dispatcher fallback when no rule matches.
// PassthroughToBackend=true hands the request to the in-memory S3 engine.
type S3Defaults struct {
	Owner                *S3Owner `json:"owner,omitempty"`
	DefaultErrorCode     string   `json:"defaultErrorCode,omitempty"`
	ExtraBuckets         []string `json:"extraBuckets,omitempty"`
	PassthroughToBackend bool     `json:"passthroughToBackend"`
}

// S3Owner is the bucket owner reported in ListBuckets / ACL envelopes.
type S3Owner struct {
	ID          string `json:"id,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
}

// ToxicityRule is a chaos rule scoped to an S3 mock (latency, errors, data
// corruption). Type names the fault; Params carries fault-specific knobs.
type ToxicityRule struct {
	Params      map[string]interface{} `json:"params,omitempty"`
	Type        string                 `json:"type"`
	Scope       ToxicityScope          `json:"scope,omitempty"`
	Probability int                    `json:"probability,omitempty"`
	Priority    int                    `json:"priority,omitempty"`
	Disabled    bool                   `json:"disabled,omitempty"`
}

// ToxicityScope narrows a toxicity rule to specific buckets / keys / S3
// operations. Empty fields impose no filter.
type ToxicityScope struct {
	BucketRegex string   `json:"bucketRegex,omitempty"`
	KeyRegex    string   `json:"keyRegex,omitempty"`
	Operations  []string `json:"operations,omitempty"`
}

// RabbitMQOutputProps holds AMQP message properties for published response messages.
type RabbitMQOutputProps struct {
	DeliveryMode  uint8  `json:"deliveryMode,omitempty"`
	CorrelationID string `json:"correlationId,omitempty"`
	ReplyTo       string `json:"replyTo,omitempty"`
	ContentType   string `json:"contentType,omitempty"`
	Priority      uint8  `json:"priority,omitempty"`
	MessageID     string `json:"messageId,omitempty"`
	Type          string `json:"type,omitempty"`
	AppID         string `json:"appId,omitempty"`
}

// ---------------------------------------------------------------------------
// Response types
// ---------------------------------------------------------------------------

// SSEEvent represents a single event in an SSE chain.
type SSEEvent struct {
	EventName    string `json:"eventName,omitempty"`
	Data         any    `json:"data"`
	Delay        int32  `json:"delay,omitempty"`
	ID           string `json:"id,omitempty"`
	Retry        *int64 `json:"retry,omitempty"`
	TemplatePath string `json:"templatePath,omitempty"`
}

// SSEEventChain holds a sequence of SSE events.
type SSEEventChain struct {
	Events    []SSEEvent `json:"events"`
	Loop      bool       `json:"loop,omitempty"`
	LoopDelay int32      `json:"loopDelay,omitempty"`
	Heartbeat int32      `json:"heartbeat,omitempty"`
	MaxLoops  int32      `json:"maxLoops,omitempty"`
	MaxTime   int32      `json:"maxTime,omitempty"`
	LogBatch  int32      `json:"logBatch,omitempty"`
}

// ErrorDetailsType classifies a gRPC error detail kind.
type ErrorDetailsType string

const (
	ErrorDetailsBadRequest          ErrorDetailsType = "badRequest"
	ErrorDetailsHelp                ErrorDetailsType = "help"
	ErrorDetailsDebugInfo           ErrorDetailsType = "debugInfo"
	ErrorDetailsErrorInfo           ErrorDetailsType = "errorInfo"
	ErrorDetailsLocalizedMessage    ErrorDetailsType = "localizedMessage"
	ErrorDetailsPreconditionFailure ErrorDetailsType = "preconditionFailure"
	ErrorDetailsQuotaFailure        ErrorDetailsType = "quotaFailure"
	ErrorDetailsRequestInfo         ErrorDetailsType = "requestInfo"
	ErrorDetailsResourceInfo        ErrorDetailsType = "resourceInfo"
	ErrorDetailsRetryInfo           ErrorDetailsType = "retryInfo"
)

// ErrorDetails provides structured error detail information (gRPC).
type ErrorDetails struct {
	Type    ErrorDetailsType       `json:"type,omitempty"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// GraphQLErrorLocation describes a position in a GraphQL query.
type GraphQLErrorLocation struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

// GraphQLError follows the GraphQL June 2018 spec.
type GraphQLError struct {
	Message    string                 `json:"message"`
	Path       []interface{}          `json:"path,omitempty"`
	Locations  []GraphQLErrorLocation `json:"locations,omitempty"`
	Extensions map[string]interface{} `json:"extensions,omitempty"`
}

// SOAPFault represents a SOAP 1.1 fault.
type SOAPFault struct {
	FaultCode   string `json:"faultCode"`
	FaultString string `json:"faultString"`
	FaultActor  string `json:"faultActor,omitempty"`
	Detail      string `json:"detail,omitempty"`
	HTTPStatus  int    `json:"httpStatus,omitempty"`
}

// ContentResponse represents the response a mock returns.
type ContentResponse struct {
	Headers             map[string][]string `json:"headers,omitempty"`
	StatusCode          uint32              `json:"statusCode,omitempty"`
	Decode              Decode              `json:"decode,omitempty"`
	Payload             any                 `json:"payload,omitempty"`
	PayloadTemplatePath string              `json:"payloadTemplatePath,omitempty"`
	Error               string              `json:"error,omitempty"`
	ErrorDetails        *[]ErrorDetails     `json:"errorDetails,omitempty"`
	Delay               int32               `json:"delay,omitempty"`
	SSEEventChain       *SSEEventChain      `json:"sseEventChain,omitempty"`
	GraphQLErrors       []GraphQLError      `json:"graphqlErrors,omitempty"`
	SOAPFault           *SOAPFault          `json:"soapFault,omitempty"`
	MCPIsError          bool                `json:"mcpIsError,omitempty"`
}

// OneOf allows a mock to return one of multiple responses.
type OneOf struct {
	Order     OneOfOrder        `json:"order"`
	Offset    int               `json:"offset"`
	Responses []ContentResponse `json:"responses,omitempty"`
}

// Proxy defines a proxy target.
type Proxy struct {
	Target string `json:"target,omitempty"`
}

// Callback defines a webhook or messaging callback triggered after mock resolution.
type Callback struct {
	Type       CallbackType      `json:"type,omitempty"`
	URL        string            `json:"url,omitempty"`
	Method     string            `json:"method,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       any               `json:"body,omitempty"`
	Timeout    int               `json:"timeout,omitempty"`
	RetryCount int               `json:"retryCount,omitempty"`
	RetryDelay int               `json:"retryDelay,omitempty"`
	Async      bool              `json:"async,omitempty"`
	Trigger    CallbackTrigger   `json:"trigger,omitempty"`

	// Kafka-specific fields
	KafkaBrokers  string `json:"kafkaBrokers,omitempty"`
	KafkaTopic    string `json:"kafkaTopic,omitempty"`
	KafkaKey      string `json:"kafkaKey,omitempty"`
	KafkaUsername string `json:"kafkaUsername,omitempty"`
	KafkaPassword string `json:"kafkaPassword,omitempty"`
	KafkaUseSASL  bool   `json:"kafkaUseSASL,omitempty"`
	KafkaUseTLS   bool   `json:"kafkaUseTLS,omitempty"`

	// RabbitMQ-specific fields
	RabbitURL        string `json:"rabbitURL,omitempty"`
	RabbitExchange   string `json:"rabbitExchange,omitempty"`
	RabbitRoutingKey string `json:"rabbitRoutingKey,omitempty"`
	RabbitQueue      string `json:"rabbitQueue,omitempty"`
	RabbitMandatory  bool   `json:"rabbitMandatory,omitempty"`
}

// ---------------------------------------------------------------------------
// Extract (store extraction from requests)
// ---------------------------------------------------------------------------

// Extract defines how to extract values from a request into stores.
type Extract struct {
	MStore map[string]any `json:"mStore,omitempty"`
	CStore map[string]any `json:"cStore,omitempty"`
	GStore map[string]any `json:"gStore,omitempty"`
}

// ---------------------------------------------------------------------------
// Mock (top-level)
// ---------------------------------------------------------------------------

// Mock represents a complete mock definition in Mockarty.
type Mock struct {
	ID        string `json:"id,omitempty"`
	ChainID   string `json:"chainId,omitempty"`
	Namespace string `json:"namespace,omitempty"`

	PathPrefix string `json:"pathPrefix,omitempty"`
	ServerName string `json:"serverName,omitempty"`

	// Protocol-specific request contexts (only one should be set)
	HTTP     *HttpRequestContext     `json:"http,omitempty"`
	GRPC     *GrpcRequestContext     `json:"grpc,omitempty"`
	MCP      *MCPRequestContext      `json:"mcp,omitempty"`
	Socket   *SocketRequestContext   `json:"socket,omitempty"`
	SOAP     *SoapRequestContext     `json:"soap,omitempty"`
	GraphQL  *GraphQLRequestContext  `json:"graphql,omitempty"`
	SSE      *SSERequestContext      `json:"sse,omitempty"`
	Kafka    *KafkaRequestContext    `json:"kafka,omitempty"`
	RabbitMQ *RabbitMQRequestContext `json:"rabbitmq,omitempty"`
	SMTP     *SmtpRequestContext     `json:"smtp,omitempty"`
	S3       *S3RequestContext       `json:"s3,omitempty"`

	// Response configuration (use exactly one of Response, OneOf, or Proxy)
	Response  *ContentResponse `json:"response,omitempty"`
	OneOf     *OneOf           `json:"oneOf,omitempty"`
	Proxy     *Proxy           `json:"proxy,omitempty"`
	Callbacks []Callback       `json:"webhooks,omitempty"`

	// Lifecycle
	TTL        int64 `json:"ttl,omitempty"`
	UseLimiter int32 `json:"useLimiter,omitempty"`
	UseCounter int32 `json:"useCounter,omitempty"`
	Priority   int64 `json:"priority,omitempty"`

	// Metadata
	Tags     []string `json:"tags,omitempty"`
	FolderID string   `json:"folderId,omitempty"`

	// Timestamps (set by server)
	CreatedAt int64 `json:"createdAt,omitempty"`
	UpdatedAt int64 `json:"updatedAt,omitempty"`
	LastUse   int64 `json:"lastUse,omitempty"`
	ExpireAt  int64 `json:"expireAt,omitempty"`
	ClosedAt  int64 `json:"closedAt,omitempty"`

	// Store
	Extract   *Extract       `json:"extract,omitempty"`
	MockStore map[string]any `json:"mStore,omitempty"`
}

// ---------------------------------------------------------------------------
// API response types
// ---------------------------------------------------------------------------

// SaveMockResponse is the response from creating/updating a mock.
//
// Wire shape (admin node, POST /api/v1/mocks):
//
//	{"id":"...","mock":{...},"isNew":<bool>,"success":true,"message":"..."}
//
// The server's `isNew` field is semantically "was overwrite" — it's
// true when an existing mock with the same id was replaced. The legacy
// field name `overwritten` was never emitted by the server (carried
// over from an internal draft model), so older SDKs silently observed
// `false` for genuine overwrites. We bind to `isNew` as the canonical
// wire name and surface both `IsNew` and `Overwritten` as accessors so
// existing call sites keep compiling. The full envelope (id, success,
// message) is exposed too so callers can correlate by id.
type SaveMockResponse struct {
	Mock    Mock   `json:"mock"`
	ID      string `json:"id,omitempty"`
	Message string `json:"message,omitempty"`
	IsNew   bool   `json:"isNew"`
	// Overwritten is a deprecated alias for IsNew — kept for back-compat
	// with code that read the old field name. Always equals IsNew after
	// UnmarshalJSON.
	Overwritten bool `json:"-"`
	Success     bool `json:"success,omitempty"`
}

// UnmarshalJSON propagates IsNew into the deprecated Overwritten field
// AND accepts the legacy `"overwritten"` wire shape so a downgraded
// server (still emitting the old key) is decoded correctly.
func (s *SaveMockResponse) UnmarshalJSON(data []byte) error {
	type alias SaveMockResponse
	var aux struct {
		alias
		LegacyOverwritten *bool `json:"overwritten,omitempty"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	*s = SaveMockResponse(aux.alias)
	if aux.LegacyOverwritten != nil && !s.IsNew {
		s.IsNew = *aux.LegacyOverwritten
	}
	s.Overwritten = s.IsNew
	return nil
}

// MockListResponse is the response from listing mocks.
//
// Wire shape (admin node, GET /api/v1/mocks):
//
//	{"mocks":[...], "count":N, "limit":N, "message":"..."}
//
// The legacy field names (`items` / `total`) were never emitted by the
// server — they were carried over from an internal draft. SDKs binding
// to the old names silently received empty slices on every call (the
// drift would surface as `len(resp.Items) == 0` even when mocks
// existed). Re-bound to the canonical wire names; legacy keys still
// accepted via UnmarshalJSON for forward-compat with downgraded
// servers. Surfaced 2026-05-17 by the same bug-pattern hunt that
// caught SaveMockResponse.isNew.
type MockListResponse struct {
	Items   []Mock `json:"mocks"`
	Message string `json:"message,omitempty"`
	Count   int    `json:"count"`
	Limit   int    `json:"limit,omitempty"`
	// Total is a deprecated alias for Count — populated by
	// UnmarshalJSON so legacy callers don't break.
	Total int `json:"-"`
}

// UnmarshalJSON accepts both the canonical {mocks, count} shape and the
// legacy {items, total} shape so a downgraded admin server still
// decodes correctly. Total is mirrored to Count after either path.
func (r *MockListResponse) UnmarshalJSON(data []byte) error {
	type alias MockListResponse
	var aux struct {
		alias
		LegacyItems *[]Mock `json:"items,omitempty"`
		LegacyTotal *int    `json:"total,omitempty"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	*r = MockListResponse(aux.alias)
	if len(r.Items) == 0 && aux.LegacyItems != nil {
		r.Items = *aux.LegacyItems
	}
	if r.Count == 0 && aux.LegacyTotal != nil {
		r.Count = *aux.LegacyTotal
	}
	r.Total = r.Count
	return nil
}

// RequestLog represents a single request log entry.
type RequestLog struct {
	ID       string `json:"id,omitempty"`
	CalledAt string `json:"calledAt"`
	Req      any    `json:"req"`
	Response any    `json:"response,omitempty"`
}

// MockLogs represents logs for a specific mock.
type MockLogs struct {
	ID       string       `json:"id"`
	Requests []RequestLog `json:"requests"`
}
