// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"encoding/json"
	"fmt"
	"strings"
)

// MarshalJSON keeps fields introduced by a newer server when callers update a
// saved performance configuration through an older SDK. Typed fields always
// win over an Extra entry with the same name.
func (p PerfConfig) MarshalJSON() ([]byte, error) {
	type wire PerfConfig
	return marshalWithExtra(wire(p), p.Extra, perfConfigKnownFields)
}

// UnmarshalJSON decodes the stable saved-config contract and retains unknown
// fields for a lossless GET -> model -> PUT round trip.
func (p *PerfConfig) UnmarshalJSON(data []byte) error {
	type wire PerfConfig
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	extra, err := unmarshalExtra(data, perfConfigKnownFields)
	if err != nil {
		return err
	}
	*p = PerfConfig(decoded)
	p.Extra = extra
	return nil
}

// MarshalJSON emits the canonical maxVUs spelling and preserves unknown
// options introduced by a newer server.
func (p PerfOptions) MarshalJSON() ([]byte, error) {
	type wire PerfOptions
	return marshalWithExtra(wire(p), p.Extra, perfOptionsKnownFields)
}

// UnmarshalJSON accepts the historical maxVus spelling on reads while always
// re-emitting maxVUs, and retains unknown options for future-server safety.
func (p *PerfOptions) UnmarshalJSON(data []byte) error {
	type wire PerfOptions
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("decode perf options object: %w", err)
	}
	// encoding/json matches object keys case-insensitively, so maxVus can
	// otherwise overwrite maxVUs according to document order. Re-decode the
	// selected spelling explicitly; canonical presence wins even when null.
	// When neither exact spelling exists, keep encoding/json's historical
	// case-insensitive result for backward compatibility.
	caseInsensitiveMaxVUs := decoded.MaxVUs
	decoded.MaxVUs = 0
	if canonical, ok := raw["maxVUs"]; ok {
		if err := json.Unmarshal(canonical, &decoded.MaxVUs); err != nil {
			return fmt.Errorf("decode canonical maxVUs: %w", err)
		}
	} else if legacy, ok := raw["maxVus"]; ok {
		if err := json.Unmarshal(legacy, &decoded.MaxVUs); err != nil {
			return fmt.Errorf("decode legacy maxVus: %w", err)
		}
	} else {
		decoded.MaxVUs = caseInsensitiveMaxVUs
	}
	extra, err := unmarshalExtra(data, perfOptionsKnownFields)
	if err != nil {
		return err
	}
	*p = PerfOptions(decoded)
	p.Extra = extra
	return nil
}

// MarshalJSON preserves fields added by newer servers without allowing the
// mutable Extra map to impersonate a typed field that happened to be omitted.
func (p PerfStage) MarshalJSON() ([]byte, error) {
	type wire PerfStage
	return marshalWithExtra(wire(p), p.Extra, perfStageKnownFields)
}

func (p *PerfStage) UnmarshalJSON(data []byte) error {
	type wire PerfStage
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	extra, err := unmarshalExtra(data, perfStageKnownFields)
	if err != nil {
		return err
	}
	*p = PerfStage(decoded)
	p.Extra = extra
	return nil
}

// MarshalJSON preserves fields added by newer servers without allowing the
// mutable Extra map to impersonate a typed field that happened to be omitted.
func (p AbortCriterion) MarshalJSON() ([]byte, error) {
	type wire AbortCriterion
	return marshalWithExtra(wire(p), p.Extra, abortCriterionKnownFields)
}

func (p *AbortCriterion) UnmarshalJSON(data []byte) error {
	type wire AbortCriterion
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	extra, err := unmarshalExtra(data, abortCriterionKnownFields)
	if err != nil {
		return err
	}
	*p = AbortCriterion(decoded)
	p.Extra = extra
	return nil
}

func marshalWithExtra(value any, extra map[string]json.RawMessage, known map[string]struct{}) ([]byte, error) {
	encoded, err := json.Marshal(value)
	if err != nil || len(extra) == 0 {
		return encoded, err
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &doc); err != nil {
		return nil, fmt.Errorf("decode typed JSON before merging extras: %w", err)
	}
	for key, raw := range extra {
		if isKnownJSONName(key, known) {
			continue
		}
		if _, typed := doc[key]; !typed {
			doc[key] = raw
		}
	}
	return json.Marshal(doc)
}

func unmarshalExtra(data []byte, known map[string]struct{}) (map[string]json.RawMessage, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("decode JSON object for extras: %w", err)
	}
	for key := range raw {
		if isKnownJSONName(key, known) {
			delete(raw, key)
		}
	}
	if len(raw) == 0 {
		return nil, nil
	}
	return raw, nil
}

func isKnownJSONName(candidate string, known map[string]struct{}) bool {
	if _, ok := known[candidate]; ok {
		return true
	}
	for name := range known {
		if strings.EqualFold(candidate, name) {
			return true
		}
	}
	return false
}

var perfConfigKnownFields = map[string]struct{}{
	"options": {}, "parentId": {}, "createdAt": {}, "updatedAt": {}, "environment": {},
	"thresholds": {}, "metadata": {}, "tags": {}, "collectionId": {}, "namespace": {},
	"userId": {}, "id": {}, "name": {}, "script": {}, "duration": {}, "sortOrder": {},
	"vus": {}, "iterations": {}, "isFolder": {},
}

var perfOptionsKnownFields = map[string]struct{}{
	"thresholds": {}, "duration": {}, "stages": {}, "abortCriteria": {}, "startAtUnixMs": {},
	"vus": {}, "iterations": {}, "rps": {}, "maxVUs": {}, "maxVus": {}, "arrivalRate": {},
	"gracefulStop": {}, "gracefulRampDown": {}, "metricsPush": {}, "metricsPushInterval": {},
	"emitHistograms": {},
}

var perfStageKnownFields = map[string]struct{}{
	"duration": {}, "target": {}, "targetRPS": {},
}

var abortCriterionKnownFields = map[string]struct{}{
	"metric": {}, "stat": {}, "condition": {}, "duration": {}, "name": {}, "value": {}, "enabled": {},
}
