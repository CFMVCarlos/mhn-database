package mcp

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/CFMVCarlos/mhn-database/pkg/query"
)

type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"inputSchema"`
}

var ToolsList = []Tool{
	{
		Name:        "get_monster",
		Description: "Get complete, accurate monster data: weaknesses, breakable parts, habitats, species, tactics, and full material drops list.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name_or_id": map[string]interface{}{
					"type":        "string",
					"description": "Monster name, slug, or ID (e.g. 'Rathalos', 'great-jagras', 'zinogre')",
				},
			},
			"required": []string{"name_or_id"},
		},
	},
	{
		Name:        "get_weapon",
		Description: "Get weapon details, attack, element, unlocked skills, and stat progression across grades 1-10.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name_or_id": map[string]interface{}{
					"type":        "string",
					"description": "Weapon name or ID (e.g. 'Hunter's Knife', 'Buster Sword', 'Wyvern Blade')",
				},
				"grade": map[string]interface{}{
					"type":        "integer",
					"description": "Optional weapon grade (1 to 10)",
				},
				"level": map[string]interface{}{
					"type":        "integer",
					"description": "Optional weapon level (1 to 5)",
				},
			},
			"required": []string{"name_or_id"},
		},
	},
	{
		Name:        "get_armor",
		Description: "Get armor piece details, slot, series, defense, driftsmelt slots, and skills granted.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name_or_id": map[string]interface{}{
					"type":        "string",
					"description": "Armor piece name or ID (e.g. 'Leather Headgear', 'Rathalos Mail', 'Kulu Vambraces')",
				},
				"grade": map[string]interface{}{
					"type":        "integer",
					"description": "Optional armor grade (1 to 10)",
				},
				"level": map[string]interface{}{
					"type":        "integer",
					"description": "Optional armor level (1 to 5)",
				},
			},
			"required": []string{"name_or_id"},
		},
	},
	{
		Name:        "get_skill",
		Description: "Get skill details, level effects (1-5), and all armor and weapons that grant this skill.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name_or_id": map[string]interface{}{
					"type":        "string",
					"description": "Skill name or ID (e.g. 'Attack Boost', 'Critical Eye', 'Water Attack')",
				},
			},
			"required": []string{"name_or_id"},
		},
	},
	{
		Name:        "find_gear_by_skill",
		Description: "Search for all armor pieces and weapons that provide a given skill, with optional minimum skill level filter.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"skill_name": map[string]interface{}{
					"type":        "string",
					"description": "Skill name (e.g. 'Critical Eye', 'Lock On', 'Weakness Exploit')",
				},
				"min_level": map[string]interface{}{
					"type":        "integer",
					"description": "Optional minimum skill level provided by the gear (default: 1)",
				},
			},
			"required": []string{"skill_name"},
		},
	},
	{
		Name:        "get_material_sources",
		Description: "Find which monsters drop a specific material, along with drop rarity and minimum star grade required.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"material_name": map[string]interface{}{
					"type":        "string",
					"description": "Material name or key (e.g. 'Rathalos Scale', 'Wyvern Gem Shard', 'Prime Gem')",
				},
			},
			"required": []string{"material_name"},
		},
	},
	{
		Name:        "run_sql",
		Description: "Execute a read-only SQL SELECT query against the normalized SQLite database to perform custom multi-table aggregations or queries.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "Read-only SQL query (SELECT ...)",
				},
			},
			"required": []string{"query"},
		},
	},
}

// ServeMCP reads JSON-RPC requests from in and writes responses to out.
func ServeMCP(db *sql.DB, in io.Reader, out io.Writer) error {
	scanner := bufio.NewScanner(in)
	buf := make([]byte, 1024*1024) // 1MB line buffer
	scanner.Buffer(buf, 10*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req JSONRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			sendError(out, nil, -32700, "Parse error")
			continue
		}

		handleRequest(db, out, req)
	}

	return scanner.Err()
}

func handleRequest(db *sql.DB, out io.Writer, req JSONRPCRequest) {
	// Notifications don't get responses
	if req.ID == nil {
		return
	}

	switch req.Method {
	case "initialize":
		res := map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{},
			},
			"serverInfo": map[string]interface{}{
				"name":    "mhn-db-server",
				"version": "1.0.0",
			},
		}
		sendResult(out, req.ID, res)

	case "ping":
		sendResult(out, req.ID, map[string]interface{}{})

	case "tools/list":
		sendResult(out, req.ID, map[string]interface{}{
			"tools": ToolsList,
		})

	case "tools/call":
		var callParams struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &callParams); err != nil {
			sendError(out, req.ID, -32602, "Invalid params")
			return
		}

		content, err := dispatchTool(db, callParams.Name, callParams.Arguments)
		if err != nil {
			sendToolResult(out, req.ID, fmt.Sprintf("Error: %v", err), true)
			return
		}
		sendToolResult(out, req.ID, content, false)

	default:
		sendError(out, req.ID, -32601, fmt.Sprintf("Method not found: %s", req.Method))
	}
}

func dispatchTool(db *sql.DB, toolName string, args map[string]interface{}) (string, error) {
	switch toolName {
	case "get_monster":
		name, _ := args["name_or_id"].(string)
		if name == "" {
			return "", fmt.Errorf("name_or_id is required")
		}
		res, err := query.LookupMonster(db, name)
		if err != nil {
			return "", err
		}
		return query.ToJSON(res), nil

	case "get_weapon":
		name, _ := args["name_or_id"].(string)
		if name == "" {
			return "", fmt.Errorf("name_or_id is required")
		}
		grade, _ := getInt(args["grade"])
		level, _ := getInt(args["level"])
		res, err := query.LookupWeapon(db, name, grade, level)
		if err != nil {
			return "", err
		}
		return query.ToJSON(res), nil

	case "get_armor":
		name, _ := args["name_or_id"].(string)
		if name == "" {
			return "", fmt.Errorf("name_or_id is required")
		}
		grade, _ := getInt(args["grade"])
		level, _ := getInt(args["level"])
		res, err := query.LookupArmor(db, name, grade, level)
		if err != nil {
			return "", err
		}
		return query.ToJSON(res), nil

	case "get_skill":
		name, _ := args["name_or_id"].(string)
		if name == "" {
			return "", fmt.Errorf("name_or_id is required")
		}
		res, err := query.LookupSkill(db, name)
		if err != nil {
			return "", err
		}
		return query.ToJSON(res), nil

	case "find_gear_by_skill":
		skillName, _ := args["skill_name"].(string)
		if skillName == "" {
			return "", fmt.Errorf("skill_name is required")
		}
		minLvl, _ := getInt(args["min_level"])
		res, err := query.FindGearBySkill(db, skillName, minLvl)
		if err != nil {
			return "", err
		}
		return query.ToJSON(res), nil

	case "get_material_sources":
		matName, _ := args["material_name"].(string)
		if matName == "" {
			return "", fmt.Errorf("material_name is required")
		}
		res, err := query.LookupMaterial(db, matName)
		if err != nil {
			return "", err
		}
		return query.ToJSON(res), nil

	case "run_sql":
		q, _ := args["query"].(string)
		if q == "" {
			return "", fmt.Errorf("query is required")
		}
		res, err := query.RunSQL(db, q)
		if err != nil {
			return "", err
		}
		return query.ToJSON(res), nil

	default:
		return "", fmt.Errorf("unknown tool: %s", toolName)
	}
}

func getInt(v interface{}) (int, bool) {
	if v == nil {
		return 0, false
	}
	switch val := v.(type) {
	case float64:
		return int(val), true
	case int:
		return val, true
	case string:
		var n int
		if _, err := fmt.Sscanf(val, "%d", &n); err == nil {
			return n, true
		}
	}
	return 0, false
}

func sendResult(out io.Writer, id interface{}, result interface{}) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	b, _ := json.Marshal(resp)
	fmt.Fprintf(out, "%s\n", b)
}

func sendToolResult(out io.Writer, id interface{}, text string, isError bool) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": text,
				},
			},
			"isError": isError,
		},
	}
	b, _ := json.Marshal(resp)
	fmt.Fprintf(out, "%s\n", b)
}

func sendError(out io.Writer, id interface{}, code int, message string) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &RPCError{
			Code:    code,
			Message: message,
		},
	}
	b, _ := json.Marshal(resp)
	fmt.Fprintf(out, "%s\n", b)
}

// StartStdioServer runs the MCP server on os.Stdin and os.Stdout
func StartStdioServer(db *sql.DB) error {
	return ServeMCP(db, os.Stdin, os.Stdout)
}
