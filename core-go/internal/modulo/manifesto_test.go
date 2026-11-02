package modulo

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ojaneri/ipedlite/core-go/internal/caso"
)

func TestCarregarEProcessarModulo(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. Cria um modulo.yaml de teste
	conteudoYAML := `
contrato: 1
nome: teste-ferramenta
titulo: Ferramenta de Teste
versao: "1.0"
reconhece:
  - tipo: disco
    extensoes: [.raw, .dd]
consome: [disco]
modo: arquivo
comando:
  windows:
    exec: cmd.exe
    args: ["/c", "echo ok"]
saida:
  arquivos: "resultado.csv"
  formato: csv
  mapas:
    - tipo: evento
      quando: "{Data}"
      fonte: "Teste: {Categoria}"
      descricao: "{Descricao}"
      severidade: "{Severidade}"
      origem_local: "{Linha}"
      entidades:
        - tipo: usuario
          valor: "{Usuario}"
`
	caminhoYAML := filepath.Join(tmpDir, "modulo.yaml")
	os.WriteFile(caminhoYAML, []byte(conteudoYAML), 0644)

	m, err := CarregarManifesto(caminhoYAML)
	if err != nil {
		t.Fatalf("falha ao carregar manifesto: %v", err)
	}
	if m.Nome != "teste-ferramenta" {
		t.Fatalf("nome incorreto: %s", m.Nome)
	}

	// 2. Simula arquivo CSV de saída da ferramenta
	caminhoCSV := filepath.Join(tmpDir, "resultado.csv")
	conteudoCSV := "Data,Categoria,Descricao,Severidade,Linha,Usuario\n2026-11-02T12:00:00Z,Alerta,Execucao suspeita mimikatz.exe,alta,1001,admin\n"
	os.WriteFile(caminhoCSV, []byte(conteudoCSV), 0644)

	// 3. Cria caso e processa a saída usando o executor
	pastaCaso := filepath.Join(tmpDir, "caso-teste")
	c, err := caso.Novo(pastaCaso, "Caso Teste Módulo", "Perito")
	if err != nil {
		t.Fatalf("falha ao criar caso: %v", err)
	}
	defer c.Fechar()

	executor := NovoExecutor(tmpDir)
	qtd, err := executor.ProcessarSaida(c, 1, m, tmpDir)
	if err != nil {
		t.Fatalf("falha ao processar saída: %v", err)
	}
	if qtd != 1 {
		t.Fatalf("esperava 1 registro processado, obteve %d", qtd)
	}

	// 4. Confere se o evento foi inserido no SQLite
	eventos, total, err := c.BuscarEventos("mimikatz", 10, 0)
	if err != nil || total != 1 || len(eventos) != 1 {
		t.Fatalf("evento não encontrado na timeline: total=%d, err=%v", total, err)
	}
	if eventos[0].Severidade != "alta" {
		t.Fatalf("severidade incorreta: %s", eventos[0].Severidade)
	}

	_ = context.Background()
}
