// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package fuzz

// Protocol identifies the transport the target speaks. The string value
// matches the SourceType / dispatch discriminator that the server-side
// engine (internal/fuzzing/engine.go) uses to pick the right driver.
type Protocol string

const (
	// ProtocolHTTP is plain HTTP/1 or HTTP/2 — the default for REST APIs.
	ProtocolHTTP Protocol = "http"
	// ProtocolGRPC is gRPC over HTTP/2 (uses GRPCAddress + GRPCServices).
	ProtocolGRPC Protocol = "grpc"
	// ProtocolGraphQL is GraphQL over HTTP (uses GraphQLEndpoint + GraphQLPath).
	ProtocolGraphQL Protocol = "graphql"
	// ProtocolSOAP is SOAP over HTTP.
	ProtocolSOAP Protocol = "soap"
	// ProtocolWebSocket is WebSocket — single long-lived connection.
	ProtocolWebSocket Protocol = "websocket"
	// ProtocolKafka publishes to a Kafka topic.
	ProtocolKafka Protocol = "kafka"
	// ProtocolRabbitMQ publishes to a RabbitMQ exchange / queue.
	ProtocolRabbitMQ Protocol = "rabbitmq"
)

// Valid reports whether p is a known protocol value.
func (p Protocol) Valid() bool {
	switch p {
	case ProtocolHTTP, ProtocolGRPC, ProtocolGraphQL, ProtocolSOAP,
		ProtocolWebSocket, ProtocolKafka, ProtocolRabbitMQ:
		return true
	}
	return false
}

// endpoint holds the per-protocol address/route triple. The transpiler
// projects this onto the canonical FuzzOptions / TargetBaseURL fields.
type endpoint struct {
	headers       map[string]string
	address       string
	method        string
	path          string
	grpcService   string
	grpcMethod    string
	graphqlPath   string
	graphqlOpName string
	graphqlQuery  string
	protocol      Protocol
	grpcUseTLS    bool
}

// WithHTTPEndpoint configures an HTTP fuzz target.
//
//	method   — HTTP verb ("GET", "POST", "PUT", "PATCH", "DELETE", ...).
//	baseURL  — origin, e.g. "https://api.example.com".
//	path     — request path, e.g. "/api/v1/login".
//
// Calling this option after another With*Endpoint replaces the previous
// protocol selection.
func WithHTTPEndpoint(method, baseURL, path string) Option {
	return func(t *Target) {
		t.endpoint = endpoint{
			protocol: ProtocolHTTP,
			method:   method,
			address:  baseURL,
			path:     path,
		}
	}
}

// WithGRPCEndpoint configures a gRPC fuzz target.
//
//	address       — host:port the server listens on.
//	service       — fully-qualified service ("pkg.Service").
//	method        — RPC method on that service.
//	useTLS        — true to dial with TLS, false for insecure plaintext.
func WithGRPCEndpoint(address, service, method string, useTLS bool) Option {
	return func(t *Target) {
		t.endpoint = endpoint{
			protocol:    ProtocolGRPC,
			address:     address,
			grpcService: service,
			grpcMethod:  method,
			grpcUseTLS:  useTLS,
		}
	}
}

// WithGraphQLEndpoint configures a GraphQL fuzz target.
//
//	baseURL       — origin hosting the GraphQL endpoint.
//	path          — GraphQL path (typically "/graphql").
//	operationName — optional operation discriminator (empty for unnamed).
//	query         — base GraphQL document the mutator will warp.
func WithGraphQLEndpoint(baseURL, path, operationName, query string) Option {
	return func(t *Target) {
		t.endpoint = endpoint{
			protocol:      ProtocolGraphQL,
			address:       baseURL,
			graphqlPath:   path,
			graphqlOpName: operationName,
			graphqlQuery:  query,
		}
	}
}

// WithSOAPEndpoint configures a SOAP fuzz target.
func WithSOAPEndpoint(baseURL, path, soapAction string) Option {
	return func(t *Target) {
		t.endpoint = endpoint{
			protocol: ProtocolSOAP,
			address:  baseURL,
			path:     path,
			method:   "POST",
			headers: map[string]string{
				"SOAPAction":   soapAction,
				"Content-Type": "text/xml; charset=utf-8",
			},
		}
	}
}

// WithWebSocketEndpoint configures a WebSocket fuzz target.
//
//	url  — full ws:// or wss:// URL (path included).
func WithWebSocketEndpoint(wsURL string) Option {
	return func(t *Target) {
		t.endpoint = endpoint{
			protocol: ProtocolWebSocket,
			address:  wsURL,
		}
	}
}

// WithKafkaEndpoint configures a Kafka fuzz target.
//
//	brokers  — comma-separated bootstrap broker list ("host1:9092,host2:9092").
//	topic    — destination topic name.
func WithKafkaEndpoint(brokers, topic string) Option {
	return func(t *Target) {
		t.endpoint = endpoint{
			protocol: ProtocolKafka,
			address:  brokers,
			path:     topic,
		}
	}
}

// WithRabbitMQEndpoint configures a RabbitMQ fuzz target.
//
//	amqpURL    — AMQP connection URL ("amqp://user:pw@host:5672/").
//	exchange   — exchange name (empty for default exchange).
//	routingKey — routing key (or queue name when using the default exchange).
func WithRabbitMQEndpoint(amqpURL, exchange, routingKey string) Option {
	return func(t *Target) {
		// Pack exchange + routing key into path with a separator the
		// transpiler decodes.  Keeps endpoint struct flat without growing
		// rabbit-only fields.
		t.endpoint = endpoint{
			protocol: ProtocolRabbitMQ,
			address:  amqpURL,
			path:     exchange + "::" + routingKey,
		}
	}
}

// WithHeader adds a single transport header (HTTP, gRPC metadata, etc.).
// Calling it more than once accumulates; the same key wins last-write.
func WithHeader(name, value string) Option {
	return func(t *Target) {
		if t.endpoint.headers == nil {
			t.endpoint.headers = make(map[string]string, 1)
		}
		t.endpoint.headers[name] = value
	}
}

// WithAuthHeader is a shorthand for setting the Authorization header
// (e.g. "Bearer …", "Basic …").
func WithAuthHeader(value string) Option { return WithHeader("Authorization", value) }
