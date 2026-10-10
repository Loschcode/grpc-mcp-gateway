package runtime

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
)

// MCP Apps (SEP-1865) identifiers.
const (
	AppsExtensionID      = "io.modelcontextprotocol/ui"
	AppResourceMIMEType  = "text/html;profile=mcp-app"
	appResourceURIScheme = "ui://"
)

// ToolUI links a tool to the ui:// resource that renders its result.
type ToolUI struct {
	ResourceURI string
	// Visibility lists "model" and/or "app". Empty means both.
	Visibility []string
}

// ResourceCSP declares the external origins a UI resource needs. Leave it nil
// when the view loads nothing external, so hosts apply their strict default.
type ResourceCSP struct {
	ConnectDomains  []string
	ResourceDomains []string
	FrameDomains    []string
	BaseURIDomains  []string
}

// Resource is a static MCP resource, typically an MCP Apps ui:// view.
type Resource struct {
	URI         string
	Name        string
	Title       string
	Description string
	MIMEType    string
	Text        string
	CSP         *ResourceCSP
	// PrefersBorder is sent as _meta.ui.prefersBorder when set.
	PrefersBorder *bool
	// Meta is merged into the resource _meta, e.g. host-specific keys such as
	// openai/widgetDescription.
	Meta map[string]any
}

// RegisterResource registers a resource served by resources/list and
// resources/read. App resources must use the ui:// scheme.
func (mux *MCPServeMux) RegisterResource(res *Resource) {
	if res == nil || res.URI == "" {
		panic("mcp resource requires a uri")
	}
	if res.MIMEType == AppResourceMIMEType && !strings.HasPrefix(res.URI, appResourceURIScheme) {
		panic(fmt.Sprintf("mcp app resource %q must use the ui:// scheme", res.URI))
	}
	mux.mu.Lock()
	defer mux.mu.Unlock()
	mux.resources[res.URI] = res
}

// MissingUIResources returns the ui:// URIs that registered tools reference
// but no registered resource serves. Call it at startup: hosts fail to render
// a tool whose view does not exist.
func (mux *MCPServeMux) MissingUIResources() []string {
	mux.mu.RLock()
	defer mux.mu.RUnlock()
	seen := map[string]bool{}
	var missing []string
	for _, tool := range mux.tools {
		if tool.UI == nil || tool.UI.ResourceURI == "" {
			continue
		}
		uri := tool.UI.ResourceURI
		if _, ok := mux.resources[uri]; ok || seen[uri] {
			continue
		}
		seen[uri] = true
		missing = append(missing, uri)
	}
	sort.Strings(missing)
	return missing
}

func (mux *MCPServeMux) hasAppResources() bool {
	for _, res := range mux.resources {
		if res.MIMEType == AppResourceMIMEType {
			return true
		}
	}
	return false
}

func toolMeta(ui *ToolUI) map[string]any {
	if ui == nil || ui.ResourceURI == "" {
		return nil
	}
	uiMeta := map[string]any{"resourceUri": ui.ResourceURI}
	if len(ui.Visibility) > 0 {
		uiMeta["visibility"] = ui.Visibility
	}
	return map[string]any{
		"ui": uiMeta,
		// Deprecated flat key, still read by hosts built before the nested form.
		"ui/resourceUri": ui.ResourceURI,
		// ChatGPT (OpenAI Apps SDK) alias of ui.resourceUri.
		"openai/outputTemplate": ui.ResourceURI,
	}
}

func resourceMeta(res *Resource) map[string]any {
	meta := map[string]any{}
	for k, v := range res.Meta {
		meta[k] = v
	}
	uiMeta := map[string]any{}
	if res.CSP != nil {
		csp := map[string]any{}
		addDomains := func(key string, domains []string) {
			if len(domains) > 0 {
				csp[key] = domains
			}
		}
		addDomains("connectDomains", res.CSP.ConnectDomains)
		addDomains("resourceDomains", res.CSP.ResourceDomains)
		addDomains("frameDomains", res.CSP.FrameDomains)
		addDomains("baseUriDomains", res.CSP.BaseURIDomains)
		if len(csp) > 0 {
			uiMeta["csp"] = csp
		}
	}
	if res.PrefersBorder != nil {
		uiMeta["prefersBorder"] = *res.PrefersBorder
	}
	if len(uiMeta) > 0 {
		meta["ui"] = uiMeta
	}
	if len(meta) == 0 {
		return nil
	}
	return meta
}

func (mux *MCPServeMux) handleListResources(w http.ResponseWriter, _ context.Context, id interface{}) {
	mux.mu.RLock()
	defer mux.mu.RUnlock()

	uris := make([]string, 0, len(mux.resources))
	for uri := range mux.resources {
		uris = append(uris, uri)
	}
	sort.Strings(uris)

	resources := make([]map[string]any, 0, len(uris))
	for _, uri := range uris {
		res := mux.resources[uri]
		entry := map[string]any{
			"uri":  res.URI,
			"name": res.Name,
		}
		if res.Title != "" {
			entry["title"] = res.Title
		}
		if res.Description != "" {
			entry["description"] = res.Description
		}
		if res.MIMEType != "" {
			entry["mimeType"] = res.MIMEType
		}
		if meta := resourceMeta(res); meta != nil {
			entry["_meta"] = meta
		}
		resources = append(resources, entry)
	}

	sendSuccess(w, id, map[string]any{"resources": resources})
}

func (mux *MCPServeMux) handleReadResource(w http.ResponseWriter, _ context.Context, id interface{}, params map[string]interface{}) {
	uri, _ := params["uri"].(string)
	if uri == "" {
		sendError(w, id, -32602, "Missing resource uri")
		return
	}

	mux.mu.RLock()
	res, ok := mux.resources[uri]
	mux.mu.RUnlock()
	if !ok {
		sendError(w, id, -32002, fmt.Sprintf("Resource not found: %s", uri))
		return
	}

	content := map[string]any{
		"uri":  res.URI,
		"text": res.Text,
	}
	if res.MIMEType != "" {
		content["mimeType"] = res.MIMEType
	}
	if meta := resourceMeta(res); meta != nil {
		content["_meta"] = meta
	}

	sendSuccess(w, id, map[string]any{"contents": []map[string]any{content}})
}
