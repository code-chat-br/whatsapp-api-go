package validation

import (
	"errors"
	"reflect"
	"testing"
)

func TestValidatorFormatsValidationErrors(t *testing.T) {
	type sample struct {
		Name  string `validate:"required"`
		Age   int    `validate:"min=18"`
		Size  int    `validate:"max=10"`
		Role  string `validate:"oneof=admin user"`
		Email string `validate:"email"`
	}

	err := New().Validate(sample{
		Age:   17,
		Size:  11,
		Role:  "guest",
		Email: "invalid",
	})
	if err == nil {
		t.Fatal("expected validation error")
	}

	var validationErr ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected ValidationError, got %T", err)
	}

	want := []string{
		"name is required",
		"age must be greater than 18",
		"size must be less than 10",
		"role must be one of [admin user]",
		"email must be a valid email",
	}
	if !reflect.DeepEqual(validationErr.Messages, want) {
		t.Fatalf("unexpected messages:\nwant %#v\ngot  %#v", want, validationErr.Messages)
	}
}

func TestValidatorFormatsRequiredIf(t *testing.T) {
	type sample struct {
		Mode  string `validate:"oneof=email phone"`
		Email string `validate:"required_if=Mode email"`
	}

	err := New().Validate(sample{Mode: "email"})
	if err == nil {
		t.Fatal("expected validation error")
	}

	var validationErr ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected ValidationError, got %T", err)
	}

	want := []string{"email is required when Mode is email"}
	if !reflect.DeepEqual(validationErr.Messages, want) {
		t.Fatalf("unexpected messages: want %#v got %#v", want, validationErr.Messages)
	}
}
