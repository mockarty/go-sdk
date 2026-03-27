// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

package mockarty

// ---------------------------------------------------------------------------
// MockBuilder — fluent API for constructing Mock objects
// ---------------------------------------------------------------------------

// MockBuilder provides a fluent interface for creating Mock definitions.
type MockBuilder struct {
	mock Mock
}

// NewMockBuilder creates a new MockBuilder.
func NewMockBuilder() *MockBuilder {
	return &MockBuilder{}
}

// Build returns the constructed Mock. It does not validate; the server
// validates on create/update.
func (b *MockBuilder) Build() *Mock {
	m := b.mock
	return &m
}

// ID sets the mock identifier.
func (b *MockBuilder) ID(id string) *MockBuilder {
	b.mock.ID = id
	return b
}

// Namespace sets the namespace.
func (b *MockBuilder) Namespace(ns string) *MockBuilder {
	b.mock.Namespace = ns
	return b
}

// ChainID sets the chain identifier for linking related mocks.
func (b *MockBuilder) ChainID(id string) *MockBuilder {
	b.mock.ChainID = id
	return b
}

// PathPrefix sets a unified path prefix.
func (b *MockBuilder) PathPrefix(prefix string) *MockBuilder {
	b.mock.PathPrefix = prefix
	return b
}

// ServerName sets the server name for grouping mocks.
func (b *MockBuilder) ServerName(name string) *MockBuilder {
	b.mock.ServerName = name
	return b
}

// Tags sets the tag list.
func (b *MockBuilder) Tags(tags ...string) *MockBuilder {
	b.mock.Tags = tags
	return b
}

// FolderID sets the folder identifier.
func (b *MockBuilder) FolderID(id string) *MockBuilder {
	b.mock.FolderID = id
	return b
}

// TTL sets the time-to-live in seconds.
func (b *MockBuilder) TTL(seconds int64) *MockBuilder {
	b.mock.TTL = seconds
	return b
}

// UseLimiter sets the maximum number of uses.
func (b *MockBuilder) UseLimiter(n int32) *MockBuilder {
	b.mock.UseLimiter = n
	return b
}

// Priority sets the mock priority (higher = matched first).
func (b *MockBuilder) Priority(p int64) *MockBuilder {
	b.mock.Priority = p
	return b
}

// MockStore sets the mock-scoped store values.
func (b *MockBuilder) MockStore(store map[string]any) *MockBuilder {
	b.mock.MockStore = store
	return b
}

// ExtractConfig sets the extraction configuration.
func (b *MockBuilder) ExtractConfig(e *Extract) *MockBuilder {
	b.mock.Extract = e
	return b
}

// Callbacks sets the webhook/messaging callbacks.
func (b *MockBuilder) Callbacks(cbs ...Callback) *MockBuilder {
	b.mock.Callbacks = cbs
	return b
}

// ---------------------------------------------------------------------------
// Protocol builders
// ---------------------------------------------------------------------------

// HTTP configures an HTTP request context using the provided builder function.
func (b *MockBuilder) HTTP(fn func(h *HTTPBuilder)) *MockBuilder {
	hb := &HTTPBuilder{}
	fn(hb)
	b.mock.HTTP = hb.build()
	return b
}

// GRPC configures a gRPC request context.
func (b *MockBuilder) GRPC(fn func(g *GRPCBuilder)) *MockBuilder {
	gb := &GRPCBuilder{}
	fn(gb)
	b.mock.GRPC = gb.build()
	return b
}

// MCPBuilder configures an MCP request context.
func (b *MockBuilder) MCPConfig(fn func(m *MCPBuilderCtx)) *MockBuilder {
	mb := &MCPBuilderCtx{}
	fn(mb)
	b.mock.MCP = mb.build()
	return b
}

// SocketConfig configures a Socket request context.
func (b *MockBuilder) SocketConfig(fn func(s *SocketBuilder)) *MockBuilder {
	sb := &SocketBuilder{}
	fn(sb)
	b.mock.Socket = sb.build()
	return b
}

// SOAPConfig configures a SOAP request context.
func (b *MockBuilder) SOAPConfig(fn func(s *SOAPBuilder)) *MockBuilder {
	sb := &SOAPBuilder{}
	fn(sb)
	b.mock.SOAP = sb.build()
	return b
}

// GraphQLConfig configures a GraphQL request context.
func (b *MockBuilder) GraphQLConfig(fn func(g *GraphQLBuilder)) *MockBuilder {
	gb := &GraphQLBuilder{}
	fn(gb)
	b.mock.GraphQL = gb.build()
	return b
}

// SSEConfig configures an SSE request context.
func (b *MockBuilder) SSEConfig(fn func(s *SSEBuilder)) *MockBuilder {
	sb := &SSEBuilder{}
	fn(sb)
	b.mock.SSE = sb.build()
	return b
}

// KafkaConfig configures a Kafka request context.
func (b *MockBuilder) KafkaConfig(fn func(k *KafkaBuilder)) *MockBuilder {
	kb := &KafkaBuilder{}
	fn(kb)
	b.mock.Kafka = kb.build()
	return b
}

// RabbitMQConfig configures a RabbitMQ request context.
func (b *MockBuilder) RabbitMQConfig(fn func(r *RabbitMQBuilder)) *MockBuilder {
	rb := &RabbitMQBuilder{}
	fn(rb)
	b.mock.RabbitMQ = rb.build()
	return b
}

// SMTPConfig configures an SMTP request context.
func (b *MockBuilder) SMTPConfig(fn func(s *SMTPBuilder)) *MockBuilder {
	sb := &SMTPBuilder{}
	fn(sb)
	b.mock.SMTP = sb.build()
	return b
}

// ---------------------------------------------------------------------------
// Response builders
// ---------------------------------------------------------------------------

// Response configures a single response using the provided builder function.
func (b *MockBuilder) Response(fn func(r *ResponseBuilder)) *MockBuilder {
	rb := &ResponseBuilder{}
	fn(rb)
	b.mock.Response = rb.build()
	return b
}

// OneOfConfig configures multiple responses (round-robin or random).
func (b *MockBuilder) OneOfConfig(order OneOfOrder, fns ...func(r *ResponseBuilder)) *MockBuilder {
	oo := &OneOf{Order: order}
	for _, fn := range fns {
		rb := &ResponseBuilder{}
		fn(rb)
		oo.Responses = append(oo.Responses, *rb.build())
	}
	b.mock.OneOf = oo
	return b
}

// ProxyTo sets the proxy target URL.
func (b *MockBuilder) ProxyTo(target string) *MockBuilder {
	b.mock.Proxy = &Proxy{Target: target}
	return b
}

// ===========================================================================
// HTTPBuilder
// ===========================================================================

// HTTPBuilder builds an HttpRequestContext.
type HTTPBuilder struct {
	ctx HttpRequestContext
}

func (h *HTTPBuilder) build() *HttpRequestContext {
	c := h.ctx
	return &c
}

// Route sets the URL route pattern, e.g. "/api/users/:id".
func (h *HTTPBuilder) Route(route string) *HTTPBuilder {
	h.ctx.Route = route
	return h
}

// RoutePattern sets an optional route pattern.
func (h *HTTPBuilder) RoutePattern(pattern string) *HTTPBuilder {
	h.ctx.RoutePattern = pattern
	return h
}

// Method sets the HTTP method (GET, POST, PUT, DELETE, PATCH, etc.).
func (h *HTTPBuilder) Method(method string) *HTTPBuilder {
	h.ctx.HttpMethod = method
	return h
}

// BodyCondition adds a request body condition.
func (h *HTTPBuilder) BodyCondition(path string, action AssertAction, value any) *HTTPBuilder {
	h.ctx.Conditions = append(h.ctx.Conditions, &Condition{
		Path:         path,
		AssertAction: action,
		Value:        value,
	})
	return h
}

// QueryCondition adds a query parameter condition.
func (h *HTTPBuilder) QueryCondition(path string, action AssertAction, value any) *HTTPBuilder {
	h.ctx.QueryParams = append(h.ctx.QueryParams, &Condition{
		Path:         path,
		AssertAction: action,
		Value:        value,
	})
	return h
}

// HeaderCondition adds a header condition.
func (h *HTTPBuilder) HeaderCondition(path string, action AssertAction, value any) *HTTPBuilder {
	h.ctx.Headers = append(h.ctx.Headers, &Condition{
		Path:         path,
		AssertAction: action,
		Value:        value,
	})
	return h
}

// SortArrays enables global array sorting for condition matching.
func (h *HTTPBuilder) SortArrays() *HTTPBuilder {
	h.ctx.ApplySortArray = true
	return h
}

// ===========================================================================
// GRPCBuilder
// ===========================================================================

// GRPCBuilder builds a GrpcRequestContext.
type GRPCBuilder struct {
	ctx GrpcRequestContext
}

func (g *GRPCBuilder) build() *GrpcRequestContext {
	c := g.ctx
	return &c
}

// Service sets the gRPC service name.
func (g *GRPCBuilder) Service(name string) *GRPCBuilder {
	g.ctx.Service = name
	return g
}

// Method sets the gRPC method name.
func (g *GRPCBuilder) Method(name string) *GRPCBuilder {
	g.ctx.Method = name
	return g
}

// MethodType sets the gRPC method type (unary, server-stream, etc.).
func (g *GRPCBuilder) MethodType(t string) *GRPCBuilder {
	g.ctx.MethodType = t
	return g
}

// BodyCondition adds a request body condition.
func (g *GRPCBuilder) BodyCondition(path string, action AssertAction, value any) *GRPCBuilder {
	g.ctx.Conditions = append(g.ctx.Conditions, &Condition{
		Path:         path,
		AssertAction: action,
		Value:        value,
	})
	return g
}

// MetaCondition adds a metadata condition.
func (g *GRPCBuilder) MetaCondition(path string, action AssertAction, value any) *GRPCBuilder {
	g.ctx.Meta = append(g.ctx.Meta, &Condition{
		Path:         path,
		AssertAction: action,
		Value:        value,
	})
	return g
}

// ===========================================================================
// MCPBuilderCtx
// ===========================================================================

// MCPBuilderCtx builds an MCPRequestContext.
type MCPBuilderCtx struct {
	ctx MCPRequestContext
}

func (m *MCPBuilderCtx) build() *MCPRequestContext {
	c := m.ctx
	return &c
}

// Method sets the MCP method (e.g. "tools/call", "resources/read").
func (m *MCPBuilderCtx) Method(method string) *MCPBuilderCtx {
	m.ctx.Method = method
	return m
}

// Tool sets the MCP tool name.
func (m *MCPBuilderCtx) Tool(name string) *MCPBuilderCtx {
	m.ctx.Tool = name
	return m
}

// Resource sets the MCP resource URI.
func (m *MCPBuilderCtx) Resource(uri string) *MCPBuilderCtx {
	m.ctx.Resource = uri
	return m
}

// Description sets the MCP tool/resource description.
func (m *MCPBuilderCtx) Description(desc string) *MCPBuilderCtx {
	m.ctx.Description = desc
	return m
}

// BodyCondition adds a request body condition.
func (m *MCPBuilderCtx) BodyCondition(path string, action AssertAction, value any) *MCPBuilderCtx {
	m.ctx.Conditions = append(m.ctx.Conditions, &Condition{
		Path:         path,
		AssertAction: action,
		Value:        value,
	})
	return m
}

// HeaderCondition adds a header condition.
func (m *MCPBuilderCtx) HeaderCondition(path string, action AssertAction, value any) *MCPBuilderCtx {
	m.ctx.Headers = append(m.ctx.Headers, &Condition{
		Path:         path,
		AssertAction: action,
		Value:        value,
	})
	return m
}

// ===========================================================================
// SocketBuilder
// ===========================================================================

// SocketBuilder builds a SocketRequestContext.
type SocketBuilder struct {
	ctx SocketRequestContext
}

func (s *SocketBuilder) build() *SocketRequestContext {
	c := s.ctx
	return &c
}

// ServerName sets the socket server name.
func (s *SocketBuilder) ServerName(name string) *SocketBuilder {
	s.ctx.ServerName = name
	return s
}

// Event sets the socket event name.
func (s *SocketBuilder) Event(event string) *SocketBuilder {
	s.ctx.Event = event
	return s
}

// SocketNamespace sets the socket.io namespace.
func (s *SocketBuilder) SocketNamespace(ns string) *SocketBuilder {
	s.ctx.Namespace = ns
	return s
}

// BodyCondition adds a request body condition.
func (s *SocketBuilder) BodyCondition(path string, action AssertAction, value any) *SocketBuilder {
	s.ctx.Conditions = append(s.ctx.Conditions, &Condition{
		Path:         path,
		AssertAction: action,
		Value:        value,
	})
	return s
}

// ===========================================================================
// SOAPBuilder
// ===========================================================================

// SOAPBuilder builds a SoapRequestContext.
type SOAPBuilder struct {
	ctx SoapRequestContext
}

func (s *SOAPBuilder) build() *SoapRequestContext {
	c := s.ctx
	return &c
}

// Service sets the SOAP service name.
func (s *SOAPBuilder) Service(name string) *SOAPBuilder {
	s.ctx.Service = name
	return s
}

// Method sets the SOAP method name.
func (s *SOAPBuilder) Method(name string) *SOAPBuilder {
	s.ctx.Method = name
	return s
}

// Action sets the SOAPAction header value.
func (s *SOAPBuilder) Action(action string) *SOAPBuilder {
	s.ctx.Action = action
	return s
}

// Path sets the URL path for the SOAP endpoint.
func (s *SOAPBuilder) Path(path string) *SOAPBuilder {
	s.ctx.Path = path
	return s
}

// BodyCondition adds a request body condition.
func (s *SOAPBuilder) BodyCondition(path string, action AssertAction, value any) *SOAPBuilder {
	s.ctx.Conditions = append(s.ctx.Conditions, &Condition{
		Path:         path,
		AssertAction: action,
		Value:        value,
	})
	return s
}

// HeaderCondition adds a header condition.
func (s *SOAPBuilder) HeaderCondition(path string, action AssertAction, value any) *SOAPBuilder {
	s.ctx.Headers = append(s.ctx.Headers, &Condition{
		Path:         path,
		AssertAction: action,
		Value:        value,
	})
	return s
}

// ===========================================================================
// GraphQLBuilder
// ===========================================================================

// GraphQLBuilder builds a GraphQLRequestContext.
type GraphQLBuilder struct {
	ctx GraphQLRequestContext
}

func (g *GraphQLBuilder) build() *GraphQLRequestContext {
	c := g.ctx
	return &c
}

// Operation sets the GraphQL operation type: "query", "mutation", "subscription".
func (g *GraphQLBuilder) Operation(op string) *GraphQLBuilder {
	g.ctx.Operation = op
	return g
}

// Field sets the GraphQL field name.
func (g *GraphQLBuilder) Field(field string) *GraphQLBuilder {
	g.ctx.Field = field
	return g
}

// TypeName sets the GraphQL type.
func (g *GraphQLBuilder) TypeName(t string) *GraphQLBuilder {
	g.ctx.Type = t
	return g
}

// Path sets the URL path for GraphQL endpoint identification.
func (g *GraphQLBuilder) Path(path string) *GraphQLBuilder {
	g.ctx.Path = path
	return g
}

// BodyCondition adds a request body condition.
func (g *GraphQLBuilder) BodyCondition(path string, action AssertAction, value any) *GraphQLBuilder {
	g.ctx.Conditions = append(g.ctx.Conditions, &Condition{
		Path:         path,
		AssertAction: action,
		Value:        value,
	})
	return g
}

// HeaderCondition adds a header condition.
func (g *GraphQLBuilder) HeaderCondition(path string, action AssertAction, value any) *GraphQLBuilder {
	g.ctx.Headers = append(g.ctx.Headers, &Condition{
		Path:         path,
		AssertAction: action,
		Value:        value,
	})
	return g
}

// ===========================================================================
// SSEBuilder
// ===========================================================================

// SSEBuilder builds an SSERequestContext.
type SSEBuilder struct {
	ctx SSERequestContext
}

func (s *SSEBuilder) build() *SSERequestContext {
	c := s.ctx
	return &c
}

// EventPath sets the SSE event path, e.g. "/events/notifications".
func (s *SSEBuilder) EventPath(path string) *SSEBuilder {
	s.ctx.EventPath = path
	return s
}

// EventName sets the SSE event name.
func (s *SSEBuilder) EventName(name string) *SSEBuilder {
	s.ctx.EventName = name
	return s
}

// Description sets a description.
func (s *SSEBuilder) Description(desc string) *SSEBuilder {
	s.ctx.Description = desc
	return s
}

// BodyCondition adds a condition (typically on query parameters).
func (s *SSEBuilder) BodyCondition(path string, action AssertAction, value any) *SSEBuilder {
	s.ctx.Conditions = append(s.ctx.Conditions, &Condition{
		Path:         path,
		AssertAction: action,
		Value:        value,
	})
	return s
}

// HeaderCondition adds a header condition.
func (s *SSEBuilder) HeaderCondition(path string, action AssertAction, value any) *SSEBuilder {
	s.ctx.HeaderConditions = append(s.ctx.HeaderConditions, &Condition{
		Path:         path,
		AssertAction: action,
		Value:        value,
	})
	return s
}

// ===========================================================================
// KafkaBuilder
// ===========================================================================

// KafkaBuilder builds a KafkaRequestContext.
type KafkaBuilder struct {
	ctx KafkaRequestContext
}

func (k *KafkaBuilder) build() *KafkaRequestContext {
	c := k.ctx
	return &c
}

// Topic sets the Kafka topic.
func (k *KafkaBuilder) Topic(topic string) *KafkaBuilder {
	k.ctx.Topic = topic
	return k
}

// ServerName sets the Kafka server name.
func (k *KafkaBuilder) ServerName(name string) *KafkaBuilder {
	k.ctx.ServerName = name
	return k
}

// ConsumerGroup sets the consumer group.
func (k *KafkaBuilder) ConsumerGroup(group string) *KafkaBuilder {
	k.ctx.ConsumerGroup = group
	return k
}

// OutputTopic sets the output topic for publishing the response.
func (k *KafkaBuilder) OutputTopic(topic string) *KafkaBuilder {
	k.ctx.OutputTopic = topic
	return k
}

// OutputBrokers sets the output brokers.
func (k *KafkaBuilder) OutputBrokers(brokers string) *KafkaBuilder {
	k.ctx.OutputBrokers = brokers
	return k
}

// OutputKey sets the output message key.
func (k *KafkaBuilder) OutputKey(key string) *KafkaBuilder {
	k.ctx.OutputKey = key
	return k
}

// OutputHeaders sets the output message headers.
func (k *KafkaBuilder) OutputHeaders(headers map[string]string) *KafkaBuilder {
	k.ctx.OutputHeaders = headers
	return k
}

// BodyCondition adds a message body condition.
func (k *KafkaBuilder) BodyCondition(path string, action AssertAction, value any) *KafkaBuilder {
	k.ctx.Conditions = append(k.ctx.Conditions, &Condition{
		Path:         path,
		AssertAction: action,
		Value:        value,
	})
	return k
}

// HeaderCondition adds a message header condition.
func (k *KafkaBuilder) HeaderCondition(path string, action AssertAction, value any) *KafkaBuilder {
	k.ctx.Headers = append(k.ctx.Headers, &Condition{
		Path:         path,
		AssertAction: action,
		Value:        value,
	})
	return k
}

// ===========================================================================
// RabbitMQBuilder
// ===========================================================================

// RabbitMQBuilder builds a RabbitMQRequestContext.
type RabbitMQBuilder struct {
	ctx RabbitMQRequestContext
}

func (r *RabbitMQBuilder) build() *RabbitMQRequestContext {
	c := r.ctx
	return &c
}

// Queue sets the RabbitMQ queue name.
func (r *RabbitMQBuilder) Queue(queue string) *RabbitMQBuilder {
	r.ctx.Queue = queue
	return r
}

// Exchange sets the RabbitMQ exchange name.
func (r *RabbitMQBuilder) Exchange(exchange string) *RabbitMQBuilder {
	r.ctx.Exchange = exchange
	return r
}

// RoutingKey sets the routing key.
func (r *RabbitMQBuilder) RoutingKey(key string) *RabbitMQBuilder {
	r.ctx.RoutingKey = key
	return r
}

// ServerName sets the RabbitMQ server name.
func (r *RabbitMQBuilder) ServerName(name string) *RabbitMQBuilder {
	r.ctx.ServerName = name
	return r
}

// OutputURL sets the output AMQP connection URI.
func (r *RabbitMQBuilder) OutputURL(url string) *RabbitMQBuilder {
	r.ctx.OutputURL = url
	return r
}

// OutputExchange sets the output exchange name.
func (r *RabbitMQBuilder) OutputExchange(exchange string) *RabbitMQBuilder {
	r.ctx.OutputExchange = exchange
	return r
}

// OutputRoutingKey sets the output routing key.
func (r *RabbitMQBuilder) OutputRoutingKey(key string) *RabbitMQBuilder {
	r.ctx.OutputRoutingKey = key
	return r
}

// OutputQueue sets the output queue name.
func (r *RabbitMQBuilder) OutputQueue(queue string) *RabbitMQBuilder {
	r.ctx.OutputQueue = queue
	return r
}

// OutputProps sets the output AMQP properties.
func (r *RabbitMQBuilder) OutputProps(props *RabbitMQOutputProps) *RabbitMQBuilder {
	r.ctx.OutputProps = props
	return r
}

// BodyCondition adds a message body condition.
func (r *RabbitMQBuilder) BodyCondition(path string, action AssertAction, value any) *RabbitMQBuilder {
	r.ctx.Conditions = append(r.ctx.Conditions, &Condition{
		Path:         path,
		AssertAction: action,
		Value:        value,
	})
	return r
}

// HeaderCondition adds a message header condition.
func (r *RabbitMQBuilder) HeaderCondition(path string, action AssertAction, value any) *RabbitMQBuilder {
	r.ctx.Headers = append(r.ctx.Headers, &Condition{
		Path:         path,
		AssertAction: action,
		Value:        value,
	})
	return r
}

// ===========================================================================
// SMTPBuilder
// ===========================================================================

// SMTPBuilder builds a SmtpRequestContext.
type SMTPBuilder struct {
	ctx SmtpRequestContext
}

func (s *SMTPBuilder) build() *SmtpRequestContext {
	c := s.ctx
	return &c
}

// ServerName sets the SMTP server name.
func (s *SMTPBuilder) ServerName(name string) *SMTPBuilder {
	s.ctx.ServerName = name
	return s
}

// SenderCondition adds a sender matching condition.
func (s *SMTPBuilder) SenderCondition(path string, action AssertAction, value any) *SMTPBuilder {
	s.ctx.SenderConditions = append(s.ctx.SenderConditions, &Condition{
		Path:         path,
		AssertAction: action,
		Value:        value,
	})
	return s
}

// RecipientCondition adds a recipient matching condition.
func (s *SMTPBuilder) RecipientCondition(path string, action AssertAction, value any) *SMTPBuilder {
	s.ctx.RecipientConditions = append(s.ctx.RecipientConditions, &Condition{
		Path:         path,
		AssertAction: action,
		Value:        value,
	})
	return s
}

// SubjectCondition adds a subject matching condition.
func (s *SMTPBuilder) SubjectCondition(path string, action AssertAction, value any) *SMTPBuilder {
	s.ctx.SubjectConditions = append(s.ctx.SubjectConditions, &Condition{
		Path:         path,
		AssertAction: action,
		Value:        value,
	})
	return s
}

// BodyCondition adds a body matching condition.
func (s *SMTPBuilder) BodyCondition(path string, action AssertAction, value any) *SMTPBuilder {
	s.ctx.BodyConditions = append(s.ctx.BodyConditions, &Condition{
		Path:         path,
		AssertAction: action,
		Value:        value,
	})
	return s
}

// HeaderCondition adds a header matching condition.
func (s *SMTPBuilder) HeaderCondition(path string, action AssertAction, value any) *SMTPBuilder {
	s.ctx.HeaderConditions = append(s.ctx.HeaderConditions, &Condition{
		Path:         path,
		AssertAction: action,
		Value:        value,
	})
	return s
}

// ===========================================================================
// ResponseBuilder
// ===========================================================================

// ResponseBuilder builds a ContentResponse.
type ResponseBuilder struct {
	resp ContentResponse
}

func (r *ResponseBuilder) build() *ContentResponse {
	c := r.resp
	return &c
}

// Status sets the HTTP status code.
func (r *ResponseBuilder) Status(code uint32) *ResponseBuilder {
	r.resp.StatusCode = code
	return r
}

// Header adds a response header.
func (r *ResponseBuilder) Header(key, value string) *ResponseBuilder {
	if r.resp.Headers == nil {
		r.resp.Headers = make(map[string][]string)
	}
	r.resp.Headers[key] = append(r.resp.Headers[key], value)
	return r
}

// JSONBody sets the payload. It can be a map, slice, string, or any
// JSON-serializable value. Mockarty template expressions like
// "$.fake.Email" are evaluated server-side.
func (r *ResponseBuilder) JSONBody(payload any) *ResponseBuilder {
	r.resp.Payload = payload
	return r
}

// TemplatePath sets a file-based template for the response payload.
func (r *ResponseBuilder) TemplatePath(path string) *ResponseBuilder {
	r.resp.PayloadTemplatePath = path
	return r
}

// Error sets a gRPC/protocol error string.
func (r *ResponseBuilder) Error(msg string) *ResponseBuilder {
	r.resp.Error = msg
	return r
}

// ErrorDetailsList sets structured error details (gRPC).
func (r *ResponseBuilder) ErrorDetailsList(details []ErrorDetails) *ResponseBuilder {
	r.resp.ErrorDetails = &details
	return r
}

// Delay sets the response delay in milliseconds.
func (r *ResponseBuilder) Delay(ms int32) *ResponseBuilder {
	r.resp.Delay = ms
	return r
}

// Decode sets the payload decode strategy (e.g., DecodeBase64).
func (r *ResponseBuilder) Decode(d Decode) *ResponseBuilder {
	r.resp.Decode = d
	return r
}

// SSEChain sets an SSE event chain on the response.
func (r *ResponseBuilder) SSEChain(chain *SSEEventChain) *ResponseBuilder {
	r.resp.SSEEventChain = chain
	return r
}

// GraphQLErrorList sets GraphQL errors alongside or instead of payload.
func (r *ResponseBuilder) GraphQLErrorList(errs []GraphQLError) *ResponseBuilder {
	r.resp.GraphQLErrors = errs
	return r
}

// SOAPFaultConfig sets a SOAP fault on the response.
func (r *ResponseBuilder) SOAPFaultConfig(fault *SOAPFault) *ResponseBuilder {
	r.resp.SOAPFault = fault
	return r
}

// MCPIsError marks the MCP response as a tool-level error.
func (r *ResponseBuilder) MCPIsError(isError bool) *ResponseBuilder {
	r.resp.MCPIsError = isError
	return r
}
