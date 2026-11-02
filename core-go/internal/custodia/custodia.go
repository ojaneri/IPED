package custodia

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Entrada representa um passo no registro imutável de custódia.
type Entrada struct {
	Indice       int64  `json:"indice"`
	Quando       string `json:"quando"`
	Acao         string `json:"acao"`
	Detalhes     string `json:"detalhes"`
	HashAnterior string `json:"hash_anterior"`
	Hash         string `json:"hash"`
}

// Registro gerencia o arquivo de custódia encadeada.
type Registro struct {
	caminho string
	mu      sync.Mutex
	ultimo  string
	indice  int64
}

// Abrir inicializa ou carrega o registro de custódia em um diretório.
func Abrir(dirCaso string) (*Registro, error) {
	caminho := filepath.Join(dirCaso, "custodia.jsonl")
	r := &Registro{
		caminho: caminho,
		ultimo:  "0000000000000000000000000000000000000000000000000000000000000000",
		indice:  0,
	}

	if _, err := os.Stat(caminho); err == nil {
		// Lê as entradas existentes para pegar o último hash e índice
		f, err := os.Open(caminho)
		if err != nil {
			return nil, err
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			var e Entrada
			if err := json.Unmarshal(scanner.Bytes(), &e); err == nil {
				r.ultimo = e.Hash
				r.indice = e.Indice
			}
		}
	}

	return r, nil
}

// Registrar grava um novo evento no arquivo de custódia com hash encadeado.
func (r *Registro) Registrar(acao, detalhes string) (*Entrada, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.indice++
	agora := time.Now().UTC().Format(time.RFC3339)

	conteudoParaHash := fmt.Sprintf("%d|%s|%s|%s|%s", r.indice, agora, acao, detalhes, r.ultimo)
	s := sha256.Sum256([]byte(conteudoParaHash))
	hashCalculado := hex.EncodeToString(s[:])

	entrada := Entrada{
		Indice:       r.indice,
		Quando:       agora,
		Acao:         acao,
		Detalhes:     detalhes,
		HashAnterior: r.ultimo,
		Hash:         hashCalculado,
	}

	dados, err := json.Marshal(entrada)
	if err != nil {
		return nil, err
	}

	f, err := os.OpenFile(r.caminho, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	if _, err := f.Write(append(dados, '\n')); err != nil {
		return nil, err
	}

	r.ultimo = hashCalculado
	return &entrada, nil
}

// Verificar recalcula e valida todo o encadeamento criptográfico do arquivo.
func (r *Registro) Verificar() (int, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	f, err := os.Open(r.caminho)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, true, nil
		}
		return 0, false, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	ultimoEsperado := "0000000000000000000000000000000000000000000000000000000000000000"
	var esperadoIndice int64 = 0
	total := 0

	for scanner.Scan() {
		esperadoIndice++
		total++
		var e Entrada
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			return total, false, fmt.Errorf("linha %d inválida: %w", total, err)
		}

		if e.Indice != esperadoIndice {
			return total, false, fmt.Errorf("quebra de índice na linha %d: esperado %d, obtido %d", total, esperadoIndice, e.Indice)
		}

		if e.HashAnterior != ultimoEsperado {
			return total, false, fmt.Errorf("quebra de elo na linha %d: hash anterior não confere", total)
		}

		conteudo := fmt.Sprintf("%d|%s|%s|%s|%s", e.Indice, e.Quando, e.Acao, e.Detalhes, e.HashAnterior)
		s := sha256.Sum256([]byte(conteudo))
		calculado := hex.EncodeToString(s[:])

		if e.Hash != calculado {
			return total, false, fmt.Errorf("adulteração de conteúdo na linha %d: hash recalculado diverge", total)
		}

		ultimoEsperado = e.Hash
	}

	if err := scanner.Err(); err != nil {
		return total, false, err
	}

	return total, true, nil
}

// UltimoHash retorna o hash da cabeça da cadeia de custódia.
func (r *Registro) UltimoHash() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.ultimo
}

// ErrAdulterado sinaliza falha na conferência de custódia.
var ErrAdulterado = errors.New("registro de custódia adulterado ou corrompido")
