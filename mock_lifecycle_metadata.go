package mockarty

import "encoding/json"

// UnmarshalJSON reads both the list's root timestamp and GET's audit envelope.
// An explicit root value, including zero, wins for backward compatibility.
func (m *Mock) UnmarshalJSON(data []byte) error {
	type plainMock Mock
	decoded := plainMock(*m)
	wire := struct {
		*plainMock
		ClosedAt *int64 `json:"closedAt"`
		Metadata *struct {
			ClosedAt *int64 `json:"closedAt"`
		} `json:"_meta"`
	}{plainMock: &decoded}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	if wire.ClosedAt != nil {
		decoded.ClosedAt = *wire.ClosedAt
	} else if wire.Metadata != nil && wire.Metadata.ClosedAt != nil {
		decoded.ClosedAt = *wire.Metadata.ClosedAt
	}
	*m = Mock(decoded)
	return nil
}
