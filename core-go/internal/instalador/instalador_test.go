package instalador

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestBaixarEVerificarHash(t *testing.T) {
	conteudo := []byte("Arquivo binário pericial de teste para verificação SHA-256")
	s := sha256.Sum256(conteudo)
	hashCorreto := hex.EncodeToString(s[:])

	// Mock HTTP Server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(conteudo)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	ins := NovoInstalador(tmpDir)

	// 1. Teste de sucesso com hash correto
	destino := filepath.Join(tmpDir, "teste.bin")
	err := ins.BaixarEVerificar(server.URL, destino, hashCorreto)
	if err != nil {
		t.Fatalf("falha no download com hash correto: %v", err)
	}

	dadosLidos, _ := os.ReadFile(destino)
	if string(dadosLidos) != string(conteudo) {
		t.Fatalf("conteúdo salvo diverge")
	}

	// 2. Teste de rejeição com hash incorreto
	destinoInvalido := filepath.Join(tmpDir, "invalido.bin")
	errInvalido := ins.BaixarEVerificar(server.URL, destinoInvalido, "0000000000000000000000000000000000000000000000000000000000000000")
	if errInvalido == nil {
		t.Fatalf("deveria ter falhado com hash divergente")
	}
	if _, errStat := os.Stat(destinoInvalido); !os.IsNotExist(errStat) {
		t.Fatalf("arquivo com hash incorreto deveria ter sido excluído")
	}
}

func TestDescompactarZip(t *testing.T) {
	tmpDir := t.TempDir()
	caminhoZip := filepath.Join(tmpDir, "pacote.zip")

	// Cria zip em memória
	fZip, err := os.Create(caminhoZip)
	if err != nil {
		t.Fatalf("falha ao criar zip: %v", err)
	}

	zw := zip.NewWriter(fZip)
	w, _ := zw.Create("ferramenta/binario.exe")
	w.Write([]byte("binario executavel mock"))
	zw.Close()
	fZip.Close()

	// Descompacta
	dirDestino := filepath.Join(tmpDir, "extracao")
	err = DescompactarZip(caminhoZip, dirDestino)
	if err != nil {
		t.Fatalf("falha ao descompactar: %v", err)
	}

	binarioExtraido := filepath.Join(dirDestino, "ferramenta", "binario.exe")
	if _, err := os.Stat(binarioExtraido); err != nil {
		t.Fatalf("arquivo não encontrado após extração: %v", err)
	}
}
