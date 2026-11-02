package web

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/ojaneri/ipedlite/core-go/internal/caso"
)

//go:embed static/*
var arquivosEstaticos embed.FS

// Servidor encapsula o servidor HTTP local do IPED Lite Hub.
type Servidor struct {
	caso     *caso.Caso
	porta    int
	srv      *http.Server
	listener net.Listener
	mu       sync.Mutex
}

// NovoServidor instancia o servidor web apontando para o caso ativo.
func NovoServidor(c *caso.Caso, porta int) *Servidor {
	if porta <= 0 {
		porta = 8080
	}
	return &Servidor{
		caso:  c,
		porta: porta,
	}
}

// Handler monta e retorna o roteador HTTP com todas as rotas de API e estáticos.
func (s *Servidor) Handler() http.Handler {
	mux := http.NewServeMux()

	// 1. Arquivos Estáticos (Single Page App)
	subFS, err := fs.Sub(arquivosEstaticos, "static")
	if err == nil {
		mux.Handle("/", http.FileServer(http.FS(subFS)))
	}

	// 2. Endpoints da API REST
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/evidencias", s.handleEvidencias)
	mux.HandleFunc("/api/eventos", s.handleEventos)
	mux.HandleFunc("/api/alertas", s.handleAlertas)
	mux.HandleFunc("/api/entidades", s.handleEntidades)
	mux.HandleFunc("/api/custodia", s.handleCustodia)
	mux.HandleFunc("/api/marcacoes", s.handleMarcacoes)

	return mux
}

// Iniciar abre o listener TCP e inicializa o servidor em background. Retorna a URL base.
func (s *Servidor) Iniciar() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	endereco := fmt.Sprintf("127.0.0.1:%d", s.porta)
	l, err := net.Listen("tcp", endereco)
	if err != nil {
		// Tenta porta aleatória/dinâmica caso a padrão esteja ocupada
		l, err = net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return "", fmt.Errorf("falha ao iniciar listener web: %w", err)
		}
	}
	s.listener = l

	s.srv = &http.Server{
		Handler:      s.Handler(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
	}

	go func() {
		_ = s.srv.Serve(s.listener)
	}()

	urlBase := fmt.Sprintf("http://%s", s.listener.Addr().String())
	return urlBase, nil
}

// Parar encerra graciosamente o servidor HTTP.
func (s *Servidor) Parar(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.srv != nil {
		return s.srv.Shutdown(ctx)
	}
	return nil
}

// --- HANDLERS DA API ---

func (s *Servidor) handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.caso == nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"opened": false})
		return
	}

	evs, _ := s.caso.ListarEvidencias()
	_, totalEventos, _ := s.caso.BuscarEventos("*", 1, 0)
	bks, _ := s.caso.ListarMarcacoes()
	totalCustodia, okCustodia, _ := s.caso.Custodia.Verificar()

	json.NewEncoder(w).Encode(map[string]interface{}{
		"opened":          true,
		"case_id":         s.caso.Info.ID,
		"case_name":       s.caso.Info.Nome,
		"examiner":        s.caso.Info.Perito,
		"created_at":      s.caso.Info.CriadoEm,
		"path":            s.caso.Dir,
		"total_evidences": len(evs),
		"total_events":    totalEventos,
		"total_bookmarks": len(bks),
		"custody_valid":   okCustodia,
		"custody_records": totalCustodia,
		"custody_head":    s.caso.Custodia.UltimoHash(),
	})
}

func (s *Servidor) handleEvidencias(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.caso == nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "caso não aberto"})
		return
	}

	evs, err := s.caso.ListarEvidencias()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"total":      len(evs),
		"evidences": evs,
	})
}

func (s *Servidor) handleEventos(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.caso == nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "caso não aberto"})
		return
	}

	query := r.URL.Query().Get("query")
	severidade := r.URL.Query().Get("severidade")
	limite := 50
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if val, err := strconv.Atoi(l); err == nil && val > 0 {
			limite = val
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if val, err := strconv.Atoi(o); err == nil && val >= 0 {
			offset = val
		}
	}

	// Se filtro de severidade foi especificado, aplica na consulta
	if severidade != "" && severidade != "*" {
		alertas, err := s.caso.BuscarAlertas(severidade, limite)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"query":      query,
			"severidade": severidade,
			"total_hits": len(alertas),
			"events":     alertas,
		})
		return
	}

	eventos, total, err := s.caso.BuscarEventos(query, limite, offset)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"query":      query,
		"total_hits": total,
		"limit":      limite,
		"offset":     offset,
		"events":     eventos,
	})
}

func (s *Servidor) handleAlertas(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.caso == nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "caso não aberto"})
		return
	}

	severidade := r.URL.Query().Get("severidade")
	limite := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if val, err := strconv.Atoi(l); err == nil && val > 0 {
			limite = val
		}
	}

	alertas, err := s.caso.BuscarAlertas(severidade, limite)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"total":  len(alertas),
		"alerts": alertas,
	})
}

func (s *Servidor) handleEntidades(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.caso == nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "caso não aberto"})
		return
	}

	tipo := r.URL.Query().Get("type")
	termo := r.URL.Query().Get("query")
	limite := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if val, err := strconv.Atoi(l); err == nil && val > 0 {
			limite = val
		}
	}

	entidades, err := s.caso.BuscarEntidades(tipo, termo, limite)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"total":    len(entidades),
		"entities": entidades,
	})
}

func (s *Servidor) handleCustodia(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.caso == nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "caso não aberto"})
		return
	}

	total, ok, err := s.caso.Custodia.Verificar()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	entradas, _ := s.caso.Custodia.ListarEntradas()

	json.NewEncoder(w).Encode(map[string]interface{}{
		"valid":       ok,
		"total_steps": total,
		"head_hash":   s.caso.Custodia.UltimoHash(),
		"records":     entradas,
	})
}

func (s *Servidor) handleMarcacoes(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.caso == nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "caso não aberto"})
		return
	}

	if r.Method == http.MethodGet {
		bks, err := s.caso.ListarMarcacoes()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"total":     len(bks),
			"bookmarks": bks,
		})
		return
	}

	if r.Method == http.MethodPost {
		var req struct {
			EvidenceID int64  `json:"evidence_id"`
			ItemID     int64  `json:"item_id"`
			Bookmark   string `json:"bookmark"`
			Note       string `json:"note"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "payload inválido"})
			return
		}

		m, err := s.caso.AdicionarMarcacao(req.EvidenceID, req.ItemID, req.Bookmark, req.Note)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		json.NewEncoder(w).Encode(m)
		return
	}

	w.WriteHeader(http.StatusMethodNotAllowed)
}
