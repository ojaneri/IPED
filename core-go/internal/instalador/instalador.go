package instalador

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/ojaneri/ipedlite/core-go/internal/modulo"
)

// StatusInstalacao resume o estado de uma ferramenta.
type StatusInstalacao struct {
	Modulo    string `json:"modulo"`
	Instalado bool   `json:"instalado"`
	Caminho   string `json:"caminho,omitempty"`
	Erro      string `json:"erro,omitempty"`
}

// Instalador gerencia o download com verificação criptográfica das ferramentas.
type Instalador struct {
	DirFerramentas string
	ClienteHTTP    *http.Client
}

// NovoInstalador cria uma nova instância do gerenciador de instalação.
func NovoInstalador(dirFerramentas string) *Instalador {
	if dirFerramentas == "" {
		dirFerramentas = "ferramentas"
	}
	return &Instalador{
		DirFerramentas: dirFerramentas,
		ClienteHTTP: &http.Client{
			Timeout: 10 * time.Minute,
		},
	}
}

// ChavePlataforma retorna a chave no padrão do manifesto (ex: "windows-amd64", "linux-amd64").
func ChavePlataforma() string {
	return fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH)
}

// BaixarEVerificar faz o download de uma URL e verifica o hash SHA-256 em streaming.
func (ins *Instalador) BaixarEVerificar(url, caminhoDestino, sha256Esperado string) error {
	resp, err := ins.ClienteHTTP.Get(url)
	if err != nil {
		return fmt.Errorf("falha ao conectar na URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("servidor retornou HTTP %d (%s)", resp.StatusCode, resp.Status)
	}

	if err := os.MkdirAll(filepath.Dir(caminhoDestino), 0755); err != nil {
		return err
	}

	f, err := os.Create(caminhoDestino)
	if err != nil {
		return fmt.Errorf("falha ao criar arquivo de destino: %w", err)
	}

	hasher := sha256.New()
	mw := io.MultiWriter(f, hasher)

	_, errCopy := io.Copy(mw, resp.Body)
	f.Close() // Fecha o descritor antes de qualquer tentativa de validação ou exclusão no Windows

	if errCopy != nil {
		os.Remove(caminhoDestino)
		return fmt.Errorf("erro durante o download: %w", errCopy)
	}

	hashCalculado := hex.EncodeToString(hasher.Sum(nil))
	if sha256Esperado != "" && !strings.EqualFold(hashCalculado, sha256Esperado) {
		os.Remove(caminhoDestino)
		return fmt.Errorf("hash SHA-256 inválido! Esperado: %s, Calculado: %s", sha256Esperado, hashCalculado)
	}

	return nil
}

// DescompactarZip extrai um arquivo zip com segurança.
func DescompactarZip(caminhoZip, dirDestino string) error {
	r, err := zip.OpenReader(caminhoZip)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		caminhoCompleto := filepath.Join(dirDestino, f.Name)

		// Proteção contra Zip Slip
		if !strings.HasPrefix(filepath.Clean(caminhoCompleto), filepath.Clean(dirDestino)+string(filepath.Separator)) {
			continue
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(caminhoCompleto, 0755)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(caminhoCompleto), 0755); err != nil {
			return err
		}

		dstFile, err := os.OpenFile(caminhoCompleto, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		srcFile, err := f.Open()
		if err != nil {
			dstFile.Close()
			return err
		}

		_, errCopy := io.Copy(dstFile, srcFile)
		srcFile.Close()
		dstFile.Close()
		if errCopy != nil {
			return errCopy
		}
	}
	return nil
}

// PrepararModulo baixa e prepara a ferramenta para um módulo específico se aplicável.
func (ins *Instalador) PrepararModulo(m *modulo.Manifesto) StatusInstalacao {
	status := StatusInstalacao{Modulo: m.Nome}

	plataforma := ChavePlataforma()
	spec, ok := m.Instalar[plataforma]
	if !ok || spec.URL == "" {
		// Módulo não tem download configurado ou usa ferramenta do sistema
		status.Instalado = true
		status.Caminho = "sistema/local"
		return status
	}

	dirModuloFerramenta := filepath.Join(ins.DirFerramentas, m.Nome)
	if _, err := os.Stat(dirModuloFerramenta); err == nil {
		// Já instalado
		status.Instalado = true
		status.Caminho = dirModuloFerramenta
		return status
	}

	nomeArquivo := filepath.Base(spec.URL)
	caminhoTemporario := filepath.Join(ins.DirFerramentas, "downloads", nomeArquivo)

	err := ins.BaixarEVerificar(spec.URL, caminhoTemporario, spec.SHA256)
	if err != nil {
		status.Erro = err.Error()
		return status
	}

	// Extrai se for ZIP
	if strings.HasSuffix(strings.ToLower(nomeArquivo), ".zip") {
		if err := DescompactarZip(caminhoTemporario, dirModuloFerramenta); err != nil {
			status.Erro = fmt.Sprintf("falha ao descompactar: %v", err)
			return status
		}
	} else {
		// Binário individual direto
		os.MkdirAll(dirModuloFerramenta, 0755)
		destinoBin := filepath.Join(dirModuloFerramenta, nomeArquivo)
		os.Rename(caminhoTemporario, destinoBin)
	}

	status.Instalado = true
	status.Caminho = dirModuloFerramenta
	return status
}

// PrepararTodos prepara todas as ferramentas declaradas nos manifestos.
func (ins *Instalador) PrepararTodos(modulos []*modulo.Manifesto) []StatusInstalacao {
	var resultados []StatusInstalacao
	for _, m := range modulos {
		resultados = append(resultados, ins.PrepararModulo(m))
	}
	return resultados
}
