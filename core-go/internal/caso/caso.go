package caso

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/ojaneri/ipedlite/core-go/internal/custodia"
)

// Info guarda os metadados do caso.
type Info struct {
	ID       string `json:"id"`
	Nome     string `json:"nome"`
	Perito   string `json:"perito"`
	CriadoEm string `json:"criado_em"`
	Versao   string `json:"versao"`
}

// Evidencia representa uma evidência registrada no caso.
type Evidencia struct {
	ID         int64  `json:"id"`
	Nome       string `json:"nome"`
	Origem     string `json:"origem"`
	InSitu     bool   `json:"in_situ"`
	Tamanho    int64  `json:"tamanho"`
	SHA256     string `json:"sha256"`
	SHA1       string `json:"sha1"`
	MD5        string `json:"md5"`
	Tipo       string `json:"tipo"`
	Adicionada string `json:"adicionada"`
}

// Evento representa uma linha na timeline consolidada.
type Evento struct {
	ID          int64  `json:"id"`
	EvidenciaID int64  `json:"evidencia_id"`
	Data        string `json:"data"`
	Fonte       string `json:"fonte"`
	Descricao   string `json:"descricao"`
	Severidade  string `json:"severidade"`
	OrigemLocal string `json:"origem_local"`
}

// EntidadeComEvento enriquece a entidade com os dados contextuais do evento associado.
type EntidadeComEvento struct {
	ID          int64  `json:"id"`
	EventoID    int64  `json:"evento_id"`
	Tipo        string `json:"tipo"`
	Valor       string `json:"valor"`
	Data        string `json:"data"`
	Fonte       string `json:"fonte"`
	Descricao   string `json:"descricao"`
	Severidade  string `json:"severidade"`
}

// Marcacao representa um bookmark/tag pericial.
type Marcacao struct {
	ID          int64  `json:"id"`
	EvidenciaID int64  `json:"evidencia_id"`
	ItemID      int64  `json:"item_id"`
	Nome        string `json:"nome"`
	Nota        string `json:"nota"`
	Adicionada  string `json:"adicionada"`
}

// Caso gerencia o ciclo de vida, o banco SQLite e a custódia de um caso pericial.
type Caso struct {
	Dir      string
	Info     Info
	DB       *sql.DB
	Custodia *custodia.Registro
}

const esquema = `
PRAGMA journal_mode=WAL;

CREATE TABLE IF NOT EXISTS info(
  chave TEXT PRIMARY KEY,
  valor TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS evidencias(
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  nome TEXT NOT NULL,
  origem TEXT NOT NULL,
  in_situ INTEGER NOT NULL DEFAULT 1,
  tamanho INTEGER NOT NULL,
  sha256 TEXT NOT NULL,
  sha1 TEXT NOT NULL,
  md5 TEXT NOT NULL,
  tipo TEXT NOT NULL DEFAULT '',
  adicionada TEXT NOT NULL,
  UNIQUE(sha256, origem)
);

CREATE TABLE IF NOT EXISTS eventos(
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  evidencia_id INTEGER REFERENCES evidencias(id),
  data TEXT NOT NULL,
  fonte TEXT NOT NULL,
  descricao TEXT NOT NULL,
  severidade TEXT NOT NULL DEFAULT 'info',
  origem_local TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_eventos_data ON eventos(data);
CREATE INDEX IF NOT EXISTS idx_eventos_fonte ON eventos(fonte);

CREATE TABLE IF NOT EXISTS entidades(
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  evento_id INTEGER REFERENCES eventos(id),
  tipo TEXT NOT NULL,
  valor TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_entidades_tipo_valor ON entidades(tipo, valor);

CREATE TABLE IF NOT EXISTS marcacoes(
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  evidencia_id INTEGER REFERENCES evidencias(id),
  item_id INTEGER NOT NULL DEFAULT 0,
  nome TEXT NOT NULL,
  nota TEXT NOT NULL DEFAULT '',
  adicionada TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS execucoes(
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  modulo TEXT NOT NULL,
  estado TEXT NOT NULL,
  inicio TEXT NOT NULL,
  fim TEXT NOT NULL DEFAULT '',
  registros INTEGER NOT NULL DEFAULT 0,
  erro TEXT NOT NULL DEFAULT ''
);
`

// Novo cria um novo caso forense no diretório especificado.
func Novo(dir, nome, perito string) (*Caso, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("falha ao criar pasta do caso: %w", err)
	}

	caminhoDB := filepath.Join(dir, "caso.db")
	if _, err := os.Stat(caminhoDB); err == nil {
		return nil, errors.New("o caso já existe no diretório informado")
	}

	db, err := sql.Open("sqlite", caminhoDB)
	if err != nil {
		return nil, fmt.Errorf("falha ao abrir banco sqlite: %w", err)
	}

	if _, err := db.Exec(esquema); err != nil {
		db.Close()
		return nil, fmt.Errorf("falha ao aplicar esquema: %w", err)
	}

	agora := time.Now().UTC().Format(time.RFC3339)
	info := Info{
		ID:       fmt.Sprintf("CASO-%d", time.Now().Unix()),
		Nome:     nome,
		Perito:   perito,
		CriadoEm: agora,
		Versao:   "1.0.0",
	}

	// Persiste info no banco
	for k, v := range map[string]string{
		"id":        info.ID,
		"nome":      info.Nome,
		"perito":    info.Perito,
		"criado_em": info.CriadoEm,
		"versao":    info.Versao,
	} {
		if _, err := db.Exec("INSERT INTO info(chave, valor) VALUES(?, ?)", k, v); err != nil {
			db.Close()
			return nil, err
		}
	}

	cust, err := custodia.Abrir(dir)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("falha ao abrir custódia: %w", err)
	}

	detalhes, _ := json.Marshal(info)
	if _, err := cust.Registrar("caso_criado", string(detalhes)); err != nil {
		db.Close()
		return nil, fmt.Errorf("falha ao registrar criação na custódia: %w", err)
	}

	return &Caso{
		Dir:      dir,
		Info:     info,
		DB:       db,
		Custodia: cust,
	}, nil
}

// Abrir carrega um caso forense existente a partir de seu diretório.
func Abrir(dir string) (*Caso, error) {
	caminhoDB := filepath.Join(dir, "caso.db")
	if _, err := os.Stat(caminhoDB); err != nil {
		return nil, fmt.Errorf("caso não encontrado em %s: %w", dir, err)
	}

	db, err := sql.Open("sqlite", caminhoDB)
	if err != nil {
		return nil, fmt.Errorf("falha ao abrir sqlite: %w", err)
	}

	// Carrega info
	rows, err := db.Query("SELECT chave, valor FROM info")
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("falha ao ler info: %w", err)
	}
	defer rows.Close()

	info := Info{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err == nil {
			switch k {
			case "id":
				info.ID = v
			case "nome":
				info.Nome = v
			case "perito":
				info.Perito = v
			case "criado_em":
				info.CriadoEm = v
			case "versao":
				info.Versao = v
			}
		}
	}

	cust, err := custodia.Abrir(dir)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("falha ao abrir custódia: %w", err)
	}

	return &Caso{
		Dir:      dir,
		Info:     info,
		DB:       db,
		Custodia: cust,
	}, nil
}

// Fechar encerra o banco de dados do caso.
func (c *Caso) Fechar() error {
	if c.DB != nil {
		return c.DB.Close()
	}
	return nil
}

// AdicionarEvidencia registra uma nova evidência calculando seus hashes em streaming.
// Se inSitu for true, lê diretamente na origem sem duplicar o arquivo em disco.
func (c *Caso) AdicionarEvidencia(caminho string, inSitu bool) (*Evidencia, error) {
	infoArq, err := os.Stat(caminho)
	if err != nil {
		return nil, fmt.Errorf("arquivo de evidência não acessível: %w", err)
	}

	f, err := os.Open(caminho)
	if err != nil {
		return nil, fmt.Errorf("falha ao abrir evidência para leitura: %w", err)
	}
	defer f.Close()

	// Cálculo simultâneo de SHA256, SHA1 e MD5 em um único passe de leitura
	h256 := sha256.New()
	h1 := sha1.New()
	hmd5 := md5.New()
	mw := io.MultiWriter(h256, h1, hmd5)

	buf := make([]byte, 64*1024)
	var totalBytes int64
	for {
		n, errRead := f.Read(buf)
		if n > 0 {
			mw.Write(buf[:n])
			totalBytes += int64(n)
		}
		if errRead != nil {
			if errRead == io.EOF {
				break
			}
			return nil, fmt.Errorf("erro lendo evidência: %w", errRead)
		}
	}

	hashSHA256 := hex.EncodeToString(h256.Sum(nil))
	hashSHA1 := hex.EncodeToString(h1.Sum(nil))
	hashMD5 := hex.EncodeToString(hmd5.Sum(nil))

	nome := filepath.Base(caminho)
	ext := strings.ToLower(filepath.Ext(caminho))
	tipo := "arquivo"
	if ext == ".e01" || ext == ".dd" || ext == ".raw" || ext == ".img" {
		tipo = "disco"
	} else if ext == ".ufdr" || ext == ".zip" || ext == ".tar" || ext == ".7z" {
		tipo = "pacote"
	}

	agora := time.Now().UTC().Format(time.RFC3339)
	inSituInt := 1
	if !inSitu {
		inSituInt = 0
	}

	res, err := c.DB.Exec(`
		INSERT INTO evidencias(nome, origem, in_situ, tamanho, sha256, sha1, md5, tipo, adicionada)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(sha256, origem) DO UPDATE SET tamanho=excluded.tamanho
	`, nome, caminho, inSituInt, totalBytes, hashSHA256, hashSHA1, hashMD5, tipo, agora)
	if err != nil {
		return nil, fmt.Errorf("falha ao gravar evidência no banco: %w", err)
	}

	id, _ := res.LastInsertId()

	ev := &Evidencia{
		ID:         id,
		Nome:       nome,
		Origem:     caminho,
		InSitu:     inSitu,
		Tamanho:    infoArq.Size(),
		SHA256:     hashSHA256,
		SHA1:       hashSHA1,
		MD5:        hashMD5,
		Tipo:       tipo,
		Adicionada: agora,
	}

	detalhes, _ := json.Marshal(ev)
	c.Custodia.Registrar("evidencia_adicionada", string(detalhes))

	return ev, nil
}

// ListarEvidencias retorna todas as evidências registradas no caso.
func (c *Caso) ListarEvidencias() ([]Evidencia, error) {
	rows, err := c.DB.Query("SELECT id, nome, origem, in_situ, tamanho, sha256, sha1, md5, tipo, adicionada FROM evidencias ORDER BY id ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lista []Evidencia
	for rows.Next() {
		var e Evidencia
		var inSituInt int
		if err := rows.Scan(&e.ID, &e.Nome, &e.Origem, &inSituInt, &e.Tamanho, &e.SHA256, &e.SHA1, &e.MD5, &e.Tipo, &e.Adicionada); err == nil {
			e.InSitu = inSituInt == 1
			lista = append(lista, e)
		}
	}
	return lista, nil
}

// ObterEvidencia busca uma evidência específica pelo seu ID.
func (c *Caso) ObterEvidencia(id int64) (*Evidencia, error) {
	row := c.DB.QueryRow("SELECT id, nome, origem, in_situ, tamanho, sha256, sha1, md5, tipo, adicionada FROM evidencias WHERE id = ?", id)
	var e Evidencia
	var inSituInt int
	if err := row.Scan(&e.ID, &e.Nome, &e.Origem, &inSituInt, &e.Tamanho, &e.SHA256, &e.SHA1, &e.MD5, &e.Tipo, &e.Adicionada); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("evidência id %d não encontrada", id)
		}
		return nil, err
	}
	e.InSitu = inSituInt == 1
	return &e, nil
}

// BuscarAlertas retorna eventos filtrados por nível de severidade (padrão 'critica' e 'alta').
func (c *Caso) BuscarAlertas(severidade string, limite int) ([]Evento, error) {
	if limite <= 0 {
		limite = 100
	}
	query := "SELECT id, evidencia_id, data, fonte, descricao, severidade, origem_local FROM eventos WHERE "
	var args []interface{}
	if severidade != "" && severidade != "*" {
		query += "LOWER(severidade) = LOWER(?) "
		args = append(args, severidade)
	} else {
		query += "LOWER(severidade) IN ('critica', 'alta', 'critico', 'alto', 'critical', 'high') "
	}
	query += "ORDER BY CASE LOWER(severidade) WHEN 'critica' THEN 1 WHEN 'critical' THEN 1 WHEN 'alta' THEN 2 WHEN 'high' THEN 2 ELSE 3 END, data DESC LIMIT ?"
	args = append(args, limite)

	rows, err := c.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lista []Evento
	for rows.Next() {
		var ev Evento
		if err := rows.Scan(&ev.ID, &ev.EvidenciaID, &ev.Data, &ev.Fonte, &ev.Descricao, &ev.Severidade, &ev.OrigemLocal); err == nil {
			lista = append(lista, ev)
		}
	}
	return lista, nil
}

// BuscarEntidades localiza entidades extraídas correlacionadas com seus respectivos eventos.
func (c *Caso) BuscarEntidades(tipo, valor string, limite int) ([]EntidadeComEvento, error) {
	if limite <= 0 {
		limite = 100
	}
	query := `
		SELECT e.id, e.evento_id, e.tipo, e.valor, ev.data, ev.fonte, ev.descricao, ev.severidade
		FROM entidades e
		JOIN eventos ev ON e.evento_id = ev.id
	`
	var whereParts []string
	var args []interface{}
	if tipo != "" && tipo != "*" {
		whereParts = append(whereParts, "e.tipo = ?")
		args = append(args, tipo)
	}
	if valor != "" && valor != "*" {
		whereParts = append(whereParts, "e.valor LIKE ?")
		args = append(args, "%"+valor+"%")
	}
	if len(whereParts) > 0 {
		query += " WHERE " + strings.Join(whereParts, " AND ")
	}
	query += " ORDER BY ev.data DESC LIMIT ?"
	args = append(args, limite)

	rows, err := c.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lista []EntidadeComEvento
	for rows.Next() {
		var item EntidadeComEvento
		if err := rows.Scan(&item.ID, &item.EventoID, &item.Tipo, &item.Valor, &item.Data, &item.Fonte, &item.Descricao, &item.Severidade); err == nil {
			lista = append(lista, item)
		}
	}
	return lista, nil
}

// InserirEvento grava um evento na timeline consolidada do caso.
func (c *Caso) InserirEvento(evidenciaID int64, data, fonte, descricao, severidade, origemLocal string) (int64, error) {
	res, err := c.DB.Exec(`
		INSERT INTO eventos(evidencia_id, data, fonte, descricao, severidade, origem_local)
		VALUES(?, ?, ?, ?, ?, ?)
	`, evidenciaID, data, fonte, descricao, severidade, origemLocal)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// InserirEntidade vincula uma entidade descoberta (ex: IP, e-mail) a um evento.
func (c *Caso) InserirEntidade(eventoID int64, tipo, valor string) error {
	_, err := c.DB.Exec("INSERT INTO entidades(evento_id, tipo, valor) VALUES(?, ?, ?)", eventoID, tipo, valor)
	return err
}

// BuscarEventos realiza consulta na timeline de eventos com filtros e paginação.
func (c *Caso) BuscarEventos(termo string, limite, offset int) ([]Evento, int, error) {
	if limite <= 0 {
		limite = 50
	}
	if offset < 0 {
		offset = 0
	}

	whereClause := ""
	args := []interface{}{}
	if termo != "" && termo != "*" {
		whereClause = "WHERE descricao LIKE ? OR fonte LIKE ?"
		args = append(args, "%"+termo+"%", "%"+termo+"%")
	}

	// Contagem total
	var total int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM eventos %s", whereClause)
	if err := c.DB.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Busca paginada
	query := fmt.Sprintf("SELECT id, evidencia_id, data, fonte, descricao, severidade, origem_local FROM eventos %s ORDER BY data DESC LIMIT ? OFFSET ?", whereClause)
	argsQuery := append(args, limite, offset)

	rows, err := c.DB.Query(query, argsQuery...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var eventos []Evento
	for rows.Next() {
		var ev Evento
		if err := rows.Scan(&ev.ID, &ev.EvidenciaID, &ev.Data, &ev.Fonte, &ev.Descricao, &ev.Severidade, &ev.OrigemLocal); err == nil {
			eventos = append(eventos, ev)
		}
	}

	return eventos, total, nil
}

// AdicionarMarcacao registra uma marcação (bookmark) em um item pericial.
func (c *Caso) AdicionarMarcacao(evidenciaID, itemID int64, nome, nota string) (*Marcacao, error) {
	agora := time.Now().UTC().Format(time.RFC3339)
	res, err := c.DB.Exec(`
		INSERT INTO marcacoes(evidencia_id, item_id, nome, nota, adicionada)
		VALUES(?, ?, ?, ?, ?)
	`, evidenciaID, itemID, nome, nota, agora)
	if err != nil {
		return nil, err
	}

	id, _ := res.LastInsertId()
	m := &Marcacao{
		ID:          id,
		EvidenciaID: evidenciaID,
		ItemID:      itemID,
		Nome:        nome,
		Nota:        nota,
		Adicionada:  agora,
	}

	detalhes, _ := json.Marshal(m)
	c.Custodia.Registrar("marcacao_adicionada", string(detalhes))

	return m, nil
}

// ListarMarcacoes lista todos os bookmarks existentes no caso.
func (c *Caso) ListarMarcacoes() ([]Marcacao, error) {
	rows, err := c.DB.Query("SELECT id, evidencia_id, item_id, nome, nota, adicionada FROM marcacoes ORDER BY id ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lista []Marcacao
	for rows.Next() {
		var m Marcacao
		if err := rows.Scan(&m.ID, &m.EvidenciaID, &m.ItemID, &m.Nome, &m.Nota, &m.Adicionada); err == nil {
			lista = append(lista, m)
		}
	}
	return lista, nil
}
