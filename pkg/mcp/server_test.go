package mcp_test

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/CFMVCarlos/mhn-database/pkg/database"
	"github.com/CFMVCarlos/mhn-database/pkg/mcp"
)

func TestMCPServer(t *testing.T) {
	dbPath := filepath.Join("..", "..", "data", "mhn.db")
	db, err := database.OpenDB(dbPath)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	// Prepare mock stdin
	requests := []mcp.JSONRPCRequest{
		{
			JSONRPC: "2.0",
			ID:      1,
			Method:  "initialize",
		},
		{
			JSONRPC: "2.0",
			ID:      2,
			Method:  "tools/list",
		},
		{
			JSONRPC: "2.0",
			ID:      3,
			Method:  "tools/call",
			Params: json.RawMessage(`{
				"name": "get_monster",
				"arguments": {"name_or_id": "Rathalos"}
			}`),
		},
	}

	var inBuf bytes.Buffer
	for _, req := range requests {
		b, _ := json.Marshal(req)
		inBuf.Write(b)
		inBuf.WriteByte('\n')
	}

	var outBuf bytes.Buffer
	err = mcp.ServeMCP(db, &inBuf, &outBuf)
	if err != nil {
		t.Fatalf("ServeMCP error: %v", err)
	}

	lines := bytes.Split(bytes.TrimSpace(outBuf.Bytes()), []byte("\n"))
	if len(lines) != 3 {
		t.Fatalf("expected 3 responses, got %d", len(lines))
	}

	// Verify initialize response
	var initResp mcp.JSONRPCResponse
	if err := json.Unmarshal(lines[0], &initResp); err != nil {
		t.Fatalf("failed to unmarshal init response: %v", err)
	}
	if initResp.Error != nil {
		t.Fatalf("init error: %v", initResp.Error)
	}

	// Verify tools/call response
	var callResp mcp.JSONRPCResponse
	if err := json.Unmarshal(lines[2], &callResp); err != nil {
		t.Fatalf("failed to unmarshal call response: %v", err)
	}
	resMap, ok := callResp.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected call result shape: %T", callResp.Result)
	}
	if resMap["isError"] == true {
		t.Fatalf("tool call returned isError=true")
	}
}
