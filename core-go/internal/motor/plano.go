package motor

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/ojaneri/ipedlite/core-go/internal/caso"
	"github.com/ojaneri/ipedlite/core-go/internal/modulo"
)

// ItemExecutado traz o resumo da execução de um módulo no plano.
type ItemExecutado struct {
	Modulo     string `json:"modulo"`
	Titulo     string `json:"titulo"`
	Sucesso    bool   `json:"sucesso"`
	Registros  int    `json:"registros"`
	Erro       string `json:"erro,omitempty"`
	DuracaoSeg float64 `json:"duracao_seg"`
}

// ResultadoPlano resume a execução completa do plano de triagem.
type ResultadoPlano struct {
	EvidenciaID     int64           `json:"evidencia_id"`
	TipoEvidencia   string          `json:"tipo_evidencia"`
	TotalModulos    int             `json:"total_modulos"`
	TotalRegistros  int             `json:"total_registros"`
	ItensExecutados []ItemExecutado `json:"executados"`
}

// MontarPlano analisa o tipo da evidência e seleciona os módulos compatíveis.
func MontarPlano(tipoEvidencia string, modulos []*modulo.Manifesto, incluirProfundo bool) []*modulo.Manifesto {
	var planoRapido []*modulo.Manifesto
	var planoProfundo []*modulo.Manifesto

	for _, m := range modulos {
		compativel := false
		for _, c := range m.Consome {
			if c == tipoEvidencia || c == "*" {
				compativel = true
				break
			}
		}

		if compativel {
			if m.Opcional {
				if incluirProfundo {
					planoProfundo = append(planoProfundo, m)
				}
			} else {
				planoRapido = append(planoRapido, m)
			}
		}
	}

	// Triagem rápida primeiro, exames profundos depois
	return append(planoRapido, planoProfundo...)
}

// ExecutarPlano roda a sequência de módulos planejados e normaliza todos os registros no SQLite.
func ExecutarPlano(ctx context.Context, c *caso.Caso, ev *caso.Evidencia, plano []*modulo.Manifesto, dirFerramentas string) (*ResultadoPlano, error) {
	executor := modulo.NovoExecutor(dirFerramentas)
	resPlano := &ResultadoPlano{
		EvidenciaID:   ev.ID,
		TipoEvidencia: ev.Tipo,
		TotalModulos:  len(plano),
	}

	dirSaidaBase := filepath.Join(c.Dir, "execucoes", fmt.Sprintf("evidencia_%d", ev.ID))

	for _, m := range plano {
		dirSaidaModulo := filepath.Join(dirSaidaBase, m.Nome)
		inicio := time.Now()

		item := ItemExecutado{
			Modulo: m.Nome,
			Titulo: m.Titulo,
		}

		// Registra início na tabela execucoes
		resExec, _ := c.DB.Exec(`
			INSERT INTO execucoes(modulo, estado, inicio)
			VALUES(?, 'em_execucao', ?)
		`, m.Nome, inicio.UTC().Format(time.RFC3339))
		execID, _ := resExec.LastInsertId()

		// 1. Executa a ferramenta
		errExec := executor.Executar(ctx, m, ev.Origem, dirSaidaModulo)
		fim := time.Now()
		item.DuracaoSeg = fim.Sub(inicio).Seconds()

		if errExec != nil {
			item.Sucesso = false
			item.Erro = errExec.Error()
			c.DB.Exec(`
				UPDATE execucoes SET estado='falha', fim=?, erro=? WHERE id=?
			`, fim.UTC().Format(time.RFC3339), item.Erro, execID)

			c.Custodia.Registrar("modulo_falha", fmt.Sprintf("Módulo: %s | Erro: %s", m.Nome, item.Erro))
			resPlano.ItensExecutados = append(resPlano.ItensExecutados, item)
			continue
		}

		// 2. Processa as saídas e normaliza eventos e entidades
		qtdRegistros, errSaida := executor.ProcessarSaida(c, ev.ID, m, dirSaidaModulo)
		if errSaida != nil {
			item.Sucesso = false
			item.Erro = fmt.Sprintf("erro processando saída: %v", errSaida)
			c.DB.Exec(`
				UPDATE execucoes SET estado='erro_saida', fim=?, erro=? WHERE id=?
			`, fim.UTC().Format(time.RFC3339), item.Erro, execID)
		} else {
			item.Sucesso = true
			item.Registros = qtdRegistros
			resPlano.TotalRegistros += qtdRegistros
			c.DB.Exec(`
				UPDATE execucoes SET estado='concluido', fim=?, registros=? WHERE id=?
			`, fim.UTC().Format(time.RFC3339), qtdRegistros, execID)

			c.Custodia.Registrar("modulo_sucesso", fmt.Sprintf("Módulo: %s | Registros inseridos: %d | Duração: %.2fs", m.Nome, qtdRegistros, item.DuracaoSeg))
		}

		resPlano.ItensExecutados = append(resPlano.ItensExecutados, item)
	}

	return resPlano, nil
}
