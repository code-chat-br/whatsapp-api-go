package webhook

import (
	"encoding"
	"encoding/base64"
	"fmt"
	"reflect"
	"strings"
	"time"
)

type EventNormalizer interface {
	ToJSONMap(value any) (map[string]any, error)
}

type JSONMapNormalizer struct{}

func NewEventNormalizer() JSONMapNormalizer {
	return JSONMapNormalizer{}
}

func (JSONMapNormalizer) ToJSONMap(value any) (map[string]any, error) {
	normalized, err := normalizeJSONValue(reflect.ValueOf(value))
	if err != nil {
		return nil, err
	}
	output, ok := normalized.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("normalized value is %T, not map[string]any", normalized)
	}
	return output, nil
}

func MergeEventData(eventType string, source map[string]any, dateTime time.Time) map[string]any {
	output := make(map[string]any, len(source)+2)
	for key, value := range source {
		output[key] = value
	}
	if eventType != "" {
		output["type"] = eventType
	}
	output["dateTime"] = dateTime.UTC()
	return output
}

func normalizeJSONValue(value reflect.Value) (any, error) {
	if !value.IsValid() {
		return nil, nil
	}
	if value.Kind() == reflect.Pointer || value.Kind() == reflect.Interface {
		if value.IsNil() {
			return nil, nil
		}
		return normalizeJSONValue(value.Elem())
	}
	if value.CanInterface() {
		if t, ok := value.Interface().(time.Time); ok {
			if t.IsZero() {
				return time.Time{}, nil
			}
			return t.UTC(), nil
		}
		if marshaler, ok := value.Interface().(encoding.TextMarshaler); ok {
			text, err := marshaler.MarshalText()
			if err != nil {
				return nil, err
			}
			return string(text), nil
		}
	}
	switch value.Kind() {
	case reflect.Struct:
		return normalizeStruct(value)
	case reflect.Map:
		output := make(map[string]any, value.Len())
		iter := value.MapRange()
		for iter.Next() {
			key := fmt.Sprint(iter.Key().Interface())
			normalized, err := normalizeJSONValue(iter.Value())
			if err != nil {
				return nil, err
			}
			output[key] = normalized
		}
		return output, nil
	case reflect.Slice, reflect.Array:
		if value.Type().Elem().Kind() == reflect.Uint8 {
			data := make([]byte, value.Len())
			reflect.Copy(reflect.ValueOf(data), value)
			return base64.StdEncoding.EncodeToString(data), nil
		}
		output := make([]any, 0, value.Len())
		for i := 0; i < value.Len(); i++ {
			normalized, err := normalizeJSONValue(value.Index(i))
			if err != nil {
				return nil, err
			}
			output = append(output, normalized)
		}
		return output, nil
	case reflect.Bool:
		return value.Bool(), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return value.Int(), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return value.Uint(), nil
	case reflect.Float32, reflect.Float64:
		return value.Float(), nil
	case reflect.String:
		return value.String(), nil
	default:
		if value.CanInterface() {
			return value.Interface(), nil
		}
		return nil, nil
	}
}

func normalizeStruct(value reflect.Value) (map[string]any, error) {
	output := make(map[string]any)
	t := value.Type()
	for i := 0; i < value.NumField(); i++ {
		field := t.Field(i)
		if field.PkgPath != "" {
			continue
		}
		name, omitEmpty, skip := jsonFieldName(field)
		if skip {
			continue
		}
		fieldValue := value.Field(i)
		if omitEmpty && fieldValue.IsZero() {
			continue
		}
		normalized, err := normalizeJSONValue(fieldValue)
		if err != nil {
			return nil, err
		}
		if field.Anonymous && field.Tag.Get("json") == "" {
			if embedded, ok := normalized.(map[string]any); ok {
				for key, value := range embedded {
					output[key] = value
				}
				continue
			}
		}
		output[name] = normalized
	}
	return output, nil
}

func jsonFieldName(field reflect.StructField) (string, bool, bool) {
	tag := field.Tag.Get("json")
	if tag == "-" {
		return "", false, true
	}
	if tag != "" {
		parts := strings.Split(tag, ",")
		name := parts[0]
		omitEmpty := false
		for _, part := range parts[1:] {
			if part == "omitempty" {
				omitEmpty = true
			}
		}
		if name != "" {
			return name, omitEmpty, false
		}
		return lowerCamelFieldName(field.Name), omitEmpty, false
	}
	return lowerCamelFieldName(field.Name), false, false
}

func lowerCamelFieldName(name string) string {
	replacer := strings.NewReplacer(
		"JID", "Jid",
		"LID", "Lid",
		"ID", "Id",
		"URL", "Url",
		"HTTP", "Http",
		"JSON", "Json",
	)
	name = replacer.Replace(name)
	if name == "" {
		return ""
	}
	return strings.ToLower(name[:1]) + name[1:]
}
