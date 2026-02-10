package main

import (
	"fmt"
	"sort"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

const (
	fieldBehaviorFieldNumber = 1052
	fieldBehaviorRequired    = 2
	fieldBehaviorOutputOnly  = 3
)

func getFieldBehaviors(field *protogen.Field) []int {
	opts := field.Desc.Options()
	if opts == nil {
		return nil
	}
	fopts, ok := opts.(*descriptorpb.FieldOptions)
	if !ok || fopts == nil {
		return nil
	}
	unknown := fopts.ProtoReflect().GetUnknown()
	var behaviors []int
	for len(unknown) > 0 {
		num, typ, n := protowire.ConsumeTag(unknown)
		if n < 0 {
			return behaviors
		}
		unknown = unknown[n:]
		switch typ {
		case protowire.VarintType:
			v, m := protowire.ConsumeVarint(unknown)
			if m < 0 {
				return behaviors
			}
			if num == fieldBehaviorFieldNumber {
				behaviors = append(behaviors, int(v))
			}
			unknown = unknown[m:]
		case protowire.Fixed32Type:
			_, m := protowire.ConsumeFixed32(unknown)
			if m < 0 {
				return behaviors
			}
			unknown = unknown[m:]
		case protowire.Fixed64Type:
			_, m := protowire.ConsumeFixed64(unknown)
			if m < 0 {
				return behaviors
			}
			unknown = unknown[m:]
		case protowire.BytesType:
			_, m := protowire.ConsumeBytes(unknown)
			if m < 0 {
				return behaviors
			}
			unknown = unknown[m:]
		default:
			return behaviors
		}
	}
	return behaviors
}

func isOutputOnly(field *protogen.Field) bool {
	for _, b := range getFieldBehaviors(field) {
		if b == fieldBehaviorOutputOnly {
			return true
		}
	}
	return false
}

func isRequired(field *protogen.Field) bool {
	for _, b := range getFieldBehaviors(field) {
		if b == fieldBehaviorRequired {
			return true
		}
	}
	return false
}

func buildMessageSchema(msg *protogen.Message, seen map[string]bool) map[string]any {
	properties := map[string]any{}
	var required []string

	for _, field := range msg.Fields {
		if isOutputOnly(field) {
			continue
		}

		jsonName := field.Desc.JSONName()
		schema := buildFieldSchema(field, seen)

		desc := normalizeComment(field.Comments.Leading.String())
		if desc != "" {
			schema["description"] = desc
		}

		properties[jsonName] = schema

		if isRequired(field) {
			required = append(required, jsonName)
		}
	}

	result := map[string]any{
		"type":                 "object",
		"properties":          properties,
		"additionalProperties": false,
	}
	if len(required) > 0 {
		result["required"] = required
	}
	return result
}

func buildFieldSchema(field *protogen.Field, seen map[string]bool) map[string]any {
	if field.Desc.IsMap() {
		valueField := field.Message.Fields[1]
		valueSchema := buildFieldSchema(valueField, seen)
		return map[string]any{
			"type":                 "object",
			"additionalProperties": valueSchema,
		}
	}

	if field.Desc.IsList() {
		elemSchema := buildScalarOrMessageSchema(field, seen)
		return map[string]any{
			"type":  "array",
			"items": elemSchema,
		}
	}

	return buildScalarOrMessageSchema(field, seen)
}

func buildScalarOrMessageSchema(field *protogen.Field, seen map[string]bool) map[string]any {
	switch field.Desc.Kind() {
	case protoreflect.BoolKind:
		return map[string]any{"type": "boolean"}
	case protoreflect.StringKind:
		return map[string]any{"type": "string"}
	case protoreflect.BytesKind:
		return map[string]any{"type": "string", "format": "byte"}
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		return map[string]any{"type": "integer", "format": "int32"}
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return map[string]any{"type": "integer", "format": "int64"}
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		return map[string]any{"type": "integer", "format": "int32"}
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return map[string]any{"type": "integer", "format": "int64"}
	case protoreflect.FloatKind:
		return map[string]any{"type": "number", "format": "float"}
	case protoreflect.DoubleKind:
		return map[string]any{"type": "number", "format": "double"}
	case protoreflect.EnumKind:
		return buildEnumSchema(field)
	case protoreflect.MessageKind, protoreflect.GroupKind:
		return buildNestedMessageSchema(field, seen)
	default:
		return map[string]any{}
	}
}

func buildEnumSchema(field *protogen.Field) map[string]any {
	enumDesc := field.Desc.Enum()
	values := enumDesc.Values()
	var enumVals []string
	for i := 0; i < values.Len(); i++ {
		v := values.Get(i)
		if v.Number() == 0 {
			continue
		}
		enumVals = append(enumVals, string(v.Name()))
	}
	if len(enumVals) == 0 {
		for i := 0; i < values.Len(); i++ {
			enumVals = append(enumVals, string(values.Get(i).Name()))
		}
	}
	return map[string]any{"type": "string", "enum": enumVals}
}

func buildNestedMessageSchema(field *protogen.Field, seen map[string]bool) map[string]any {
	fullName := string(field.Message.Desc.FullName())

	switch protoreflect.FullName(fullName) {
	case "google.protobuf.Timestamp":
		return map[string]any{"type": "string", "format": "date-time"}
	case "google.protobuf.Duration":
		return map[string]any{"type": "string"}
	case "google.protobuf.Struct":
		return map[string]any{"type": "object", "additionalProperties": true}
	case "google.protobuf.Value":
		return map[string]any{}
	case "google.protobuf.ListValue":
		return map[string]any{"type": "array"}
	case "google.protobuf.Empty":
		return map[string]any{"type": "object", "properties": map[string]any{}, "additionalProperties": false}
	case "google.protobuf.StringValue":
		return map[string]any{"type": "string"}
	case "google.protobuf.BoolValue":
		return map[string]any{"type": "boolean"}
	case "google.protobuf.Int32Value":
		return map[string]any{"type": "integer", "format": "int32"}
	case "google.protobuf.Int64Value":
		return map[string]any{"type": "integer", "format": "int64"}
	case "google.protobuf.UInt32Value":
		return map[string]any{"type": "integer", "format": "int32"}
	case "google.protobuf.UInt64Value":
		return map[string]any{"type": "integer", "format": "int64"}
	case "google.protobuf.FloatValue":
		return map[string]any{"type": "number", "format": "float"}
	case "google.protobuf.DoubleValue":
		return map[string]any{"type": "number", "format": "double"}
	case "google.protobuf.BytesValue":
		return map[string]any{"type": "string", "format": "byte"}
	}

	if seen[fullName] {
		return map[string]any{"type": "object", "additionalProperties": true}
	}

	seen[fullName] = true
	schema := buildMessageSchema(field.Message, seen)
	delete(seen, fullName)

	return schema
}

func emitSchemaField(g *protogen.GeneratedFile, fieldName string, schema map[string]any, indent string) {
	g.P(indent, fieldName, ": map[string]any{")
	emitMapEntries(g, schema, indent+"\t")
	g.P(indent, "},")
}

func emitMapEntries(g *protogen.GeneratedFile, m map[string]any, indent string) {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		emitValue(g, k, m[k], indent)
	}
}

func emitValue(g *protogen.GeneratedFile, key string, val any, indent string) {
	switch v := val.(type) {
	case string:
		g.P(indent, fmt.Sprintf("%q", key), ": ", fmt.Sprintf("%q", v), ",")
	case bool:
		g.P(indent, fmt.Sprintf("%q", key), ": ", fmt.Sprintf("%t", v), ",")
	case int:
		g.P(indent, fmt.Sprintf("%q", key), ": ", fmt.Sprintf("%d", v), ",")
	case float64:
		g.P(indent, fmt.Sprintf("%q", key), ": ", fmt.Sprintf("%g", v), ",")
	case []string:
		g.P(indent, fmt.Sprintf("%q", key), ": []string{")
		for _, s := range v {
			g.P(indent, "\t", fmt.Sprintf("%q", s), ",")
		}
		g.P(indent, "},")
	case []any:
		g.P(indent, fmt.Sprintf("%q", key), ": []any{")
		for _, item := range v {
			emitAnonymousValue(g, item, indent+"\t")
		}
		g.P(indent, "},")
	case map[string]any:
		g.P(indent, fmt.Sprintf("%q", key), ": map[string]any{")
		emitMapEntries(g, v, indent+"\t")
		g.P(indent, "},")
	}
}

func emitAnonymousValue(g *protogen.GeneratedFile, val any, indent string) {
	switch v := val.(type) {
	case string:
		g.P(indent, fmt.Sprintf("%q", v), ",")
	case bool:
		g.P(indent, fmt.Sprintf("%t", v), ",")
	case int:
		g.P(indent, fmt.Sprintf("%d", v), ",")
	case float64:
		g.P(indent, fmt.Sprintf("%g", v), ",")
	case map[string]any:
		g.P(indent, "map[string]any{")
		emitMapEntries(g, v, indent+"\t")
		g.P(indent, "},")
	}
}
