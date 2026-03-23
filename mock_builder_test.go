// Copyright (c) 2024-2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

package mockarty

import (
	"encoding/json"
	"testing"
)

func TestMockBuilder_BasicFields(t *testing.T) {
	mock := NewMockBuilder().
		ID("test-mock-1").
		Namespace("production").
		ChainID("chain-1").
		PathPrefix("/api").
		ServerName("srv-1").
		Tags("users", "v2").
		FolderID("folder-1").
		TTL(3600).
		UseLimiter(100).
		Priority(10).
		Build()

	if mock.ID != "test-mock-1" {
		t.Errorf("ID: got %q", mock.ID)
	}
	if mock.Namespace != "production" {
		t.Errorf("Namespace: got %q", mock.Namespace)
	}
	if mock.ChainID != "chain-1" {
		t.Errorf("ChainID: got %q", mock.ChainID)
	}
	if mock.PathPrefix != "/api" {
		t.Errorf("PathPrefix: got %q", mock.PathPrefix)
	}
	if mock.ServerName != "srv-1" {
		t.Errorf("ServerName: got %q", mock.ServerName)
	}
	if len(mock.Tags) != 2 || mock.Tags[0] != "users" || mock.Tags[1] != "v2" {
		t.Errorf("Tags: got %v", mock.Tags)
	}
	if mock.FolderID != "folder-1" {
		t.Errorf("FolderID: got %q", mock.FolderID)
	}
	if mock.TTL != 3600 {
		t.Errorf("TTL: got %d", mock.TTL)
	}
	if mock.UseLimiter != 100 {
		t.Errorf("UseLimiter: got %d", mock.UseLimiter)
	}
	if mock.Priority != 10 {
		t.Errorf("Priority: got %d", mock.Priority)
	}
}

func TestMockBuilder_HTTP(t *testing.T) {
	mock := NewMockBuilder().
		ID("http-mock").
		HTTP(func(h *HTTPBuilder) {
			h.Route("/api/v2/users/:id").
				Method("GET").
				HeaderCondition("Authorization", AssertNotEmpty, nil).
				QueryCondition("fields", AssertContains, "name").
				BodyCondition("$.name", AssertEquals, "John").
				SortArrays()
		}).
		Response(func(r *ResponseBuilder) {
			r.Status(200).
				Header("Content-Type", "application/json").
				JSONBody(map[string]any{
					"id":    "$.pathParam.id",
					"name":  "$.fake.FirstName",
					"email": "$.fake.Email",
				}).
				Delay(100)
		}).
		Build()

	if mock.HTTP == nil {
		t.Fatal("HTTP context is nil")
	}
	if mock.HTTP.Route != "/api/v2/users/:id" {
		t.Errorf("Route: got %q", mock.HTTP.Route)
	}
	if mock.HTTP.HttpMethod != "GET" {
		t.Errorf("Method: got %q", mock.HTTP.HttpMethod)
	}
	if len(mock.HTTP.Headers) != 1 {
		t.Fatalf("expected 1 header condition, got %d", len(mock.HTTP.Headers))
	}
	if mock.HTTP.Headers[0].AssertAction != AssertNotEmpty {
		t.Errorf("header assert action: got %q", mock.HTTP.Headers[0].AssertAction)
	}
	if len(mock.HTTP.QueryParams) != 1 {
		t.Fatalf("expected 1 query condition, got %d", len(mock.HTTP.QueryParams))
	}
	if len(mock.HTTP.Conditions) != 1 {
		t.Fatalf("expected 1 body condition, got %d", len(mock.HTTP.Conditions))
	}
	if !mock.HTTP.ApplySortArray {
		t.Error("expected ApplySortArray=true")
	}

	if mock.Response == nil {
		t.Fatal("Response is nil")
	}
	if mock.Response.StatusCode != 200 {
		t.Errorf("StatusCode: got %d", mock.Response.StatusCode)
	}
	if mock.Response.Delay != 100 {
		t.Errorf("Delay: got %d", mock.Response.Delay)
	}
	if mock.Response.Headers["Content-Type"][0] != "application/json" {
		t.Errorf("Headers: got %v", mock.Response.Headers)
	}
}

func TestMockBuilder_GRPC(t *testing.T) {
	mock := NewMockBuilder().
		ID("grpc-mock").
		GRPC(func(g *GRPCBuilder) {
			g.Service("UserService").
				Method("GetUser").
				MethodType("unary").
				BodyCondition("$.userId", AssertEquals, "123").
				MetaCondition("authorization", AssertNotEmpty, nil)
		}).
		Response(func(r *ResponseBuilder) {
			r.JSONBody(map[string]any{"name": "John"})
		}).
		Build()

	if mock.GRPC == nil {
		t.Fatal("GRPC context is nil")
	}
	if mock.GRPC.Service != "UserService" {
		t.Errorf("Service: got %q", mock.GRPC.Service)
	}
	if mock.GRPC.Method != "GetUser" {
		t.Errorf("Method: got %q", mock.GRPC.Method)
	}
	if mock.GRPC.MethodType != "unary" {
		t.Errorf("MethodType: got %q", mock.GRPC.MethodType)
	}
	if len(mock.GRPC.Conditions) != 1 {
		t.Errorf("expected 1 body condition, got %d", len(mock.GRPC.Conditions))
	}
	if len(mock.GRPC.Meta) != 1 {
		t.Errorf("expected 1 meta condition, got %d", len(mock.GRPC.Meta))
	}
}

func TestMockBuilder_MCP(t *testing.T) {
	mock := NewMockBuilder().
		ID("mcp-mock").
		MCPConfig(func(m *MCPBuilderCtx) {
			m.Method("tools/call").
				Tool("calculate").
				Description("A calculator tool").
				BodyCondition("$.expression", AssertNotEmpty, nil).
				HeaderCondition("x-api-key", AssertNotEmpty, nil)
		}).
		Build()

	if mock.MCP == nil {
		t.Fatal("MCP context is nil")
	}
	if mock.MCP.Method != "tools/call" {
		t.Errorf("Method: got %q", mock.MCP.Method)
	}
	if mock.MCP.Tool != "calculate" {
		t.Errorf("Tool: got %q", mock.MCP.Tool)
	}
	if mock.MCP.Description != "A calculator tool" {
		t.Errorf("Description: got %q", mock.MCP.Description)
	}
}

func TestMockBuilder_SOAP(t *testing.T) {
	mock := NewMockBuilder().
		ID("soap-mock").
		SOAPConfig(func(s *SOAPBuilder) {
			s.Service("WeatherService").
				Method("GetWeather").
				Action("http://weather.example.com/GetWeather").
				Path("/api/weather").
				BodyCondition("$.city", AssertEquals, "Moscow").
				HeaderCondition("SOAPAction", AssertNotEmpty, nil)
		}).
		Build()

	if mock.SOAP == nil {
		t.Fatal("SOAP context is nil")
	}
	if mock.SOAP.Service != "WeatherService" {
		t.Errorf("Service: got %q", mock.SOAP.Service)
	}
	if mock.SOAP.Action != "http://weather.example.com/GetWeather" {
		t.Errorf("Action: got %q", mock.SOAP.Action)
	}
	if mock.SOAP.Path != "/api/weather" {
		t.Errorf("Path: got %q", mock.SOAP.Path)
	}
}

func TestMockBuilder_GraphQL(t *testing.T) {
	mock := NewMockBuilder().
		ID("graphql-mock").
		GraphQLConfig(func(g *GraphQLBuilder) {
			g.Operation("query").
				Field("user").
				TypeName("User").
				Path("/graphql").
				BodyCondition("$.id", AssertEquals, "1").
				HeaderCondition("Authorization", AssertNotEmpty, nil)
		}).
		Build()

	if mock.GraphQL == nil {
		t.Fatal("GraphQL context is nil")
	}
	if mock.GraphQL.Operation != "query" {
		t.Errorf("Operation: got %q", mock.GraphQL.Operation)
	}
	if mock.GraphQL.Field != "user" {
		t.Errorf("Field: got %q", mock.GraphQL.Field)
	}
	if mock.GraphQL.Type != "User" {
		t.Errorf("Type: got %q", mock.GraphQL.Type)
	}
}

func TestMockBuilder_SSE(t *testing.T) {
	mock := NewMockBuilder().
		ID("sse-mock").
		SSEConfig(func(s *SSEBuilder) {
			s.EventPath("/events/notifications").
				EventName("notification").
				Description("Real-time notifications").
				BodyCondition("$.userId", AssertNotEmpty, nil).
				HeaderCondition("Authorization", AssertNotEmpty, nil)
		}).
		Build()

	if mock.SSE == nil {
		t.Fatal("SSE context is nil")
	}
	if mock.SSE.EventPath != "/events/notifications" {
		t.Errorf("EventPath: got %q", mock.SSE.EventPath)
	}
	if mock.SSE.EventName != "notification" {
		t.Errorf("EventName: got %q", mock.SSE.EventName)
	}
}

func TestMockBuilder_Kafka(t *testing.T) {
	mock := NewMockBuilder().
		ID("kafka-mock").
		KafkaConfig(func(k *KafkaBuilder) {
			k.Topic("orders").
				ServerName("kafka-srv").
				ConsumerGroup("order-consumers").
				OutputTopic("order-responses").
				OutputBrokers("localhost:9092").
				OutputKey("$.orderId").
				OutputHeaders(map[string]string{"source": "mockarty"}).
				BodyCondition("$.type", AssertEquals, "new_order").
				HeaderCondition("x-source", AssertNotEmpty, nil)
		}).
		Build()

	if mock.Kafka == nil {
		t.Fatal("Kafka context is nil")
	}
	if mock.Kafka.Topic != "orders" {
		t.Errorf("Topic: got %q", mock.Kafka.Topic)
	}
	if mock.Kafka.OutputTopic != "order-responses" {
		t.Errorf("OutputTopic: got %q", mock.Kafka.OutputTopic)
	}
	if mock.Kafka.OutputHeaders["source"] != "mockarty" {
		t.Errorf("OutputHeaders: got %v", mock.Kafka.OutputHeaders)
	}
}

func TestMockBuilder_RabbitMQ(t *testing.T) {
	mock := NewMockBuilder().
		ID("rabbit-mock").
		RabbitMQConfig(func(r *RabbitMQBuilder) {
			r.Queue("tasks").
				Exchange("task-exchange").
				RoutingKey("task.created").
				ServerName("rabbit-srv").
				OutputURL("amqp://guest:guest@localhost:5672/").
				OutputExchange("responses").
				OutputRoutingKey("task.response").
				OutputQueue("response-queue").
				OutputProps(&RabbitMQOutputProps{
					DeliveryMode: 2,
					ContentType:  "application/json",
				}).
				BodyCondition("$.taskType", AssertEquals, "process").
				HeaderCondition("priority", AssertNotEmpty, nil)
		}).
		Build()

	if mock.RabbitMQ == nil {
		t.Fatal("RabbitMQ context is nil")
	}
	if mock.RabbitMQ.Queue != "tasks" {
		t.Errorf("Queue: got %q", mock.RabbitMQ.Queue)
	}
	if mock.RabbitMQ.OutputProps == nil || mock.RabbitMQ.OutputProps.DeliveryMode != 2 {
		t.Error("OutputProps not set correctly")
	}
}

func TestMockBuilder_SMTP(t *testing.T) {
	mock := NewMockBuilder().
		ID("smtp-mock").
		SMTPConfig(func(s *SMTPBuilder) {
			s.ServerName("smtp-srv").
				SenderCondition("$", AssertContains, "@example.com").
				RecipientCondition("$", AssertNotEmpty, nil).
				SubjectCondition("$", AssertContains, "Order").
				BodyCondition("$", AssertNotEmpty, nil).
				HeaderCondition("X-Priority", AssertEquals, "1")
		}).
		Build()

	if mock.SMTP == nil {
		t.Fatal("SMTP context is nil")
	}
	if mock.SMTP.ServerName != "smtp-srv" {
		t.Errorf("ServerName: got %q", mock.SMTP.ServerName)
	}
	if len(mock.SMTP.SenderConditions) != 1 {
		t.Errorf("expected 1 sender condition, got %d", len(mock.SMTP.SenderConditions))
	}
	if len(mock.SMTP.RecipientConditions) != 1 {
		t.Errorf("expected 1 recipient condition, got %d", len(mock.SMTP.RecipientConditions))
	}
	if len(mock.SMTP.SubjectConditions) != 1 {
		t.Errorf("expected 1 subject condition, got %d", len(mock.SMTP.SubjectConditions))
	}
	if len(mock.SMTP.BodyConditions) != 1 {
		t.Errorf("expected 1 body condition, got %d", len(mock.SMTP.BodyConditions))
	}
	if len(mock.SMTP.HeaderConditions) != 1 {
		t.Errorf("expected 1 header condition, got %d", len(mock.SMTP.HeaderConditions))
	}
}

func TestMockBuilder_Socket(t *testing.T) {
	mock := NewMockBuilder().
		ID("socket-mock").
		SocketConfig(func(s *SocketBuilder) {
			s.ServerName("ws-srv").
				Event("message").
				SocketNamespace("/chat").
				BodyCondition("$.type", AssertEquals, "text")
		}).
		Build()

	if mock.Socket == nil {
		t.Fatal("Socket context is nil")
	}
	if mock.Socket.ServerName != "ws-srv" {
		t.Errorf("ServerName: got %q", mock.Socket.ServerName)
	}
	if mock.Socket.Event != "message" {
		t.Errorf("Event: got %q", mock.Socket.Event)
	}
	if mock.Socket.Namespace != "/chat" {
		t.Errorf("Namespace: got %q", mock.Socket.Namespace)
	}
}

func TestMockBuilder_OneOf(t *testing.T) {
	mock := NewMockBuilder().
		ID("oneof-mock").
		HTTP(func(h *HTTPBuilder) {
			h.Route("/api/random").Method("GET")
		}).
		OneOfConfig(OneOfOrderRandom,
			func(r *ResponseBuilder) {
				r.Status(200).JSONBody(map[string]any{"result": "a"})
			},
			func(r *ResponseBuilder) {
				r.Status(200).JSONBody(map[string]any{"result": "b"})
			},
			func(r *ResponseBuilder) {
				r.Status(500).Error("simulated error")
			},
		).
		Build()

	if mock.OneOf == nil {
		t.Fatal("OneOf is nil")
	}
	if mock.OneOf.Order != OneOfOrderRandom {
		t.Errorf("Order: got %q", mock.OneOf.Order)
	}
	if len(mock.OneOf.Responses) != 3 {
		t.Errorf("expected 3 responses, got %d", len(mock.OneOf.Responses))
	}
}

func TestMockBuilder_Proxy(t *testing.T) {
	mock := NewMockBuilder().
		ID("proxy-mock").
		HTTP(func(h *HTTPBuilder) {
			h.Route("/api/proxy").Method("GET")
		}).
		ProxyTo("https://real-service.example.com").
		Build()

	if mock.Proxy == nil {
		t.Fatal("Proxy is nil")
	}
	if mock.Proxy.Target != "https://real-service.example.com" {
		t.Errorf("Target: got %q", mock.Proxy.Target)
	}
}

func TestMockBuilder_Callbacks(t *testing.T) {
	mock := NewMockBuilder().
		ID("callback-mock").
		HTTP(func(h *HTTPBuilder) {
			h.Route("/api/order").Method("POST")
		}).
		Callbacks(
			Callback{
				Type:   CallbackTypeHTTP,
				URL:    "https://webhook.example.com/notify",
				Method: "POST",
				Headers: map[string]string{
					"X-Custom": "value",
				},
				Body:       map[string]any{"event": "order_created"},
				Trigger:    TriggerOnSuccess,
				RetryCount: 3,
			},
			Callback{
				Type:         CallbackTypeKafka,
				KafkaBrokers: "localhost:9092",
				KafkaTopic:   "events",
				Trigger:      TriggerAlways,
			},
		).
		Build()

	if len(mock.Callbacks) != 2 {
		t.Fatalf("expected 2 callbacks, got %d", len(mock.Callbacks))
	}
	if mock.Callbacks[0].Type != CallbackTypeHTTP {
		t.Errorf("first callback type: got %q", mock.Callbacks[0].Type)
	}
	if mock.Callbacks[1].Type != CallbackTypeKafka {
		t.Errorf("second callback type: got %q", mock.Callbacks[1].Type)
	}
}

func TestMockBuilder_Extract(t *testing.T) {
	mock := NewMockBuilder().
		ID("extract-mock").
		HTTP(func(h *HTTPBuilder) {
			h.Route("/api/data").Method("POST")
		}).
		ExtractConfig(&Extract{
			GStore: map[string]any{"counter": "$.body.count"},
			CStore: map[string]any{"lastOrder": "$.body.orderId"},
			MStore: map[string]any{"temp": "$.body.temp"},
		}).
		MockStore(map[string]any{"initial": "value"}).
		Build()

	if mock.Extract == nil {
		t.Fatal("Extract is nil")
	}
	if mock.Extract.GStore["counter"] != "$.body.count" {
		t.Errorf("GStore counter: got %v", mock.Extract.GStore["counter"])
	}
	if mock.MockStore["initial"] != "value" {
		t.Errorf("MockStore initial: got %v", mock.MockStore["initial"])
	}
}

func TestMockBuilder_ResponseFeatures(t *testing.T) {
	mock := NewMockBuilder().
		ID("response-features").
		HTTP(func(h *HTTPBuilder) {
			h.Route("/api/test").Method("GET")
		}).
		Response(func(r *ResponseBuilder) {
			r.Status(200).
				Decode(DecodeBase64).
				TemplatePath("/templates/response.json").
				Error("some error").
				ErrorDetailsList([]ErrorDetails{
					{Type: ErrorDetailsBadRequest, Details: map[string]interface{}{"field": "name"}},
				}).
				SSEChain(&SSEEventChain{
					Events: []SSEEvent{{Data: "test", Delay: 100}},
					Loop:   true,
				}).
				GraphQLErrorList([]GraphQLError{
					{Message: "field not found"},
				}).
				SOAPFaultConfig(&SOAPFault{FaultCode: "soap:Client", FaultString: "invalid"}).
				MCPIsError(true)
		}).
		Build()

	resp := mock.Response
	if resp.Decode != DecodeBase64 {
		t.Errorf("Decode: got %q", resp.Decode)
	}
	if resp.PayloadTemplatePath != "/templates/response.json" {
		t.Errorf("TemplatePath: got %q", resp.PayloadTemplatePath)
	}
	if resp.Error != "some error" {
		t.Errorf("Error: got %q", resp.Error)
	}
	if resp.ErrorDetails == nil || len(*resp.ErrorDetails) != 1 {
		t.Error("ErrorDetails not set")
	}
	if resp.SSEEventChain == nil {
		t.Error("SSEEventChain is nil")
	}
	if len(resp.GraphQLErrors) != 1 {
		t.Error("GraphQLErrors not set")
	}
	if resp.SOAPFault == nil {
		t.Error("SOAPFault is nil")
	}
	if !resp.MCPIsError {
		t.Error("MCPIsError should be true")
	}
}

func TestMockBuilder_JSONSerialization(t *testing.T) {
	mock := NewMockBuilder().
		ID("json-test").
		Namespace("test").
		HTTP(func(h *HTTPBuilder) {
			h.Route("/api/users/:id").
				Method("GET").
				HeaderCondition("Authorization", AssertNotEmpty, nil)
		}).
		Response(func(r *ResponseBuilder) {
			r.Status(200).
				Header("Content-Type", "application/json").
				JSONBody(map[string]any{
					"id":   "$.pathParam.id",
					"name": "$.fake.FirstName",
				})
		}).
		TTL(3600).
		Priority(5).
		Tags("users").
		Build()

	data, err := json.Marshal(mock)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// Verify key JSON tags
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if raw["id"] != "json-test" {
		t.Errorf("expected id=json-test in JSON")
	}
	if raw["namespace"] != "test" {
		t.Errorf("expected namespace=test in JSON")
	}
	if raw["ttl"] != float64(3600) {
		t.Errorf("expected ttl=3600 in JSON, got %v", raw["ttl"])
	}
	if raw["priority"] != float64(5) {
		t.Errorf("expected priority=5 in JSON, got %v", raw["priority"])
	}

	// Verify HTTP context uses correct JSON tags
	httpCtx, ok := raw["http"].(map[string]any)
	if !ok {
		t.Fatal("expected 'http' key in JSON")
	}
	if httpCtx["route"] != "/api/users/:id" {
		t.Errorf("expected http.route in JSON, got %v", httpCtx["route"])
	}
	if httpCtx["httpMethod"] != "GET" {
		t.Errorf("expected http.httpMethod in JSON, got %v", httpCtx["httpMethod"])
	}

	// Verify header conditions use "header" key (not "headers")
	if _, ok := httpCtx["header"]; !ok {
		t.Error("expected 'header' key (not 'headers') in HTTP context JSON")
	}
}

func TestMockBuilder_CallbackJSONTags(t *testing.T) {
	mock := NewMockBuilder().
		ID("callback-json").
		HTTP(func(h *HTTPBuilder) {
			h.Route("/test").Method("POST")
		}).
		Callbacks(Callback{
			Type:    CallbackTypeHTTP,
			URL:     "http://example.com",
			Trigger: TriggerOnSuccess,
		}).
		Build()

	data, err := json.Marshal(mock)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var raw map[string]any
	_ = json.Unmarshal(data, &raw)

	// Callbacks should be serialized under "webhooks" key
	if _, ok := raw["webhooks"]; !ok {
		t.Error("expected callbacks to be serialized as 'webhooks' in JSON")
	}
}

func TestMockBuilder_BuildReturnsCopy(t *testing.T) {
	builder := NewMockBuilder().ID("original")
	mock1 := builder.Build()
	mock2 := builder.ID("modified").Build()

	if mock1.ID == mock2.ID {
		t.Error("Build() should return a copy; modifying builder should not affect previous Build result")
	}
}

func TestMockBuilder_MCPResource(t *testing.T) {
	mock := NewMockBuilder().
		ID("mcp-resource").
		MCPConfig(func(m *MCPBuilderCtx) {
			m.Method("resources/read").
				Resource("file://config.json")
		}).
		Build()

	if mock.MCP.Resource != "file://config.json" {
		t.Errorf("Resource: got %q", mock.MCP.Resource)
	}
}
