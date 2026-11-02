package modulo

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Reconhecimento ensina o motor a identificar o tipo de evidência por assinatura ou extensão.
type Reconhecimento struct {
	Tipo      string   `yaml:"tipo"`
	MagicaHex string   `yaml:"magica_hex"`
	Extensoes []string `yaml:"extensoes"`
}

// Instalacao traz o link e hash do binário oficial da ferramenta para download seguro.
type Instalacao struct {
	URL    string `yaml:"url"`
	SHA256 string `yaml:"sha256"`
}

// Comando detalha o executável e argumentos para uma plataforma (windows/linux).
type Comando struct {
	Exec   string   `yaml:"exec"`
	Dir    string   `yaml:"dir"`
	Args   []string `yaml:"args"`
	Stdout string   `yaml:"stdout"`
}

// MapaEntidade extrai entidades (IP, e-mail, hash, etc.) a partir dos campos do resultado.
type MapaEntidade struct {
	Tipo  string `yaml:"tipo"`
	Valor string `yaml:"valor"`
}

// MapaRegra normaliza registros de saída do módulo para a timeline unificada.
type MapaRegra struct {
	Tipo        string         `yaml:"tipo"`
	Quando      string         `yaml:"quando"`
	Fonte       string         `yaml:"fonte"`
	Descricao   string         `yaml:"descricao"`
	Severidade  string         `yaml:"severidade"`
	OrigemLocal string         `yaml:"origem_local"`
	Entidades   []MapaEntidade `yaml:"entidades"`
}

// SaidaConfig define o formato gerado pela ferramenta (CSV ou JSONL) e as regras de mapeamento.
type SaidaConfig struct {
	Arquivos string      `yaml:"arquivos"`
	Formato  string      `yaml:"formato"`
	Mapas    []MapaRegra `yaml:"mapas"`
}

// Manifesto representa o contrato declarativo completo de um módulo pericial (modulo.yaml).
type Manifesto struct {
	Contrato  int                   `yaml:"contrato"`
	Nome      string                `yaml:"nome"`
	Titulo    string                `yaml:"titulo"`
	Versao    string                `yaml:"versao"`
	Opcional  bool                  `yaml:"opcional"`
	Reconhece []Reconhecimento      `yaml:"reconhece"`
	Consome   []string              `yaml:"consome"`
	Modo      string                `yaml:"modo"` // arquivo, lote, imagem
	Instalar  map[string]Instalacao `yaml:"instalar"`
	Comando   map[string]Comando    `yaml:"comando"`
	Saida     SaidaConfig           `yaml:"saida"`
	DirOrigem string                `yaml:"-"`
}

// CarregarManifesto lê e valida um arquivo modulo.yaml.
func CarregarManifesto(caminhoArquivo string) (*Manifesto, error) {
	dados, err := os.ReadFile(caminhoArquivo)
	if err != nil {
		return nil, fmt.Errorf("falha ao ler manifesto %s: %w", caminhoArquivo, err)
	}

	var m Manifesto
	if err := yaml.Unmarshal(dados, &m); err != nil {
		return nil, fmt.Errorf("falha ao interpretar YAML %s: %w", caminhoArquivo, err)
	}

	m.DirOrigem = filepath.Dir(caminhoArquivo)
	return &m, nil
}

// CarregarModulos carrega recursivamente todos os arquivos modulo.yaml de um diretório.
func CarregarModulos(dirModulos string) ([]*Manifesto, error) {
	var modulos []*Manifesto

	if _, err := os.Stat(dirModulos); os.IsNotExist(err) {
		return modulos, nil
	}

	err := filepath.Walk(dirModulos, func(caminho string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && (info.Name() == "modulo.yaml" || info.Name() == "modulo.yml") {
			m, errCarregar := CarregarManifesto(caminho)
			if errCarregar == nil && m.Nome != "" {
				modulos = append(modulos, m)
			}
		}
		return nil
	})

	return modulos, err
}

// IdentificarTipo examina a assinatura mágica e a extensão para determinar o tipo da evidência.
func IdentificarTipo(caminho string, modulos []*Manifesto) (string, error) {
	f, err := os.Open(caminho)
	if err != nil {
		return "desconhecido", err
	}
	defer f.Close()

	// Lê os primeiros 64 bytes para conferência de magic bytes
	cabecalho := make([]byte, 64)
	n, _ := f.Read(cabecalho)
	cabecalhoHex := hex.EncodeToString(cabecalho[:n])

	ext := strings.ToLower(filepath.Ext(caminho))

	// 1. Tenta identificar por assinatura mágica (prioridade mais alta)
	for _, m := range modulos {
		for _, r := range m.Reconhece {
			if r.MagicaHex != "" {
				magicaLimpa := strings.ToLower(strings.ReplaceAll(r.MagicaHex, " ", ""))
				if strings.HasPrefix(cabecalhoHex, magicaLimpa) {
					return r.Tipo, nil
				}
			}
		}
	}

	// 2. Tenta identificar por extensão de arquivo
	for _, m := range modulos {
		for _, r := range m.Reconhece {
			for _, e := range r.Extensoes {
				if strings.EqualFold(ext, e) {
					return r.Tipo, nil
				}
			}
		}
	}

	// 3. Fallbacks comuns conhecidos
	switch ext {
	case ".e01", ".dd", ".raw", ".img", ".vmdk", ".vhd":
		return "disco", nil
	case ".zip", ".tar", ".gz", ".7z", ".ufdr":
		return "pacote", nil
	case ".evtx":
		return "log_eventos", nil
	default:
		return "arquivo", nil
	}
}

// Interpolar substitui variáveis {entrada}, {saida}, {ferramenta} nos argumentos.
func Interpolar(template string, vars map[string]string) string {
	resultado := template
	for k, v := range vars {
		resultado = strings.ReplaceAll(resultado, "{"+k+"}", v)
	}
	return resultado
}
