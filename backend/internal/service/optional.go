package service

import "encoding/json"

// Optional distinguishes a JSON field that is absent (Set false) from one
// sent as null (Set true, Value nil), so PATCH requests can clear values.
type Optional[T any] struct {
	Set   bool
	Value *T
}

// UnmarshalJSON implements json.Unmarshaler (called for null too).
func (o *Optional[T]) UnmarshalJSON(b []byte) error {
	o.Set = true
	if string(b) == "null" {
		o.Value = nil
		return nil
	}
	var v T
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	o.Value = &v
	return nil
}

// Some returns a set Optional holding v (tests and internal callers).
func Some[T any](v T) Optional[T] { return Optional[T]{Set: true, Value: &v} }
