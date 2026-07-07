package types

import "encoding/json"

type OptionalField[T any] struct {
	Set   bool
	Value *T
}

func NullJSON() OptionalField[json.RawMessage] {
	value := json.RawMessage("null")
	return OptionalField[json.RawMessage]{Set: true, Value: &value}
}
