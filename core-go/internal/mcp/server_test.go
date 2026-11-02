package mcp

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMCPServerEmMemoria(t *testing.T) {
	tmpDir := t.TempDir()
	pastaCaso := filepath.Join(tmpDir, "caso-mcp")

	// Prepara requisições sequenciais de teste JSON-RPC
	requisicoes := []string{
		// 1. initialize
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		// 2. tools/list
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`,
		// 3. iped_create_case
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"iped_create_case","arguments":{"path":"` + strings.ReplaceAll(pastaCaso, "\\", "\\\\") + `","name":"Caso MCP Teste","examiner":"Perito IA"}}}`,
		// 4. iped_case_status
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"iped_case_status","arguments":{}}}`,
		// 5. iped_custody_verify
		`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"iped_custody_verify","arguments":{}}}`,
		// 6. iped_triage_plan
		`{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"iped_triage_plan","arguments":{"path":"C:\\\\Windows\\\\System32\\\\winevt\\\\Logs\\\\Security.evtx","include_deep":false}}}`,
		// 7. iped_alerts_list
		`{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"iped_alerts_list","arguments":{}}}`,
		// 8. iped_entities_query
		`{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"iped_entities_query","arguments":{"type":"*","query":"*"}}}`,
	}

	payloadEntrada := strings.Join(requisicoes, "\n") + "\n"
	in := bytes.NewBufferString(payloadEntrada)
	out := &bytes.Buffer{}

	server := NovoServer(in, out, nil)
	defer server.Fechar()

	if err := server.Executar(); err != nil {
		t.Fatalf("falha executando loop do servidor MCP: %v", err)
	}

	linhasSaida := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(linhasSaida) != len(requisicoes) {
		t.Fatalf("esperava %d respostas, obteve %d", len(requisicoes), len(linhasSaida))
	}

	// Valida resposta 1 (initialize)
	var resp1 JSONRPCResponse
	json.Unmarshal([]byte(linhasSaida[0]), &resp1)
	if resp1.Error != nil {
		t.Fatalf("erro em initialize: %v", resp1.Error)
	}

	// Valida resposta 2 (tools/list)
	var resp2 JSONRPCResponse
	json.Unmarshal([]byte(linhasSaida[1]), &resp2)
	resTools := resp2.Result.(map[string]interface{})
	toolsList := resTools["tools"].([]interface{})
	if len(toolsList) < 18 {
		t.Fatalf("esperava pelo menos 18 ferramentas no MCP, obteve %d", len(toolsList))
	}

	// Valida resposta 3 (iped_create_case)
	var resp3 JSONRPCResponse
	json.Unmarshal([]byte(linhasSaida[2]), &resp3)
	if resp3.Error != nil {
		t.Fatalf("erro criando caso: %v", resp3.Error)
	}

	// Valida resposta 5 (iped_custody_verify)
	var resp5 JSONRPCResponse
	json.Unmarshal([]byte(linhasSaida[4]), &resp5)
	if resp5.Error != nil {
		t.Fatalf("erro verificando custódia: %v", resp5.Error)
	}

	// Confere se o arquivo do banco caso.db e custodia.jsonl foram criados
	if _, err := os.Stat(filepath.Join(pastaCaso, "caso.db")); err != nil {
		t.Fatalf("arquivo caso.db não foi criado no disco")
	}
	if _, err := os.Stat(filepath.Join(pastaCaso, "custodia.jsonl")); err != nil {
		t.Fatalf("arquivo custodia.jsonl não foi criado no disco")
	}
}
