package service

import (
	"encoding/json"
	"strings"
)

func decodeJSONForTest(s string, v any) error {
	dec := json.NewDecoder(strings.NewReader(s))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}
