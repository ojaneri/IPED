package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/ojaneri/ipedlite/core-go/internal/caso"
	"github.com/ojaneri/ipedlite/core-go/internal/modulo"
	"github.com/ojaneri/ipedlite/core-go/internal/motor"
	"github.com/ojaneri/ipedlite/core-go/internal/web"
	"github.com/ojaneri/ipedlite/core-go/internal/worker"
)

// JSONRPCRequest estrutura de uma requisição JSON-RPC 2.0.
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSONRPCResponse estrutura de resposta JSON-RPC 2.0.
type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

// RPCError erro JSON-RPC.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ToolInputSchema define o esquema JSON de parâmetros de uma ferramenta.
type ToolInputSchema struct {
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties"`
	Required   []string               `json:"required,omitempty"`
}

// ToolInfo descreve uma ferramenta exposta pelo servidor MCP.
type ToolInfo struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema ToolInputSchema `json:"inputSchema"`
}

// ToolContent conteúdo de resposta retornado para o agente de IA.
type ToolContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// ToolCallResult resultado de tools/call no protocolo MCP.
type ToolCallResult struct {
	Content []ToolContent `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

// Server implementa o servidor MCP para o IPED Lite Hub.
type Server struct {
	casoAtivo      *caso.Caso
	worker         *worker.IPEDWorker
	webServer      *web.Servidor
	webURL         string
	dirModulos     string
	dirFerramentas string
	mu             sync.Mutex
	in             io.Reader
	out            io.Writer
}

// NovoServer instancia um novo servidor MCP.
func NovoServer(in io.Reader, out io.Writer, w *worker.IPEDWorker) *Server {
	if w == nil {
		cfg := worker.ConfigAutodetectar()
		w = worker.NovoWorker(cfg)
	}
	dirModulos := "modulos"
	if _, err := os.Stat(dirModulos); os.IsNotExist(err) {
		if _, err := os.Stat("core-go/modulos"); err == nil {
			dirModulos = "core-go/modulos"
		}
	}
	dirFerramentas := "ferramentas"
	if _, err := os.Stat(dirFerramentas); os.IsNotExist(err) {
		if _, err := os.Stat("core-go/ferramentas"); err == nil {
			dirFerramentas = "core-go/ferramentas"
		}
	}
	return &Server{
		in:             in,
		out:            out,
		worker:         w,
		dirModulos:     dirModulos,
		dirFerramentas: dirFerramentas,
	}
}

// Executar roda o loop principal do servidor MCP sobre os streams configurados.
func (s *Server) Executar() error {
	scanner := bufio.NewScanner(s.in)
	// Buffer de até 10 MB para requisições com payloads grandes
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, 10*1024*1024)

	for scanner.Scan() {
		linha := scanner.Bytes()
		if len(linha) == 0 {
			continue
		}

		var req JSONRPCRequest
		if err := json.Unmarshal(linha, &req); err != nil {
			s.enviarErro(nil, -32700, "Parse error: "+err.Error())
			continue
		}

		s.processarRequisicao(&req)
	}

	return scanner.Err()
}

func (s *Server) enviarResposta(id interface{}, resultado interface{}) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  resultado,
	}
	dados, _ := json.Marshal(resp)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.out.Write(append(dados, '\n'))
}

func (s *Server) enviarErro(id interface{}, codigo int, msg string) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &RPCError{
			Code:    codigo,
			Message: msg,
		},
	}
	dados, _ := json.Marshal(resp)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.out.Write(append(dados, '\n'))
}

func (s *Server) processarRequisicao(req *JSONRPCRequest) {
	switch req.Method {
	case "initialize":
		res := map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{},
			},
			"serverInfo": map[string]interface{}{
				"name":    "iped-hub",
				"version": "1.0.0",
			},
		}
		s.enviarResposta(req.ID, res)

	case "notifications/initialized":
		// Handshake concluído

	case "tools/list":
		s.enviarResposta(req.ID, map[string]interface{}{
			"tools": s.definicaoFerramentas(),
		})

	case "tools/call":
		var params struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			s.enviarErro(req.ID, -32602, "Invalid params: "+err.Error())
			return
		}

		res := s.executarFerramenta(params.Name, params.Arguments)
		s.enviarResposta(req.ID, res)

	default:
		s.enviarErro(req.ID, -32601, fmt.Sprintf("Método desconhecido: %s", req.Method))
	}
}

func (s *Server) definicaoFerramentas() []ToolInfo {
	return []ToolInfo{
		{
			Name:        "iped_create_case",
			Description: "Cria um novo caso pericial com custódia encadeada e banco SQLite",
			InputSchema: ToolInputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"path":     map[string]string{"type": "string", "description": "Caminho da pasta do novo caso"},
					"name":     map[string]string{"type": "string", "description": "Nome da operação ou caso"},
					"examiner": map[string]string{"type": "string", "description": "Nome do perito responsável"},
				},
				Required: []string{"path", "name"},
			},
		},
		{
			Name:        "iped_open_case",
			Description: "Abre um caso pericial existente no diretório indicado",
			InputSchema: ToolInputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"path": map[string]string{"type": "string", "description": "Caminho da pasta do caso"},
				},
				Required: []string{"path"},
			},
		},
		{
			Name:        "iped_case_status",
			Description: "Retorna o status geral do caso ativo, contagem de evidências, eventos e custódia",
			InputSchema: ToolInputSchema{
				Type:       "object",
				Properties: map[string]interface{}{},
			},
		},
		{
			Name:        "iped_custody_verify",
			Description: "Recalcula e valida todos os elos criptográficos (SHA-256) da cadeia de custódia do caso",
			InputSchema: ToolInputSchema{
				Type:       "object",
				Properties: map[string]interface{}{},
			},
		},
		{
			Name:        "iped_evidence_add",
			Description: "Registra uma nova evidência calculando hashes SHA256, SHA1 e MD5 em streaming. Suporta in-situ (sem cópia)",
			InputSchema: ToolInputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"path":    map[string]string{"type": "string", "description": "Caminho do arquivo ou imagem da evidência"},
					"in_situ": map[string]string{"type": "boolean", "description": "Se true, lê na origem sem duplicar em disco (padrão true)"},
				},
				Required: []string{"path"},
			},
		},
		{
			Name:        "iped_evidence_list",
			Description: "Lista todas as evidências registradas no caso com seus respectivos hashes",
			InputSchema: ToolInputSchema{
				Type:       "object",
				Properties: map[string]interface{}{},
			},
		},
		{
			Name:        "iped_timeline_search",
			Description: "Pesquisa na linha do tempo consolidada de eventos ($MFT, logs EVTX, web, etc.)",
			InputSchema: ToolInputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"query":  map[string]string{"type": "string", "description": "Termo de busca (* para todos)"},
					"limit":  map[string]string{"type": "integer", "description": "Quantidade máxima de registros (padrão 50)"},
					"offset": map[string]string{"type": "integer", "description": "Deslocamento de paginação"},
				},
			},
		},
		{
			Name:        "iped_bookmarks_add",
			Description: "Cria e adiciona uma marcação pericial (bookmark) em uma evidência ou item específico",
			InputSchema: ToolInputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"evidence_id": map[string]string{"type": "integer", "description": "ID da evidência no caso"},
					"item_id":     map[string]string{"type": "integer", "description": "ID do item específico (opcional)"},
					"bookmark":    map[string]string{"type": "string", "description": "Nome da categoria ou etiqueta"},
					"note":        map[string]string{"type": "string", "description": "Observações periciais"},
				},
				Required: []string{"evidence_id", "bookmark"},
			},
		},
		{
			Name:        "iped_bookmarks_list",
			Description: "Lista todas as marcações periciais criadas no caso ativo",
			InputSchema: ToolInputSchema{
				Type:       "object",
				Properties: map[string]interface{}{},
			},
		},
		{
			Name:        "iped_deep_process",
			Description: "Inicia o exame profundo de conteúdo em background via worker Java (IPED Lite)",
			InputSchema: ToolInputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"evidence": map[string]string{"type": "string", "description": "Caminho do arquivo ou imagem de evidência"},
					"output":   map[string]string{"type": "string", "description": "Pasta de saída dos resultados do IPED"},
					"profile":  map[string]string{"type": "string", "description": "Perfil de processamento (padrão 'lite')"},
				},
				Required: []string{"evidence", "output"},
			},
		},
		{
			Name:        "iped_deep_status",
			Description: "Verifica o status e duração do processamento profundo Java em background",
			InputSchema: ToolInputSchema{
				Type:       "object",
				Properties: map[string]interface{}{},
			},
		},
		{
			Name:        "iped_import_iped_case",
			Description: "Importa os resultados de um caso processado pelo IPED (FileList.csv) para o SQLite e timeline do caso ativo",
			InputSchema: ToolInputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"iped_case_dir": map[string]string{"type": "string", "description": "Caminho da pasta do caso processado pelo IPED"},
					"evidence_id":   map[string]string{"type": "integer", "description": "ID da evidência associada (opcional, padrão 1)"},
				},
				Required: []string{"iped_case_dir"},
			},
		},
		{
			Name:        "iped_modules_list",
			Description: "Lista todos os módulos periciais declarativos (modulo.yaml) disponíveis no sistema",
			InputSchema: ToolInputSchema{
				Type:       "object",
				Properties: map[string]interface{}{},
			},
		},
		{
			Name:        "iped_identify_evidence",
			Description: "Identifica o tipo pericial de uma evidência analisando assinatura mágica e extensão",
			InputSchema: ToolInputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"path": map[string]string{"type": "string", "description": "Caminho do arquivo ou imagem da evidência"},
				},
				Required: []string{"path"},
			},
		},
		{
			Name:        "iped_triage_plan",
			Description: "Monta o plano de execução pericial ordenado (triagem rápida primeiro, exames profundos opcionais)",
			InputSchema: ToolInputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"path":         map[string]string{"type": "string", "description": "Caminho do arquivo ou imagem da evidência"},
					"evidence_id":  map[string]string{"type": "integer", "description": "ID da evidência já cadastrada no caso (alternativa ao path)"},
					"include_deep": map[string]string{"type": "boolean", "description": "Se true, inclui módulos profundos e mais lentos (ex: IPED Lite Java). Padrão false"},
				},
			},
		},
		{
			Name:        "iped_triage_run",
			Description: "Executa o plano de triagem pericial para uma evidência e consolida eventos e entidades no caso",
			InputSchema: ToolInputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"evidence_id":  map[string]string{"type": "integer", "description": "ID da evidência registrada no caso a ser processada"},
					"include_deep": map[string]string{"type": "boolean", "description": "Se true, executa também os exames profundos (ex: IPED Lite). Padrão false"},
				},
				Required: []string{"evidence_id"},
			},
		},
		{
			Name:        "iped_alerts_list",
			Description: "Consulta alertas de segurança e achados críticos/altos detectados na linha do tempo",
			InputSchema: ToolInputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"severity": map[string]string{"type": "string", "description": "Filtro de severidade ('critica', 'alta', 'media', 'baixa' ou vazio para críticas e altas)"},
					"limit":    map[string]string{"type": "integer", "description": "Limite máximo de alertas retornados (padrão 50)"},
				},
			},
		},
		{
			Name:        "iped_entities_query",
			Description: "Pesquisa entidades forenses extraídas (IPs, e-mails, domínios, arquivos, usuários) correlacionadas com seus eventos",
			InputSchema: ToolInputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"type":  map[string]string{"type": "string", "description": "Tipo de entidade (ex: 'ip', 'email', 'usuario', 'arquivo' ou '*' para todos)"},
					"query": map[string]string{"type": "string", "description": "Termo ou padrão do valor da entidade para busca"},
					"limit": map[string]string{"type": "integer", "description": "Limite máximo de entidades retornadas (padrão 50)"},
				},
			},
		},
		{
			Name:        "iped_web_serve",
			Description: "Inicia ou retorna a URL do servidor web local para inspeção visual do caso pelo perito",
			InputSchema: ToolInputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"port": map[string]string{"type": "integer", "description": "Porta TCP local para o servidor (padrão 8080)"},
				},
			},
		},
	}
}

func (s *Server) executarFerramenta(nome string, args map[string]interface{}) ToolCallResult {
	switch nome {
	case "iped_create_case":
		caminho, _ := args["path"].(string)
		nomeCaso, _ := args["name"].(string)
		perito, _ := args["examiner"].(string)
		if perito == "" {
			perito = "Perito Forense"
		}
		c, err := caso.Novo(caminho, nomeCaso, perito)
		if err != nil {
			return respostaErro("Falha ao criar caso: " + err.Error())
		}
		s.casoAtivo = c
		return respostaSucesso(map[string]interface{}{
			"status":    "ok",
			"case_id":   c.Info.ID,
			"case_name": c.Info.Nome,
			"path":      c.Dir,
			"examiner":  c.Info.Perito,
		})

	case "iped_open_case":
		caminho, _ := args["path"].(string)
		c, err := caso.Abrir(caminho)
		if err != nil {
			return respostaErro("Falha ao abrir caso: " + err.Error())
		}
		s.casoAtivo = c
		return respostaSucesso(map[string]interface{}{
			"status":    "ok",
			"case_id":   c.Info.ID,
			"case_name": c.Info.Nome,
			"path":      c.Dir,
			"examiner":  c.Info.Perito,
		})

	case "iped_case_status":
		if s.casoAtivo == nil {
			return respostaSucesso(map[string]interface{}{
				"opened":  false,
				"message": "Nenhum caso aberto. Utilize iped_open_case ou iped_create_case.",
			})
		}
		evs, _ := s.casoAtivo.ListarEvidencias()
		_, totalEvt, _ := s.casoAtivo.BuscarEventos("*", 1, 0)
		bks, _ := s.casoAtivo.ListarMarcacoes()
		totalCustodia, okCustodia, _ := s.casoAtivo.Custodia.Verificar()

		return respostaSucesso(map[string]interface{}{
			"opened":          true,
			"case_id":         s.casoAtivo.Info.ID,
			"case_name":       s.casoAtivo.Info.Nome,
			"path":            s.casoAtivo.Dir,
			"examiner":        s.casoAtivo.Info.Perito,
			"total_evidences": len(evs),
			"total_events":    totalEvt,
			"total_bookmarks": len(bks),
			"custody_valid":   okCustodia,
			"custody_records": totalCustodia,
			"custody_head":    s.casoAtivo.Custodia.UltimoHash(),
		})

	case "iped_custody_verify":
		if s.casoAtivo == nil {
			return respostaErro("Nenhum caso aberto.")
		}
		total, ok, err := s.casoAtivo.Custodia.Verificar()
		if err != nil {
			return respostaErro("Erro na verificação da custódia: " + err.Error())
		}
		return respostaSucesso(map[string]interface{}{
			"valid":        ok,
			"total_steps":  total,
			"head_hash":    s.casoAtivo.Custodia.UltimoHash(),
			"verification": "Todos os registros criptográficos conferem rigorosamente com os hashes calculados.",
		})

	case "iped_evidence_add":
		if s.casoAtivo == nil {
			return respostaErro("Nenhum caso aberto. Abra um caso antes de registrar evidências.")
		}
		caminho, _ := args["path"].(string)
		inSitu := true
		if val, exists := args["in_situ"]; exists {
			if b, ok := val.(bool); ok {
				inSitu = b
			}
		}
		ev, err := s.casoAtivo.AdicionarEvidencia(caminho, inSitu)
		if err != nil {
			return respostaErro("Erro adicionando evidência: " + err.Error())
		}
		return respostaSucesso(ev)

	case "iped_evidence_list":
		if s.casoAtivo == nil {
			return respostaErro("Nenhum caso aberto.")
		}
		evs, err := s.casoAtivo.ListarEvidencias()
		if err != nil {
			return respostaErro("Erro listando evidências: " + err.Error())
		}
		return respostaSucesso(map[string]interface{}{
			"total":      len(evs),
			"evidences": evs,
		})

	case "iped_timeline_search":
		if s.casoAtivo == nil {
			return respostaErro("Nenhum caso aberto.")
		}
		query, _ := args["query"].(string)
		limite := 50
		offset := 0
		if l, ok := args["limit"].(float64); ok && l > 0 {
			limite = int(l)
		}
		if o, ok := args["offset"].(float64); ok && o >= 0 {
			offset = int(o)
		}
		evts, total, err := s.casoAtivo.BuscarEventos(query, limite, offset)
		if err != nil {
			return respostaErro("Erro na busca de eventos: " + err.Error())
		}
		return respostaSucesso(map[string]interface{}{
			"query":      query,
			"total_hits": total,
			"returned":   len(evts),
			"offset":     offset,
			"events":     evts,
		})

	case "iped_bookmarks_add":
		if s.casoAtivo == nil {
			return respostaErro("Nenhum caso aberto.")
		}
		evIDFloat, _ := args["evidence_id"].(float64)
		evID := int64(evIDFloat)
		itemIDFloat, _ := args["item_id"].(float64)
		itemID := int64(itemIDFloat)
		nome, _ := args["bookmark"].(string)
		nota, _ := args["note"].(string)

		m, err := s.casoAtivo.AdicionarMarcacao(evID, itemID, nome, nota)
		if err != nil {
			return respostaErro("Erro criando marcação: " + err.Error())
		}
		return respostaSucesso(m)

	case "iped_bookmarks_list":
		if s.casoAtivo == nil {
			return respostaErro("Nenhum caso aberto.")
		}
		bks, err := s.casoAtivo.ListarMarcacoes()
		if err != nil {
			return respostaErro("Erro listando marcações: " + err.Error())
		}
		return respostaSucesso(map[string]interface{}{
			"total":     len(bks),
			"bookmarks": bks,
		})

	case "iped_deep_process":
		evidencia, _ := args["evidence"].(string)
		saida, _ := args["output"].(string)
		perfil, _ := args["profile"].(string)
		if perfil == "" {
			perfil = "lite"
		}
		if err := s.worker.IniciarProcessamento(evidencia, saida, perfil); err != nil {
			return respostaErro("Erro iniciando IPED Lite: " + err.Error())
		}
		return respostaSucesso(map[string]interface{}{
			"status":   "launched",
			"evidence": evidencia,
			"output":   saida,
			"profile":  perfil,
			"message":  "Exame profundo do IPED Lite iniciado em background.",
		})

	case "iped_deep_status":
		rodando, duracao, err := s.worker.Status()
		statusStr := "idle"
		if rodando {
			statusStr = "running"
		} else if err != "" {
			statusStr = "failed"
		} else if duracao > 0 {
			statusStr = "completed"
		}
		return respostaSucesso(map[string]interface{}{
			"status":            statusStr,
			"elapsed_seconds":   duracao.Seconds(),
			"elapsed_formatted": duracao.String(),
			"error":             err,
		})

	case "iped_import_iped_case":
		if s.casoAtivo == nil {
			return respostaErro("Nenhum caso aberto. Abra um caso antes de importar dados do IPED.")
		}
		ipedDir, _ := args["iped_case_dir"].(string)
		var evID int64 = 1
		if evIDFloat, ok := args["evidence_id"].(float64); ok && evIDFloat > 0 {
			evID = int64(evIDFloat)
		}
		res, err := s.casoAtivo.ImportarCasoIPED(ipedDir, evID)
		if err != nil {
			return respostaErro("Erro importando caso IPED: " + err.Error())
		}
		return respostaSucesso(res)

	case "iped_modules_list":
		modulos, err := modulo.CarregarModulos(s.dirModulos)
		if err != nil {
			return respostaErro("Erro carregando módulos: " + err.Error())
		}
		var lista []map[string]interface{}
		for _, m := range modulos {
			lista = append(lista, map[string]interface{}{
				"nome":     m.Nome,
				"titulo":   m.Titulo,
				"versao":   m.Versao,
				"opcional": m.Opcional,
				"consome":  m.Consome,
				"modo":     m.Modo,
			})
		}
		return respostaSucesso(map[string]interface{}{
			"total":   len(lista),
			"modules": lista,
		})

	case "iped_identify_evidence":
		caminho, _ := args["path"].(string)
		modulos, _ := modulo.CarregarModulos(s.dirModulos)
		tipo, err := modulo.IdentificarTipo(caminho, modulos)
		if err != nil {
			return respostaErro("Erro identificando evidência: " + err.Error())
		}
		return respostaSucesso(map[string]interface{}{
			"path": caminho,
			"type": tipo,
		})

	case "iped_triage_plan":
		caminho, _ := args["path"].(string)
		incluirProfundo, _ := args["include_deep"].(bool)
		evIDFloat, _ := args["evidence_id"].(float64)

		if evIDFloat > 0 && s.casoAtivo != nil {
			ev, err := s.casoAtivo.ObterEvidencia(int64(evIDFloat))
			if err == nil && caminho == "" {
				caminho = ev.Origem
			}
		}

		if caminho == "" {
			return respostaErro("Parâmetro 'path' ou 'evidence_id' válido é obrigatório.")
		}

		modulos, err := modulo.CarregarModulos(s.dirModulos)
		if err != nil {
			return respostaErro("Erro carregando módulos: " + err.Error())
		}

		tipo, err := modulo.IdentificarTipo(caminho, modulos)
		if err != nil {
			return respostaErro("Erro identificando tipo da evidência: " + err.Error())
		}

		plano := motor.MontarPlano(tipo, modulos, incluirProfundo)
		var lista []map[string]interface{}
		for _, m := range plano {
			lista = append(lista, map[string]interface{}{
				"modulo":   m.Nome,
				"titulo":   m.Titulo,
				"versao":   m.Versao,
				"modo":     m.Modo,
				"opcional": m.Opcional,
				"consome":  m.Consome,
			})
		}

		return respostaSucesso(map[string]interface{}{
			"path":          caminho,
			"type":          tipo,
			"include_deep":  incluirProfundo,
			"total_modules": len(lista),
			"modules":       lista,
		})

	case "iped_triage_run":
		if s.casoAtivo == nil {
			return respostaErro("Nenhum caso aberto. Abra um caso antes de executar a triagem.")
		}
		evIDFloat, _ := args["evidence_id"].(float64)
		if evIDFloat <= 0 {
			return respostaErro("Parâmetro 'evidence_id' obrigatório.")
		}
		evID := int64(evIDFloat)
		ev, err := s.casoAtivo.ObterEvidencia(evID)
		if err != nil {
			return respostaErro("Evidência não encontrada: " + err.Error())
		}

		incluirProfundo, _ := args["include_deep"].(bool)
		modulos, err := modulo.CarregarModulos(s.dirModulos)
		if err != nil {
			return respostaErro("Erro carregando módulos: " + err.Error())
		}

		if ev.Tipo == "" || ev.Tipo == "arquivo" {
			detected, errId := modulo.IdentificarTipo(ev.Origem, modulos)
			if errId == nil && detected != "" {
				ev.Tipo = detected
			}
		}

		plano := motor.MontarPlano(ev.Tipo, modulos, incluirProfundo)
		if len(plano) == 0 {
			return respostaSucesso(map[string]interface{}{
				"evidencia_id":   ev.ID,
				"tipo_evidencia": ev.Tipo,
				"total_modulos":  0,
				"message":        "Nenhum módulo aplicável para o tipo de evidência: " + ev.Tipo,
			})
		}

		resultado, err := motor.ExecutarPlano(context.Background(), s.casoAtivo, ev, plano, s.dirFerramentas)
		if err != nil {
			return respostaErro("Erro executando plano de triagem: " + err.Error())
		}

		return respostaSucesso(resultado)

	case "iped_alerts_list":
		if s.casoAtivo == nil {
			return respostaErro("Nenhum caso aberto.")
		}
		severidade, _ := args["severity"].(string)
		limite := 50
		if l, ok := args["limit"].(float64); ok && l > 0 {
			limite = int(l)
		}
		alertas, err := s.casoAtivo.BuscarAlertas(severidade, limite)
		if err != nil {
			return respostaErro("Erro buscando alertas: " + err.Error())
		}
		return respostaSucesso(map[string]interface{}{
			"total":  len(alertas),
			"alerts": alertas,
		})

	case "iped_entities_query":
		if s.casoAtivo == nil {
			return respostaErro("Nenhum caso aberto.")
		}
		tipo, _ := args["type"].(string)
		termo, _ := args["query"].(string)
		limite := 50
		if l, ok := args["limit"].(float64); ok && l > 0 {
			limite = int(l)
		}
		entidades, err := s.casoAtivo.BuscarEntidades(tipo, termo, limite)
		if err != nil {
			return respostaErro("Erro buscando entidades: " + err.Error())
		}
		return respostaSucesso(map[string]interface{}{
			"total":    len(entidades),
			"entities": entidades,
		})

	case "iped_web_serve":
		if s.casoAtivo == nil {
			return respostaErro("Nenhum caso aberto. Abra um caso antes de iniciar o servidor web.")
		}
		porta := 8080
		if p, ok := args["port"].(float64); ok && p > 0 {
			porta = int(p)
		}
		if s.webServer != nil {
			return respostaSucesso(map[string]interface{}{
				"status":  "running",
				"url":     s.webURL,
				"message": "Servidor web já está ativo para este caso.",
			})
		}
		srv := web.NovoServidor(s.casoAtivo, porta)
		urlBase, err := srv.Iniciar()
		if err != nil {
			return respostaErro("Erro iniciando servidor web: " + err.Error())
		}
		s.webServer = srv
		s.webURL = urlBase
		return respostaSucesso(map[string]interface{}{
			"status":  "started",
			"url":     urlBase,
			"message": "Painel web forense iniciado com sucesso.",
		})

	default:
		return respostaErro(fmt.Sprintf("Ferramenta não encontrada: %s", nome))
	}
}

func respostaSucesso(obj interface{}) ToolCallResult {
	dados, _ := json.Marshal(obj)
	return ToolCallResult{
		Content: []ToolContent{
			{
				Type: "text",
				Text: string(dados),
			},
		},
	}
}

func respostaErro(msg string) ToolCallResult {
	return ToolCallResult{
		IsError: true,
		Content: []ToolContent{
			{
				Type: "text",
				Text: msg,
			},
		},
	}
}

// Fechar encerra o servidor web e o caso ativo se houver.
func (s *Server) Fechar() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.webServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = s.webServer.Parar(ctx)
		s.webServer = nil
	}
	if s.casoAtivo != nil {
		return s.casoAtivo.Fechar()
	}
	return nil
}

// RedirecionarLogsStdErr redireciona a saída padrão de logs para stderr garantindo isolamento de stdio.
func RedirecionarLogsStdErr() {
	// Garante que logs do processo não poluam stdout
	os.Stdout.Sync()
}
