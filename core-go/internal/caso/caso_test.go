package caso

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCicloDeVidaDoCaso(t *testing.T) {
	tmpDir := t.TempDir()
	pastaCaso := filepath.Join(tmpDir, "caso-01")

	// 1. Criação do caso
	c, err := Novo(pastaCaso, "Operação Alpha", "Perito Federal")
	if err != nil {
		t.Fatalf("falha ao criar caso: %v", err)
	}

	if c.Info.Nome != "Operação Alpha" {
		t.Fatalf("nome divergente: %s", c.Info.Nome)
	}

	// 2. Cria arquivo de teste para simular evidência
	arqEvidencia := filepath.Join(tmpDir, "evidencia_teste.bin")
	os.WriteFile(arqEvidencia, []byte("Conteudo forense de teste 1234567890"), 0644)

	// 3. Adiciona evidência in-situ
	ev, err := c.AdicionarEvidencia(arqEvidencia, true)
	if err != nil {
		t.Fatalf("falha ao adicionar evidência: %v", err)
	}

	if ev.SHA256 == "" || ev.MD5 == "" || ev.SHA1 == "" {
		t.Fatalf("hashes não calculados: %+v", ev)
	}
	if !ev.InSitu {
		t.Fatalf("esperava inSitu=true")
	}

	// 4. Insere eventos e entidades na timeline
	evID, err := c.InserirEvento(ev.ID, "2026-11-02T10:00:00Z", "Log de Sistema", "Login de usuário suspeito admin", "alta", "Security.evtx:102")
	if err != nil {
		t.Fatalf("falha ao inserir evento: %v", err)
	}
	if err := c.InserirEntidade(evID, "usuario", "admin"); err != nil {
		t.Fatalf("falha ao inserir entidade: %v", err)
	}

	// 5. Adiciona marcação (bookmark)
	m, err := c.AdicionarMarcacao(ev.ID, 1, "Login Suspeito", "Acesso fora de horário comercial")
	if err != nil {
		t.Fatalf("falha ao adicionar marcação: %v", err)
	}
	if m.Nome != "Login Suspeito" {
		t.Fatalf("nome da marcação divergente")
	}

	// 6. Busca na timeline
	eventos, total, err := c.BuscarEventos("suspeito", 10, 0)
	if err != nil {
		t.Fatalf("falha na busca de eventos: %v", err)
	}
	if total != 1 || len(eventos) != 1 {
		t.Fatalf("esperava 1 evento, obteve total=%d, len=%d", total, len(eventos))
	}

	// 6.1 Testa ObterEvidencia
	evObtida, err := c.ObterEvidencia(ev.ID)
	if err != nil || evObtida.Nome != "evidencia_teste.bin" {
		t.Fatalf("falha ao obter evidencia: %v", err)
	}

	// 6.2 Testa BuscarAlertas
	alertas, err := c.BuscarAlertas("alta", 10)
	if err != nil || len(alertas) != 1 {
		t.Fatalf("esperava 1 alerta de severidade alta, obteve %d", len(alertas))
	}

	// 6.3 Testa BuscarEntidades
	entidades, err := c.BuscarEntidades("usuario", "admin", 10)
	if err != nil || len(entidades) != 1 || entidades[0].Valor != "admin" {
		t.Fatalf("esperava entidade admin, obteve: %+v", entidades)
	}

	// 7. Fecha o caso e reabre
	if err := c.Fechar(); err != nil {
		t.Fatalf("falha ao fechar: %v", err)
	}

	cReaberto, err := Abrir(pastaCaso)
	if err != nil {
		t.Fatalf("falha ao reabrir caso: %v", err)
	}
	defer cReaberto.Fechar()

	if cReaberto.Info.Nome != "Operação Alpha" {
		t.Fatalf("reabertura perdeu o nome do caso")
	}

	// 8. Verifica custódia
	totalCustodia, ok, err := cReaberto.Custodia.Verificar()
	if err != nil || !ok || totalCustodia < 3 {
		t.Fatalf("custódia inconsistente: total=%d, ok=%v, err=%v", totalCustodia, ok, err)
	}
}
