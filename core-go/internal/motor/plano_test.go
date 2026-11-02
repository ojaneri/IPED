package motor

import (
	"testing"

	"github.com/ojaneri/ipedlite/core-go/internal/modulo"
)

func TestMontarPlano(t *testing.T) {
	modulos := []*modulo.Manifesto{
		{
			Nome:     "deep-iped",
			Consome:  []string{"disco"},
			Opcional: true,
		},
		{
			Nome:     "fast-mft",
			Consome:  []string{"disco"},
			Opcional: false,
		},
		{
			Nome:     "fast-logs",
			Consome:  []string{"log_eventos"},
			Opcional: false,
		},
	}

	// 1. Testa evidência do tipo disco apenas com triagem rápida
	planoDiscoRapido := MontarPlano("disco", modulos, false)
	if len(planoDiscoRapido) != 1 || planoDiscoRapido[0].Nome != "fast-mft" {
		t.Fatalf("esperava apenas fast-mft, obteve: %+v", planoDiscoRapido)
	}

	// 2. Testa evidência do tipo disco com exame profundo incluído
	planoDiscoCompleto := MontarPlano("disco", modulos, true)
	if len(planoDiscoCompleto) != 2 {
		t.Fatalf("esperava 2 módulos, obteve %d", len(planoDiscoCompleto))
	}
	// Garante que o rápido vem antes do profundo
	if planoDiscoCompleto[0].Nome != "fast-mft" || planoDiscoCompleto[1].Nome != "deep-iped" {
		t.Fatalf("ordem incorreta de execução: %+v", planoDiscoCompleto)
	}

	// 3. Testa evidência do tipo log_eventos
	planoLogs := MontarPlano("log_eventos", modulos, true)
	if len(planoLogs) != 1 || planoLogs[0].Nome != "fast-logs" {
		t.Fatalf("esperava fast-logs, obteve: %+v", planoLogs)
	}
}
