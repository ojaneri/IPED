package modulo

import (
	"bufio"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ojaneri/ipedlite/core-go/internal/caso"
)

// Executor executa os comandos declarados nos manifestos e processa as saídas.
type Executor struct {
	DirFerramentas string
}

// NovoExecutor instancia o executor de módulos.
func NovoExecutor(dirFerramentas string) *Executor {
	return &Executor{
		DirFerramentas: dirFerramentas,
	}
}

// Executar roda o comando definido no manifesto para a plataforma atual.
func (e *Executor) Executar(ctx context.Context, m *Manifesto, caminhoEntrada, dirSaida string) error {
	plataforma := runtime.GOOS // "windows" ou "linux"
	cmdCfg, ok := m.Comando[plataforma]
	if !ok {
		return fmt.Errorf("módulo %s não possui comando para a plataforma %s", m.Nome, plataforma)
	}

	if err := os.MkdirAll(dirSaida, 0755); err != nil {
		return fmt.Errorf("falha ao criar pasta de saída do módulo: %w", err)
	}

	vars := map[string]string{
		"entrada":    caminhoEntrada,
		"saida":      dirSaida,
		"ferramenta": filepath.Join(e.DirFerramentas, m.Nome),
		"modulo":     m.DirOrigem,
	}

	execCmd := Interpolar(cmdCfg.Exec, vars)
	args := make([]string, len(cmdCfg.Args))
	for i, a := range cmdCfg.Args {
		args[i] = Interpolar(a, vars)
	}

	cmd := exec.CommandContext(ctx, execCmd, args...)
	if cmdCfg.Dir != "" {
		cmd.Dir = Interpolar(cmdCfg.Dir, vars)
	}

	// Se houver arquivo de stdout configurado
	if cmdCfg.Stdout != "" {
		caminhoStdout := filepath.Join(dirSaida, cmdCfg.Stdout)
		fOut, err := os.Create(caminhoStdout)
		if err != nil {
			return fmt.Errorf("falha ao criar arquivo de stdout: %w", err)
		}
		defer fOut.Close()
		cmd.Stdout = fOut
		cmd.Stderr = os.Stderr
	} else {
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
	}

	return cmd.Run()
}

// ProcessarSaida lê os arquivos gerados pelo módulo e normaliza os registros no banco do caso.
func (e *Executor) ProcessarSaida(c *caso.Caso, evidenciaID int64, m *Manifesto, dirSaida string) (int, error) {
	padrao := m.Saida.Arquivos
	if padrao == "" {
		return 0, nil
	}

	caminhoArquivo := filepath.Join(dirSaida, padrao)
	matches, err := filepath.Glob(caminhoArquivo)
	if err != nil || len(matches) == 0 {
		return 0, fmt.Errorf("nenhum arquivo de saída encontrado para o padrão: %s", caminhoArquivo)
	}

	totalRegistros := 0
	formato := strings.ToLower(m.Saida.Formato)

	for _, arquivo := range matches {
		switch formato {
		case "csv":
			qtd, errCSV := e.processarCSV(c, evidenciaID, m, arquivo)
			if errCSV != nil {
				return totalRegistros, errCSV
			}
			totalRegistros += qtd

		case "jsonl":
			qtd, errJSONL := e.processarJSONL(c, evidenciaID, m, arquivo)
			if errJSONL != nil {
				return totalRegistros, errJSONL
			}
			totalRegistros += qtd
		}
	}

	return totalRegistros, nil
}

func (e *Executor) processarCSV(c *caso.Caso, evidenciaID int64, m *Manifesto, caminhoCSV string) (int, error) {
	f, err := os.Open(caminhoCSV)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	reader := csv.NewReader(f)
	reader.LazyQuotes = true

	cabecalho, err := reader.Read()
	if err != nil {
		return 0, err
	}

	indicesColunas := make(map[string]int)
	for i, col := range cabecalho {
		indicesColunas[strings.TrimSpace(col)] = i
	}

	tx, err := c.DB.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	stmtEvento, err := tx.Prepare(`
		INSERT INTO eventos(evidencia_id, data, fonte, descricao, severidade, origem_local)
		VALUES(?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return 0, err
	}
	defer stmtEvento.Close()

	stmtEntidade, err := tx.Prepare(`
		INSERT INTO entidades(evento_id, tipo, valor)
		VALUES(?, ?, ?)
	`)
	if err != nil {
		return 0, err
	}
	defer stmtEntidade.Close()

	total := 0
	for {
		linha, errRead := reader.Read()
		if errRead != nil {
			if errRead == io.EOF {
				break
			}
			continue
		}

		// Cria mapa de valores da linha
		valores := make(map[string]string)
		for col, idx := range indicesColunas {
			if idx < len(linha) {
				valores[col] = linha[idx]
			}
		}

		for _, mapa := range m.Saida.Mapas {
			quando := Interpolar(mapa.Quando, valores)
			fonte := Interpolar(mapa.Fonte, valores)
			desc := Interpolar(mapa.Descricao, valores)
			sev := Interpolar(mapa.Severidade, valores)
			if sev == "" {
				sev = "info"
			}
			origemLocal := Interpolar(mapa.OrigemLocal, valores)

			res, errExec := stmtEvento.Exec(evidenciaID, quando, fonte, desc, sev, origemLocal)
			if errExec == nil {
				eventoID, _ := res.LastInsertId()
				total++

				// Insere entidades mapeadas
				for _, ent := range mapa.Entidades {
					tipoEnt := Interpolar(ent.Tipo, valores)
					valEnt := Interpolar(ent.Valor, valores)
					if valEnt != "" {
						stmtEntidade.Exec(eventoID, tipoEnt, valEnt)
					}
				}
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	return total, nil
}

func (e *Executor) processarJSONL(c *caso.Caso, evidenciaID int64, m *Manifesto, caminhoJSONL string) (int, error) {
	f, err := os.Open(caminhoJSONL)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, 10*1024*1024)

	tx, err := c.DB.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	stmtEvento, err := tx.Prepare(`
		INSERT INTO eventos(evidencia_id, data, fonte, descricao, severidade, origem_local)
		VALUES(?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return 0, err
	}
	defer stmtEvento.Close()

	stmtEntidade, err := tx.Prepare(`
		INSERT INTO entidades(evento_id, tipo, valor)
		VALUES(?, ?, ?)
	`)
	if err != nil {
		return 0, err
	}
	defer stmtEntidade.Close()

	total := 0
	for scanner.Scan() {
		var registro map[string]interface{}
		if err := json.Unmarshal(scanner.Bytes(), &registro); err != nil {
			continue
		}

		// Converte valores para strings para interpolação
		valores := make(map[string]string)
		for k, v := range registro {
			valores[k] = fmt.Sprintf("%v", v)
		}

		for _, mapa := range m.Saida.Mapas {
			quando := Interpolar(mapa.Quando, valores)
			fonte := Interpolar(mapa.Fonte, valores)
			desc := Interpolar(mapa.Descricao, valores)
			sev := Interpolar(mapa.Severidade, valores)
			if sev == "" {
				sev = "info"
			}
			origemLocal := Interpolar(mapa.OrigemLocal, valores)

			res, errExec := stmtEvento.Exec(evidenciaID, quando, fonte, desc, sev, origemLocal)
			if errExec == nil {
				eventoID, _ := res.LastInsertId()
				total++

				for _, ent := range mapa.Entidades {
					tipoEnt := Interpolar(ent.Tipo, valores)
					valEnt := Interpolar(ent.Valor, valores)
					if valEnt != "" {
						stmtEntidade.Exec(eventoID, tipoEnt, valEnt)
					}
				}
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	return total, nil
}
