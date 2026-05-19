// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

package pact

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Message-pact DSL — consumer-side declaration of async/message
// contracts (Kafka events, AMQP messages, SNS notifications, NATS
// subjects, etc.).
//
// Mirrors the V4 "Asynchronous/Messages" interaction type from the
// pact-foundation spec so files produced here verify against any
// pact-compatible message verifier (pact-go, pact-jvm, pact-python,
// our own Verifier below).
//
// Consumer flow (run inside a unit test):
//
//	mp := pact.NewMessagePact("OrderConsumer", "OrderEvents")
//	mp.Given("user 42 exists").
//	    ExpectsToReceive("an order-created event").
//	    WithMetadata(map[string]string{"topic": "orders"}).
//	    WithContent(map[string]any{
//	        "orderId": pact.Like(42),
//	        "status":  pact.Regex("^(open|closed)$", "open"),
//	    })
//
//	// Verify the consumer's real message-handler can decode the
//	// example bytes that the contract advertises.
//	if err := mp.Verify(handleOrderEvent); err != nil { t.Fatal(err) }
//	_, _ = mp.WriteFile("./pacts")
//
// Provider flow uses the `Verifier` extension below — register a
// MessageProducer per interaction description, the verifier asks the
// producer for the actual bytes, matches against the recorded shape.

// MessageInteractionType is the V4 discriminator.
const MessageInteractionType = "Asynchronous/Messages"

// MessagePact is one consumer ↔ message-provider contract.
type MessagePact struct {
	consumer    string
	provider    string
	specVersion SpecVersion
	messages    []Message
}

// Message is a single expected message.
type Message struct {
	Contents    any
	Metadata    map[string]string
	States      []ProviderState
	Description string
	ContentType string
}

// MessageBuilder is the fluent DSL handle.
type MessageBuilder struct {
	pact *MessagePact
	msg  *Message
}

// NewMessagePact starts a new message-pact for the given consumer +
// provider names.
func NewMessagePact(consumer, provider string) *MessagePact {
	return &MessagePact{
		consumer:    consumer,
		provider:    provider,
		specVersion: SpecV4,
	}
}

// WithSpecVersion lets the caller force a different on-disk spec.
// Default is V4 (only spec that defines Asynchronous/Messages
// natively); V3 emits the legacy `messages` array shape.
func (p *MessagePact) WithSpecVersion(v SpecVersion) *MessagePact {
	p.specVersion = v
	return p
}

// Given starts a new message with a provider state.
func (p *MessagePact) Given(state string) *MessageBuilder {
	m := &Message{States: []ProviderState{{Name: state}}}
	p.messages = append(p.messages, *m)
	// Keep a stable pointer to the just-appended slot — append may
	// have reallocated; index the live slice.
	return &MessageBuilder{pact: p, msg: &p.messages[len(p.messages)-1]}
}

// GivenWithParams is Given with a parametrised state.
func (p *MessagePact) GivenWithParams(state string, params map[string]any) *MessageBuilder {
	m := Message{States: []ProviderState{{Name: state, Params: params}}}
	p.messages = append(p.messages, m)
	return &MessageBuilder{pact: p, msg: &p.messages[len(p.messages)-1]}
}

// ExpectsToReceive attaches the human description of the message.
func (b *MessageBuilder) ExpectsToReceive(description string) *MessageBuilder {
	b.msg.Description = description
	return b
}

// WithMetadata attaches the on-wire metadata (Kafka headers, AMQP
// properties, SNS attributes).
func (b *MessageBuilder) WithMetadata(meta map[string]string) *MessageBuilder {
	if b.msg.Metadata == nil {
		b.msg.Metadata = map[string]string{}
	}
	for k, v := range meta {
		b.msg.Metadata[k] = v
	}
	return b
}

// WithContent sets the expected body. Matchers (Like/Regex/EachLike)
// inside `body` are supported just like HTTP responses — the writer
// extracts them into `matchingRules` at serialise time.
func (b *MessageBuilder) WithContent(body any) *MessageBuilder {
	b.msg.Contents = body
	if b.msg.ContentType == "" {
		b.msg.ContentType = "application/json"
	}
	return b
}

// WithContentType overrides the default `application/json`.
func (b *MessageBuilder) WithContentType(ct string) *MessageBuilder {
	b.msg.ContentType = ct
	return b
}

// ToJSON renders the pact file to bytes.
func (p *MessagePact) ToJSON() ([]byte, error) {
	if p.consumer == "" || p.provider == "" {
		return nil, errors.New("pact: MessagePact requires consumer + provider")
	}
	doc := map[string]any{
		"consumer": map[string]any{"name": p.consumer},
		"provider": map[string]any{"name": p.provider},
		"metadata": map[string]any{
			"pactSpecification": map[string]any{"version": string(p.specVersion)},
			"mockarty":          map[string]any{"role": "messageConsumer"},
		},
	}
	switch p.specVersion {
	case SpecV4:
		doc["interactions"] = serialiseMessagesV4(p.messages)
	default:
		doc["messages"] = serialiseMessagesV3(p.messages)
	}
	return json.MarshalIndent(doc, "", "  ")
}

// WriteFile renders + writes the pact to disk under `dir`, returning
// the absolute path. Creates `dir` if missing.
func (p *MessagePact) WriteFile(dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	body, err := p.ToJSON()
	if err != nil {
		return "", err
	}
	name := safeFilename(strings.ToLower(p.consumer)) + "-" +
		safeFilename(strings.ToLower(p.provider)) + ".json"
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, body, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// MessageHandler is the consumer's real callback — invoked by Verify
// with the example bytes the pact advertises. Returning a non-nil
// error means "the consumer cannot process this contract".
type MessageHandler func(content []byte, metadata map[string]string) error

// Verify runs the consumer's handler against every recorded message's
// example bytes. Reports the first handler that rejects the example.
func (p *MessagePact) Verify(handler MessageHandler) error {
	if handler == nil {
		return errors.New("pact: Verify requires a non-nil MessageHandler")
	}
	for _, m := range p.messages {
		resolved := stripMatchersForBody(m.Contents)
		raw, err := encodeMessageBody(resolved, m.ContentType)
		if err != nil {
			return fmt.Errorf("encode %q: %w", m.Description, err)
		}
		if err := handler(raw, m.Metadata); err != nil {
			return fmt.Errorf("consumer rejected %q: %w", m.Description, err)
		}
	}
	return nil
}

// ----------------------------------------------------------------------
// Provider-side message verification — extends Verifier
// ----------------------------------------------------------------------

// MessageProducer is the provider-side callback registered per
// interaction description. The verifier asks the producer for the
// actual bytes the provider would publish given the interaction's
// provider state(s), then matches them against the expected contents.
type MessageProducer func(ctx context.Context, description string,
	states []ProviderState) (body []byte, metadata map[string]string, err error)

// WithMessageProducer registers the per-description producer used by
// VerifyMessagePact* on a Verifier.
func WithMessageProducer(description string, fn MessageProducer) VerifierOption {
	return func(v *Verifier) {
		if v.messageProducers == nil {
			v.messageProducers = map[string]MessageProducer{}
		}
		v.messageProducers[description] = fn
	}
}

// VerifyMessagePactBytes parses a message-pact (V3 or V4) and matches
// each expected message against the bytes the registered producer
// returns.
func (v *Verifier) VerifyMessagePactBytes(ctx context.Context, raw []byte) (*VerificationResult, error) {
	msgs, err := parseMessagePactDoc(raw)
	if err != nil {
		return nil, err
	}
	return v.verifyMessages(ctx, msgs)
}

func (v *Verifier) verifyMessages(ctx context.Context, msgs []Message) (*VerificationResult, error) {
	res := &VerificationResult{Provider: v.providerName}
	for _, m := range msgs {
		ir := InteractionResult{Description: m.Description}
		if len(m.States) > 0 {
			ir.State = m.States[0].Name
		}
		// Setup state — same path as HTTP verifier.
		if err := v.setUpStates(ctx, m.States); err != nil {
			ir.Error = "state setup: " + err.Error()
			res.Interactions = append(res.Interactions, ir)
			continue
		}
		producer, ok := v.messageProducers[m.Description]
		if !ok {
			ir.Error = "no MessageProducer registered for description " +
				strconv.Quote(m.Description)
			res.Interactions = append(res.Interactions, ir)
			continue
		}
		body, _, err := producer(ctx, m.Description, m.States)
		if err != nil {
			ir.Error = "producer: " + err.Error()
			res.Interactions = append(res.Interactions, ir)
			continue
		}
		ir.Mismatches = bodyMismatches(m.Contents, body)
		ir.Passed = len(ir.Mismatches) == 0
		res.Interactions = append(res.Interactions, ir)
	}
	return res, nil
}

// ----------------------------------------------------------------------
// serialisation
// ----------------------------------------------------------------------

func serialiseMessagesV4(msgs []Message) []map[string]any {
	out := make([]map[string]any, 0, len(msgs))
	for _, m := range msgs {
		ix := map[string]any{
			"type":        MessageInteractionType,
			"description": m.Description,
		}
		if len(m.States) > 0 {
			states := make([]map[string]any, 0, len(m.States))
			for _, st := range m.States {
				row := map[string]any{"name": st.Name}
				if len(st.Params) > 0 {
					row["params"] = st.Params
				}
				states = append(states, row)
			}
			ix["providerStates"] = states
		}
		contents := map[string]any{
			"contentType": m.ContentType,
		}
		rules := map[string]MatchCategory{}
		contents["content"] = walkAndExtract(m.Contents, "$.body", rules, SpecV4)
		if len(rules) > 0 {
			ix["matchingRules"] = formatMatchingRules(rules, SpecV4)
		}
		ix["contents"] = contents
		if len(m.Metadata) > 0 {
			ix["metadata"] = m.Metadata
		}
		out = append(out, ix)
	}
	return out
}

func serialiseMessagesV3(msgs []Message) []map[string]any {
	out := make([]map[string]any, 0, len(msgs))
	for _, m := range msgs {
		mr := map[string]any{
			"description": m.Description,
		}
		if len(m.States) == 1 {
			mr["providerState"] = m.States[0].Name
		} else if len(m.States) > 1 {
			// V3 'providerStates' plural is non-standard but tolerated
			// by many verifiers; the canonical V3 'providerState' is
			// singular.
			states := make([]map[string]any, 0, len(m.States))
			for _, st := range m.States {
				states = append(states, map[string]any{"name": st.Name})
			}
			mr["providerStates"] = states
		}
		rules := map[string]MatchCategory{}
		mr["contents"] = walkAndExtract(m.Contents, "$.body", rules, SpecV3)
		if len(rules) > 0 {
			mr["matchingRules"] = formatMatchingRules(rules, SpecV3)
		}
		if len(m.Metadata) > 0 {
			mr["metaData"] = m.Metadata
		}
		out = append(out, mr)
	}
	return out
}

func encodeMessageBody(body any, contentType string) ([]byte, error) {
	if body == nil {
		return nil, nil
	}
	switch v := body.(type) {
	case []byte:
		return v, nil
	case string:
		return []byte(v), nil
	}
	switch {
	case strings.Contains(strings.ToLower(contentType), "json"),
		contentType == "":
		return json.Marshal(body)
	}
	// Non-JSON, non-string body — best effort: still JSON-encode it.
	return json.Marshal(body)
}

// ----------------------------------------------------------------------
// parsing
// ----------------------------------------------------------------------

func parseMessagePactDoc(raw []byte) ([]Message, error) {
	var doc struct {
		Interactions []map[string]any `json:"interactions"`
		Messages     []map[string]any `json:"messages"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("pact: parse: %w", err)
	}
	var out []Message
	// V4 path — only Asynchronous/Messages entries are message pacts.
	for _, ix := range doc.Interactions {
		if t, _ := ix["type"].(string); t != MessageInteractionType {
			continue
		}
		out = append(out, decodeMessageV4(ix))
	}
	// V3 path.
	for _, mr := range doc.Messages {
		out = append(out, decodeMessageV3(mr))
	}
	return out, nil
}

func decodeMessageV4(ix map[string]any) Message {
	m := Message{}
	m.Description, _ = ix["description"].(string)
	m.States = decodeStates(ix)
	if c, ok := ix["contents"].(map[string]any); ok {
		m.ContentType, _ = c["contentType"].(string)
		m.Contents = c["content"]
	}
	if meta, ok := ix["metadata"].(map[string]any); ok {
		m.Metadata = stringMap(meta)
	}
	return m
}

func decodeMessageV3(mr map[string]any) Message {
	m := Message{}
	m.Description, _ = mr["description"].(string)
	m.States = decodeStates(mr)
	m.Contents = mr["contents"]
	if meta, ok := mr["metaData"].(map[string]any); ok {
		m.Metadata = stringMap(meta)
	} else if meta, ok := mr["metadata"].(map[string]any); ok {
		m.Metadata = stringMap(meta)
	}
	return m
}

func decodeStates(ix map[string]any) []ProviderState {
	var out []ProviderState
	if arr, ok := ix["providerStates"].([]any); ok {
		for _, s := range arr {
			if mm, ok := s.(map[string]any); ok {
				ps := ProviderState{}
				ps.Name, _ = mm["name"].(string)
				if p, ok := mm["params"].(map[string]any); ok {
					ps.Params = p
				}
				out = append(out, ps)
			}
		}
		return out
	}
	if s, ok := ix["providerState"].(string); ok && s != "" {
		out = append(out, ProviderState{Name: s})
	}
	return out
}

func stringMap(in map[string]any) map[string]string {
	out := map[string]string{}
	for k, v := range in {
		if s, ok := v.(string); ok {
			out[k] = s
		}
	}
	return out
}

// stub used elsewhere — ensure http isn't dropped by linters in case
// we extend producers to HTTP-fetch bodies later.
var _ = http.StatusOK
