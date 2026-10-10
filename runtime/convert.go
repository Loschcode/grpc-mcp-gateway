package runtime

import (
	"bytes"
	"encoding/json"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// ArgPreprocessor is a function that preprocesses arguments before decoding.
// This can be used to transform enum values or perform other custom conversions.
type ArgPreprocessor func(args map[string]any) map[string]any

var globalPreprocessor ArgPreprocessor

// SetArgPreprocessor sets a global argument preprocessor that will be called
// before decoding arguments in all DecodeArgs calls.
func SetArgPreprocessor(fn ArgPreprocessor) {
	globalPreprocessor = fn
}

// ProtoMarshaler encodes a tool's protobuf response as JSON.
type ProtoMarshaler func(msg proto.Message) ([]byte, error)

var protoMarshaler ProtoMarshaler = protojson.MarshalOptions{
	UseProtoNames:   false,
	EmitUnpopulated: false,
}.Marshal

// SetProtoMarshaler replaces the protojson encoding EncodeProto uses for every
// tool response, so a server can keep its MCP output identical to its other
// transports (for instance, 64-bit integers as JSON numbers).
func SetProtoMarshaler(fn ProtoMarshaler) {
	if fn == nil {
		panic("proto marshaler is nil")
	}
	protoMarshaler = fn
}

// DecodeArgs converts MCP tool arguments into a protobuf request message.
func DecodeArgs(args map[string]any, msg proto.Message) error {
	if msg == nil {
		return nil
	}
	if args == nil {
		args = map[string]any{}
	}

	// Apply global preprocessor if set
	if globalPreprocessor != nil {
		args = globalPreprocessor(args)
	}

	b, err := json.Marshal(args)
	if err != nil {
		return err
	}
	unmarshal := protojson.UnmarshalOptions{
		DiscardUnknown: false,
	}
	return unmarshal.Unmarshal(b, msg)
}

// EncodeProto converts a protobuf response message into a JSON-compatible map.
func EncodeProto(msg proto.Message) (map[string]any, error) {
	if msg == nil {
		return map[string]any{}, nil
	}
	b, err := protoMarshaler(msg)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var out map[string]any
	if err := dec.Decode(&out); err != nil {
		return nil, err
	}
	if out == nil {
		out = map[string]any{}
	}
	return out, nil
}
