package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

func main() {
	body, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "echo",
			"arguments": map[string]any{
				"message": "hello from client",
				"count":   2,
			},
		},
	})

	resp, err := http.Post("http://localhost:8090/", "application/json", bytes.NewReader(body))
	if err != nil {
		log.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		log.Fatalf("parse failed: %v", err)
	}

	pretty, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(pretty))
}
