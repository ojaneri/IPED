package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/ojaneri/ipedlite/core-go/internal/caso"
)

func TestServidorWebEndpoints(t *testing.T) {
	tmpDir := t.TempDir()
	pastaCaso := filepath.Join(tmpDir, "caso-web-test")

	c, err := caso.Novo(pastaCaso, "Operacao Web Test", "Perito Web")
	if err != nil {
		t.Fatalf("falha ao criar caso: %v", err)
	}
	defer c.Fechar()

	// Adiciona uma evidência
	arqEv := filepath.Join(tmpDir, "evidencia.raw")
	os.WriteFile(arqEv, []byte("RAW_FORENSIC_DATA_SAMPLE"), 0644)
	ev, err := c.AdicionarEvidencia(arqEv, true)
	if err != nil {
		t.Fatalf("falha ao adicionar evidencia: %v", err)
	}

	// Adiciona evento e entidade
	evtID, err := c.InserirEvento(ev.ID, "2026-11-02T12:00:00Z", "hayabusa", "Alerta de Mimikatz em memória", "critica", "EventID:4688")
	if err != nil {
		t.Fatalf("falha ao inserir evento: %v", err)
	}
	if err := c.InserirEntidade(evtID, "ip", "10.0.0.5"); err != nil {
		t.Fatalf("falha ao inserir entidade: %v", err)
	}

	servidor := NovoServidor(c, 0)
	ts := httptest.NewServer(servidor.Handler())
	defer ts.Close()

	// 1. Testa página estática /
	resHome, err := http.Get(ts.URL + "/")
	if err != nil || resHome.StatusCode != http.StatusOK {
		t.Fatalf("falha ao acessar /: err=%v, code=%d", err, resHome.StatusCode)
	}

	// 2. Testa /api/status
	resStatus, err := http.Get(ts.URL + "/api/status")
	if err != nil || resStatus.StatusCode != http.StatusOK {
		t.Fatalf("falha ao acessar /api/status: %v", err)
	}
	var dataStatus map[string]interface{}
	json.NewDecoder(resStatus.Body).Decode(&dataStatus)
	if dataStatus["case_name"] != "Operacao Web Test" || dataStatus["custody_valid"] != true {
		t.Fatalf("status incorreto: %+v", dataStatus)
	}

	// 3. Testa /api/evidencias
	resEv, err := http.Get(ts.URL + "/api/evidencias")
	if err != nil || resEv.StatusCode != http.StatusOK {
		t.Fatalf("falha ao acessar /api/evidencias: %v", err)
	}
	var dataEv map[string]interface{}
	json.NewDecoder(resEv.Body).Decode(&dataEv)
	if int(dataEv["total"].(float64)) != 1 {
		t.Fatalf("esperava 1 evidencia, obteve: %+v", dataEv)
	}

	// 4. Testa /api/eventos
	resEventos, err := http.Get(ts.URL + "/api/eventos?query=Mimikatz")
	if err != nil || resEventos.StatusCode != http.StatusOK {
		t.Fatalf("falha ao acessar /api/eventos: %v", err)
	}
	var dataEventos map[string]interface{}
	json.NewDecoder(resEventos.Body).Decode(&dataEventos)
	if int(dataEventos["total_hits"].(float64)) != 1 {
		t.Fatalf("esperava 1 evento retornado, obteve: %+v", dataEventos)
	}

	// 5. Testa /api/alertas
	resAlertas, err := http.Get(ts.URL + "/api/alertas?severidade=critica")
	if err != nil || resAlertas.StatusCode != http.StatusOK {
		t.Fatalf("falha ao acessar /api/alertas: %v", err)
	}
	var dataAlertas map[string]interface{}
	json.NewDecoder(resAlertas.Body).Decode(&dataAlertas)
	if int(dataAlertas["total"].(float64)) != 1 {
		t.Fatalf("esperava 1 alerta crítico, obteve: %+v", dataAlertas)
	}

	// 6. Testa /api/entidades
	resEntidades, err := http.Get(ts.URL + "/api/entidades?type=ip&query=10.0.0")
	if err != nil || resEntidades.StatusCode != http.StatusOK {
		t.Fatalf("falha ao acessar /api/entidades: %v", err)
	}
	var dataEntidades map[string]interface{}
	json.NewDecoder(resEntidades.Body).Decode(&dataEntidades)
	if int(dataEntidades["total"].(float64)) != 1 {
		t.Fatalf("esperava 1 entidade IP encontrada, obteve: %+v", dataEntidades)
	}

	// 7. Testa /api/custodia
	resCustodia, err := http.Get(ts.URL + "/api/custodia")
	if err != nil || resCustodia.StatusCode != http.StatusOK {
		t.Fatalf("falha ao acessar /api/custodia: %v", err)
	}
	var dataCustodia map[string]interface{}
	json.NewDecoder(resCustodia.Body).Decode(&dataCustodia)
	if dataCustodia["valid"] != true || int(dataCustodia["total_steps"].(float64)) < 2 {
		t.Fatalf("custódia inconsistente: %+v", dataCustodia)
	}

	// 8. Testa POST /api/marcacoes
	payloadMarcacao := `{"evidence_id": 1, "item_id": 10, "bookmark": "Artefato Malicioso", "note": "Mimikatz detectado"}`
	resPostMarc, err := http.Post(ts.URL+"/api/marcacoes", "application/json", bytes.NewBufferString(payloadMarcacao))
	if err != nil || resPostMarc.StatusCode != http.StatusOK {
		t.Fatalf("falha ao postar marcacao: %v", err)
	}

	// 9. Testa GET /api/marcacoes
	resGetMarc, err := http.Get(ts.URL + "/api/marcacoes")
	if err != nil || resGetMarc.StatusCode != http.StatusOK {
		t.Fatalf("falha ao listar marcacoes: %v", err)
	}
	var dataMarc map[string]interface{}
	json.NewDecoder(resGetMarc.Body).Decode(&dataMarc)
	if int(dataMarc["total"].(float64)) != 1 {
		t.Fatalf("esperava 1 marcacao listada, obteve: %+v", dataMarc)
	}
}
