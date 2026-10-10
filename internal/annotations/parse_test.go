package annotations

import (
	"reflect"
	"testing"

	mcpv1 "github.com/linkbreakers-com/grpc-mcp-gateway/mcp/gateway/v1"
	"google.golang.org/protobuf/proto"
)

func TestParseToolOptionsUI(t *testing.T) {
	raw, err := proto.Marshal(&mcpv1.Tool{
		Name:     "links_get",
		ReadOnly: true,
		Ui: &mcpv1.ToolUI{
			ResourceUri: "ui://example/link-card.html",
			Visibility:  []mcpv1.ToolVisibility{mcpv1.ToolVisibility_TOOL_VISIBILITY_MODEL, mcpv1.ToolVisibility_TOOL_VISIBILITY_APP},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	got := parseToolOptions(raw)
	if got.Name != "links_get" || !got.ReadOnly {
		t.Fatalf("base options not parsed: %+v", got)
	}
	want := &ToolUIOptions{ResourceURI: "ui://example/link-card.html", Visibility: []string{"model", "app"}}
	if !reflect.DeepEqual(got.UI, want) {
		t.Fatalf("ui = %+v, want %+v", got.UI, want)
	}
}

func TestParseToolOptionsWithoutUI(t *testing.T) {
	raw, err := proto.Marshal(&mcpv1.Tool{Name: "links_list"})
	if err != nil {
		t.Fatal(err)
	}
	if got := parseToolOptions(raw); got.UI != nil {
		t.Fatalf("ui = %+v, want nil", got.UI)
	}
}
