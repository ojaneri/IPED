package custodia

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCustodiaEncadeada(t *testing.T) {
	tmpDir := t.TempDir()

	reg, err := Abrir(tmpDir)
	if err != nil {
		t.Fatalf("falha ao abrir custódia: %v", err)
	}

	// 1. Registra eventos
	_, err = reg.Registrar("criacao_caso", "Caso iniciado pelo perito")
	if err != nil {
		t.Fatalf("falha ao registrar evento 1: %v", err)
	}

	_, err = reg.Registrar("adicao_evidencia", "Evidencia disco.E01 adicionada")
	if err != nil {
		t.Fatalf("falha ao registrar evento 2: %v", err)
	}

	_, err = reg.Registrar("bookmark", "Item #42 marcado como suspeito")
	if err != nil {
		t.Fatalf("falha ao registrar evento 3: %v", err)
	}

	// 2. Verifica cadeia
	total, ok, err := reg.Verificar()
	if err != nil || !ok || total != 3 {
		t.Fatalf("verificação falhou: total=%d, ok=%v, err=%v", total, ok, err)
	}

	// 2.1 Lista entradas
	entradas, err := reg.ListarEntradas()
	if err != nil || len(entradas) != 3 {
		t.Fatalf("falha ao listar entradas: total=%d, err=%v", len(entradas), err)
	}

	// 3. Testa persistência recarregando
	reg2, err := Abrir(tmpDir)
	if err != nil {
		t.Fatalf("falha ao reabrir custódia: %v", err)
	}
	if reg2.UltimoHash() != reg.UltimoHash() {
		t.Fatalf("hash divergente ao recarregar: %s != %s", reg2.UltimoHash(), reg.UltimoHash())
	}

	// 4. Testa detecção de adulteração modificando o arquivo
	caminho := filepath.Join(tmpDir, "custodia.jsonl")
	conteudo, _ := os.ReadFile(caminho)
	conteudoAlterado := []byte(string(conteudo) + "adulteracao_invalida\n")
	os.WriteFile(caminho, conteudoAlterado, 0644)

	_, ok, _ = reg2.Verificar()
	if ok {
		t.Fatalf("verificação deveria ter detectado adulteração mas retornou ok=true")
	}
}
