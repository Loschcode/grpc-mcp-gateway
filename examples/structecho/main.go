package main

import (
	"context"
	"log"
	"net/http"

	"github.com/linkbreakers-com/grpc-mcp-gateway/runtime"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/structpb"
)

type echoClient struct{}

func (e *echoClient) Echo(ctx context.Context, in *structpb.Struct, _ ...grpc.CallOption) (*structpb.Struct, error) {
	return in, nil
}

func main() {
	mcpMux := runtime.NewMCPServeMux(runtime.ServerMetadata{
		Name:    "structecho",
		Version: "v0.1.0",
	})
	RegisterEchoServiceMCPHandler(mcpMux, &echoClient{})

	log.Println("structecho MCP server listening on :8090")
	if err := http.ListenAndServe(":8090", mcpMux); err != nil {
		log.Fatal(err)
	}
}
