package runtime

import (
	"encoding/json"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestEncodeProtoUsesTheConfiguredMarshaler(t *testing.T) {
	defer SetProtoMarshaler(protoMarshaler)

	msg, _ := structpb.NewStruct(map[string]any{"name": "x"})
	SetProtoMarshaler(func(proto.Message) ([]byte, error) {
		return []byte(`{"totalCount": 9007199254740993, "name": "x"}`), nil
	})
	out, err := EncodeProto(msg)
	if err != nil {
		t.Fatal(err)
	}
	if out["totalCount"] != json.Number("9007199254740993") {
		t.Fatalf("totalCount = %#v, want the exact number", out["totalCount"])
	}
	b, _ := json.Marshal(out)
	if string(b) != `{"name":"x","totalCount":9007199254740993}` {
		t.Fatalf("re-encoded as %s", b)
	}
}

func TestEncodeProtoDefaultsToProtojson(t *testing.T) {
	msg, _ := structpb.NewStruct(map[string]any{"name": "x"})
	out, err := EncodeProto(msg)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := protojson.Marshal(msg)
	got, _ := json.Marshal(out)
	var a, b any
	_ = json.Unmarshal(want, &a)
	_ = json.Unmarshal(got, &b)
	if string(mustJSON(a)) != string(mustJSON(b)) {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
