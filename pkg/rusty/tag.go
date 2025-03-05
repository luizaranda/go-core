package rusty

import (
	"errors"
	"fmt"
	"reflect"
)

// getParams will extract the values from the fields of the struct v to be used as parameters.
// The field should be considered as a parameter if it has the tag "param" or is exported in which case
// the field name will be used as the parameter name.
// The field will be ignored if it has the tag "param" with the value "-".
// The field values will be converted to string using the function toString.
func getParams(value any) map[string]string {
	if value == nil {
		panic("value is nil")
	}

	rv, err := reflectValue(value)
	if err != nil {
		panic(fmt.Errorf("failed to obtain reflect value: %v", err))
	}

	params := make(map[string]string)
	for i := 0; i < rv.NumField(); i++ {
		field := rv.Type().Field(i)
		if !field.IsExported() {
			continue
		}

		tag := field.Tag.Get("param")
		if tag == "-" {
			continue
		}

		if tag == "" {
			tag = field.Name
		}

		v := rv.Field(i).Interface()
		params[tag] = toString(v)
	}

	return params
}

// reflectValue will obtain the [reflect.Value] of v only if it is a struct or a pointer to a struct.
// If it is a pointer to a struct, it will dereference it and return the [reflect.Value] of the struct.
// Otherwise, it will return an error.
func reflectValue(v any) (reflect.Value, error) {
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}

	if rv.Kind() != reflect.Struct {
		return reflect.Value{}, errors.New("value is not a struct or a pointer to a struct")
	}

	return rv, nil
}
