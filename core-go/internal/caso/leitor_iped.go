package caso

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ResultadoImportacao resume os números do caso IPED importado.
type ResultadoImportacao struct {
	DiretorioIPED string `json:"diretorio_iped"`
	TotalItens    int    `json:"total_itens"`
	Categorias    int    `json:"categorias"`
	Marcacoes     int    `json:"marcacoes"`
	Apagados      int    `json:"apagados"`
	Recuperados   int    `json:"recuperados"`
}

// ImportarCasoIPED lê o FileList.csv de uma pasta de caso processada pelo IPED e o indexa no SQLite do caso.
func (c *Caso) ImportarCasoIPED(dirIPED string, evidenciaID int64) (*ResultadoImportacao, error) {
	caminhoCSV := filepath.Join(dirIPED, "FileList.csv")
	if _, err := os.Stat(caminhoCSV); err != nil {
		return nil, fmt.Errorf("FileList.csv não encontrado no diretório: %s (%w)", dirIPED, err)
	}

	f, err := os.Open(caminhoCSV)
	if err != nil {
		return nil, fmt.Errorf("falha ao abrir FileList.csv: %w", err)
	}
	defer f.Close()

	reader := csv.NewReader(f)
	reader.LazyQuotes = true

	cabecalho, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("falha ao ler cabeçalho do CSV: %w", err)
	}

	colIdx := make(map[string]int)
	for i, col := range cabecalho {
		colIdx[strings.TrimSpace(col)] = i
	}

	tx, err := c.DB.Begin()
	if err != nil {
		return nil, fmt.Errorf("falha ao iniciar transação: %w", err)
	}
	defer tx.Rollback()

	stmtEvento, err := tx.Prepare(`
		INSERT INTO eventos(evidencia_id, data, fonte, descricao, severidade, origem_local)
		VALUES(?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return nil, err
	}
	defer stmtEvento.Close()

	stmtEntidade, err := tx.Prepare(`
		INSERT INTO entidades(evento_id, tipo, valor)
		VALUES(?, ?, ?)
	`)
	if err != nil {
		return nil, err
	}
	defer stmtEntidade.Close()

	stmtMarcacao, err := tx.Prepare(`
		INSERT INTO marcacoes(evidencia_id, item_id, nome, nota, adicionada)
		VALUES(?, ?, ?, ?, ?)
	`)
	if err != nil {
		return nil, err
	}
	defer stmtMarcacao.Close()

	res := &ResultadoImportacao{
		DiretorioIPED: dirIPED,
	}

	categoriasVistas := make(map[string]bool)
	var itemID int64 = 0

	for {
		linha, errRead := reader.Read()
		if errRead != nil {
			if errRead == io.EOF {
				break
			}
			continue
		}

		itemID++
		getCol := func(nome string) string {
			idx, ok := colIdx[nome]
			if ok && idx < len(linha) {
				return strings.TrimSpace(linha[idx])
			}
			return ""
		}

		name := getCol("Name")
		size := getCol("Size")
		ext := getCol("Extension")
		bookmark := getCol("Bookmark")
		cat := getCol("Category")
		md5Hash := getCol("MD5")
		sha1Hash := getCol("SHA1")
		deleted := strings.ToLower(getCol("Deleted"))
		carved := strings.ToLower(getCol("Carved"))
		modified := getCol("Modified")
		created := getCol("Created")
		path := getCol("Path")
		trackID := getCol("TrackId")

		data := modified
		if data == "" {
			data = created
		}

		fonte := cat
		if fonte == "" {
			fonte = "IPED: Arquivo"
		} else {
			categoriasVistas[cat] = true
		}

		severidade := "info"
		if deleted == "true" {
			severidade = "media"
			res.Apagados++
		}
		if carved == "true" {
			severidade = "alta"
			res.Recuperados++
		}

		desc := name
		if path != "" && path != name {
			desc = fmt.Sprintf("%s (%s)", name, path)
		}
		if size != "" && size != "0" {
			desc += fmt.Sprintf(" [%s bytes]", size)
		}

		origemLocal := trackID
		if origemLocal == "" {
			origemLocal = fmt.Sprintf("item:%d", itemID)
		}

		execRes, errExec := stmtEvento.Exec(evidenciaID, data, fonte, desc, severidade, origemLocal)
		if errExec != nil {
			continue
		}

		eventoID, _ := execRes.LastInsertId()
		res.TotalItens++

		// Entidades indexadas
		if md5Hash != "" {
			stmtEntidade.Exec(eventoID, "hash_md5", md5Hash)
		}
		if sha1Hash != "" {
			stmtEntidade.Exec(eventoID, "hash_sha1", sha1Hash)
		}
		if ext != "" {
			stmtEntidade.Exec(eventoID, "extensao", ext)
		}

		// Se tiver bookmark importado
		if bookmark != "" {
			stmtMarcacao.Exec(evidenciaID, itemID, bookmark, "Importado do caso IPED", data)
			res.Marcacoes++
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("falha ao confirmar gravação dos itens: %w", err)
	}

	res.Categorias = len(categoriasVistas)
	c.Custodia.Registrar("caso_iped_importado", fmt.Sprintf("Dir: %s | Itens: %d | Categorias: %d | Bookmarks: %d", dirIPED, res.TotalItens, res.Categorias, res.Marcacoes))

	return res, nil
}
