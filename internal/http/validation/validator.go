package validation

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
)

type RequestValidator interface {
	Validate(value any) error
}

type Validator struct {
	validate *validator.Validate
}

type ValidationError struct {
	Messages []string
}

func (e ValidationError) Error() string {
	return strings.Join(e.Messages, "; ")
}

func New() *Validator {
	validate := validator.New(validator.WithRequiredStructEnabled())
	validate.RegisterTagNameFunc(func(field reflect.StructField) string {
		name := strings.Split(field.Tag.Get("json"), ",")[0]
		if name == "" || name == "-" {
			return strings.ToLower(field.Name)
		}
		return name
	})
	return &Validator{validate: validate}
}

func (v *Validator) Validate(value any) error {
	err := v.validate.Struct(value)
	if err == nil {
		return nil
	}

	validateErrors, ok := err.(validator.ValidationErrors)
	if !ok {
		return err
	}

	messages := make([]string, len(validateErrors))
	for index, validationErr := range validateErrors {
		field := validationErr.Field()
		param := strings.Split(validationErr.Param(), " ")
		switch validationErr.Tag() {
		case "required":
			messages[index] = fmt.Sprintf("%s is required", field)
		case "min":
			messages[index] = fmt.Sprintf("%s must be greater than %s", field, validationErr.Param())
		case "max":
			messages[index] = fmt.Sprintf("%s must be less than %s", field, validationErr.Param())
		case "oneof":
			messages[index] = fmt.Sprintf("%s must be one of [%s]", field, validationErr.Param())
		case "uuid4":
			messages[index] = fmt.Sprintf("%s must be a valid uuid4", field)
		case "uuid7":
			messages[index] = fmt.Sprintf("%s must be a valid uuid7", field)
		case "required_if":
			if len(param) >= 2 {
				messages[index] = fmt.Sprintf("%s is required when %s is %s", field, param[0], param[1])
			} else {
				messages[index] = fmt.Sprintf("%s is required", field)
			}
		case "email":
			messages[index] = fmt.Sprintf("%s must be a valid email", field)
		case "url":
			messages[index] = fmt.Sprintf("%s must be a valid URL", field)
		default:
			messages[index] = fmt.Sprintf("%s %s", field, validationErr.Tag())
		}
	}

	return ValidationError{Messages: messages}
}
