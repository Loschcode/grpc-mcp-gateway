package greeter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/linkbreakers-com/grpc-mcp-gateway/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

type greeterServer struct {
	UnimplementedGreeterServer
}

func (greeterServer) SayHello(ctx context.Context, req *HelloRequest) (*HelloReply, error) {
	return &HelloReply{Message: fmt.Sprintf("Hello, %s", req.GetName())}, nil
}

func TestGreeterMCPFlow(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start gRPC server on a bufconn listener.
	lis := bufconn.Listen(1024 * 1024)
	grpcServer := grpc.NewServer()
	RegisterGreeterServer(grpcServer, greeterServer{})
	go func() {
		_ = grpcServer.Serve(lis)
	}()
	defer grpcServer.Stop()

	dialer := func(context.Context, string) (net.Conn, error) {
		return lis.Dial()
	}
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(dialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("grpc dial failed: %v", err)
	}
	defer conn.Close()

	grpcClient := NewGreeterClient(conn)

	// Create MCP mux and register handler.
	mcpMux := runtime.NewMCPServeMux(runtime.ServerMetadata{
		Name:    "greeter-mcp",
		Version: "v0.1.0",
	})
	RegisterGreeterMCPHandler(mcpMux, grpcClient)

	// Call tool via HTTP.
	body, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "greeter.say_hello",
			"arguments": map[string]any{"name": "Ada"},
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mcpMux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d, body: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Result struct {
			StructuredContent map[string]any `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	message, _ := resp.Result.StructuredContent["message"].(string)
	if message != "Hello, Ada" {
		t.Fatalf("unexpected message: %q", message)
	}
}
