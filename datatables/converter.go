package datatables

import (
	"reflect"
	"strings"
)

// structToMapSlice converts a slice of structs into a slice of map[string]interface{}.
// It uses reflection to read exported fields and respects JSON struct tags.
//
// Supported struct tag formats:
//   - `json:"field_name"`: Uses "field_name" as the map key
//   - `json:"field_name,omitempty"`: Uses "field_name" (options are ignored)
//   - `json:"-"`: Field is excluded from output
//   - No tag: Uses the field name as-is
//
// Embedded (anonymous) struct fields, such as gorm.Model, are flattened into
// the parent map rather than nested under the embedded type's name, and the
// outer struct's own fields take precedence over anything an embedded type
// would otherwise contribute — matching encoding/json's behavior.
//
// Parameters:
//   - data: Pointer to a slice of structs (e.g., *[]User)
//
// Returns a slice of maps where each map represents one struct instance, and
// an error if data is not a pointer to a slice of structs (ErrInvalidData).
//
// Example:
//
//	type User struct {
//	    ID   int    `json:"id"`
//	    Name string `json:"name"`
//	}
//	users := []User{{ID: 1, Name: "John"}}
//	result, err := structToMapSlice(&users)
//	// result: [{"id": 1, "name": "John"}]
func structToMapSlice(data interface{}) ([]map[string]interface{}, error) {
	v := reflect.ValueOf(data)

	// Dereference pointer if necessary
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	// Validate that we have a slice
	if v.Kind() != reflect.Slice {
		return nil, ErrInvalidData
	}

	result := make([]map[string]interface{}, 0, v.Len())

	// Iterate through each element in the slice
	for i := 0; i < v.Len(); i++ {
		item := v.Index(i)

		// Dereference pointer if necessary
		if item.Kind() == reflect.Ptr {
			if item.IsNil() {
				result = append(result, map[string]interface{}{})
				continue
			}
			item = item.Elem()
		}

		if item.Kind() != reflect.Struct {
			return nil, ErrInvalidData
		}

		result = append(result, structToMap(item))
	}

	return result, nil
}

// structToMap converts a single struct value to a map[string]interface{}.
// It processes all exported fields, respects JSON tags, and flattens
// embedded struct fields via collectFields.
func structToMap(v reflect.Value) map[string]interface{} {
	m := make(map[string]interface{})
	collectFields(v, m)
	return m
}

// collectFields writes v's own directly-declared fields into m first, so
// they take precedence, then merges in any embedded struct's fields for
// keys not already present. Recursing after the direct fields are set means
// the shallowest declaration always wins on a name collision, the same rule
// encoding/json applies to embedded types.
func collectFields(v reflect.Value, m map[string]interface{}) {
	t := v.Type()

	for j := 0; j < v.NumField(); j++ {
		field := t.Field(j)
		if !field.IsExported() || field.Anonymous {
			continue
		}

		col := getFieldName(field)
		if col == "" {
			continue
		}

		if _, exists := m[col]; !exists {
			m[col] = v.Field(j).Interface()
		}
	}

	for j := 0; j < v.NumField(); j++ {
		field := t.Field(j)
		if !field.IsExported() || !field.Anonymous {
			continue
		}

		embedded := v.Field(j)
		if embedded.Kind() == reflect.Ptr {
			if embedded.IsNil() {
				continue
			}
			embedded = embedded.Elem()
		}

		if embedded.Kind() == reflect.Struct {
			collectFields(embedded, m)
		}
	}
}

// getFieldName extracts the field name from the JSON struct tag.
// Returns an empty string if the field should be excluded (json:"-").
func getFieldName(field reflect.StructField) string {
	jsonTag := field.Tag.Get("json")

	// If no JSON tag, use the field name
	if jsonTag == "" {
		return field.Name
	}

	// Handle json:"-" (exclude field)
	if jsonTag == "-" {
		return ""
	}

	// Extract field name (before comma, if present)
	// e.g., "field_name,omitempty" -> "field_name"
	parts := strings.Split(jsonTag, ",")
	name := strings.TrimSpace(parts[0])
	if name == "" {
		return field.Name
	}
	return name
}
