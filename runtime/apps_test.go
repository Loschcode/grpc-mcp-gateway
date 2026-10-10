package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func rpc(t *testing.T, mux *MCPServeMux, method string, params map[string]any) map[string]any {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body)))
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("%s: decode: %v", method, err)
	}
	return resp
}

func newAppsMux() *MCPServeMux {
	mux := NewMCPServeMux(ServerMetadata{Name: "test", Version: "v0"})
	border := true
	mux.RegisterTool(&ToolHandler{
		Name: "links_get",
		UI:   &ToolUI{ResourceURI: "ui://test/link.html"},
		Handler: func(context.Context, map[string]any) (any, error) {
			return map[string]any{"id": "1"}, nil
		},
	})
	mux.RegisterTool(&ToolHandler{
		Name:    "links_list",
		Handler: func(context.Context, map[string]any) (any, error) { return nil, nil },
	})
	mux.RegisterResource(&Resource{
		URI:           "ui://test/link.html",
		Name:          "link_card",
		MIMEType:      AppResourceMIMEType,
		Text:          "<!DOCTYPE html><html></html>",
		PrefersBorder: &border,
		Meta:          map[string]any{"openai/widgetDescription": "A link card."},
	})
	return mux
}

func TestInitializeAdvertisesApps(t *testing.T) {
	resp := rpc(t, newAppsMux(), "initialize", nil)
	caps := resp["result"].(map[string]any)["capabilities"].(map[string]any)
	if _, ok := caps["resources"]; !ok {
		t.Fatal("resources capability missing")
	}
	ext := caps["extensions"].(map[string]any)[AppsExtensionID].(map[string]any)
	if !reflect.DeepEqual(ext["mimeTypes"], []any{AppResourceMIMEType}) {
		t.Fatalf("mimeTypes = %v", ext["mimeTypes"])
	}
}

func TestInitializeWithoutResources(t *testing.T) {
	mux := NewMCPServeMux(ServerMetadata{Name: "test", Version: "v0"})
	caps := rpc(t, mux, "initialize", nil)["result"].(map[string]any)["capabilities"].(map[string]any)
	if _, ok := caps["resources"]; ok {
		t.Fatal("resources capability advertised without resources")
	}
	if _, ok := caps["extensions"]; ok {
		t.Fatal("apps extension advertised without app resources")
	}
}

func TestToolsListCarriesUIMeta(t *testing.T) {
	tools := rpc(t, newAppsMux(), "tools/list", nil)["result"].(map[string]any)["tools"].([]any)
	byName := map[string]map[string]any{}
	for _, tool := range tools {
		m := tool.(map[string]any)
		byName[m["name"].(string)] = m
	}
	meta := byName["links_get"]["_meta"].(map[string]any)
	if meta["ui"].(map[string]any)["resourceUri"] != "ui://test/link.html" {
		t.Fatalf("ui.resourceUri = %v", meta["ui"])
	}
	if meta["ui/resourceUri"] != "ui://test/link.html" || meta["openai/outputTemplate"] != "ui://test/link.html" {
		t.Fatalf("compat keys = %v", meta)
	}
	if _, ok := byName["links_list"]["_meta"]; ok {
		t.Fatal("tool without ui must not carry _meta")
	}
}

func TestResourcesListAndRead(t *testing.T) {
	mux := newAppsMux()
	list := rpc(t, mux, "resources/list", nil)["result"].(map[string]any)["resources"].([]any)
	if len(list) != 1 {
		t.Fatalf("resources = %v", list)
	}
	entry := list[0].(map[string]any)
	if entry["uri"] != "ui://test/link.html" || entry["mimeType"] != AppResourceMIMEType {
		t.Fatalf("entry = %v", entry)
	}

	read := rpc(t, mux, "resources/read", map[string]any{"uri": "ui://test/link.html"})
	content := read["result"].(map[string]any)["contents"].([]any)[0].(map[string]any)
	if content["text"] != "<!DOCTYPE html><html></html>" || content["mimeType"] != AppResourceMIMEType {
		t.Fatalf("content = %v", content)
	}
	meta := content["_meta"].(map[string]any)
	if meta["ui"].(map[string]any)["prefersBorder"] != true || meta["openai/widgetDescription"] != "A link card." {
		t.Fatalf("meta = %v", meta)
	}
	if _, ok := meta["ui"].(map[string]any)["csp"]; ok {
		t.Fatal("csp must be omitted when not declared")
	}

	missing := rpc(t, mux, "resources/read", map[string]any{"uri": "ui://test/nope.html"})
	if code := missing["error"].(map[string]any)["code"]; code != float64(-32002) {
		t.Fatalf("missing resource code = %v", code)
	}

	templates := rpc(t, mux, "resources/templates/list", nil)["result"].(map[string]any)["resourceTemplates"].([]any)
	if len(templates) != 0 {
		t.Fatalf("templates = %v", templates)
	}
}

func TestMissingUIResources(t *testing.T) {
	mux := NewMCPServeMux(ServerMetadata{Name: "test", Version: "v0"})
	mux.RegisterTool(&ToolHandler{Name: "a", UI: &ToolUI{ResourceURI: "ui://test/a.html"}})
	if got := mux.MissingUIResources(); !reflect.DeepEqual(got, []string{"ui://test/a.html"}) {
		t.Fatalf("missing = %v", got)
	}
	mux.RegisterResource(&Resource{URI: "ui://test/a.html", Name: "a", MIMEType: AppResourceMIMEType})
	if got := mux.MissingUIResources(); len(got) != 0 {
		t.Fatalf("missing = %v", got)
	}
}

func TestRegisterAppResourceRequiresUIScheme(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for non ui:// app resource")
		}
	}()
	NewMCPServeMux(ServerMetadata{}).RegisterResource(&Resource{URI: "https://x", Name: "x", MIMEType: AppResourceMIMEType})
}
