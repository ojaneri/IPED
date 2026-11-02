package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/ojaneri/ipedlite/core-go/internal/caso"
	"github.com/ojaneri/ipedlite/core-go/internal/instalador"
	"github.com/ojaneri/ipedlite/core-go/internal/mcp"
	"github.com/ojaneri/ipedlite/core-go/internal/modulo"
	"github.com/ojaneri/ipedlite/core-go/internal/motor"
	"github.com/ojaneri/ipedlite/core-go/internal/worker"
)

const versao = "1.0.0"

func main() {
	mcpFlag := flag.Bool("mcp", false, "Executa como servidor Model Context Protocol (MCP) sobre stdio")
	flag.Parse()

	args := flag.Args()

	// Se a flag --mcp for passada ou nenhum comando for fornecido em modo não-interativo, executa como MCP
	if *mcpFlag || len(args) == 0 {
		executarMCP()
		return
	}

	comando := args[0]
	switch comando {
	case "mcp":
		executarMCP()

	case "novo":
		if len(args) < 3 {
			fmt.Println("Uso: iped-hub novo <pasta_caso> <nome_caso> [perito]")
			os.Exit(1)
		}
		dir := args[1]
		nome := args[2]
		perito := "Perito Forense"
		if len(args) >= 4 {
			perito = args[3]
		}
		c, err := caso.Novo(dir, nome, perito)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Erro criando caso: %v\n", err)
			os.Exit(1)
		}
		defer c.Fechar()
		fmt.Printf("Caso criado com sucesso!\nID: %s\nNome: %s\nPerito: %s\nDiretório: %s\n", c.Info.ID, c.Info.Nome, c.Info.Perito, c.Dir)

	case "evidencia":
		if len(args) < 3 {
			fmt.Println("Uso: iped-hub evidencia <pasta_caso> <arquivo_evidencia> [--copiar]")
			os.Exit(1)
		}
		dirCaso := args[1]
		caminhoEvidencia := args[2]
		inSitu := true
		if len(args) >= 4 && args[3] == "--copiar" {
			inSitu = false
		}

		c, err := caso.Abrir(dirCaso)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Erro abrindo caso: %v\n", err)
			os.Exit(1)
		}
		defer c.Fechar()

		fmt.Printf("Calculando hashes e registrando evidência '%s' (in-situ: %v)...\n", caminhoEvidencia, inSitu)
		ev, err := c.AdicionarEvidencia(caminhoEvidencia, inSitu)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Erro registrando evidência: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Evidência registrada com sucesso!\nID: %d\nNome: %s\nTamanho: %d bytes\nSHA-256: %s\nMD5: %s\n", ev.ID, ev.Nome, ev.Tamanho, ev.SHA256, ev.MD5)

	case "verificar":
		if len(args) < 2 {
			fmt.Println("Uso: iped-hub verificar <pasta_caso>")
			os.Exit(1)
		}
		c, err := caso.Abrir(args[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Erro abrindo caso: %v\n", err)
			os.Exit(1)
		}
		defer c.Fechar()

		total, ok, err := c.Custodia.Verificar()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Falha na verificação: %v\n", err)
			os.Exit(1)
		}
		if ok {
			fmt.Printf("Custódia íntegra! Todos os %d registros encadeados foram verificados e conferem.\nHash Atual da Cadeia: %s\n", total, c.Custodia.UltimoHash())
		} else {
			fmt.Fprintf(os.Stderr, "ATENÇÃO: A cadeia de custódia apresentou inconsistências!\n")
			os.Exit(2)
		}

	case "status":
		if len(args) < 2 {
			fmt.Println("Uso: iped-hub status <pasta_caso>")
			os.Exit(1)
		}
		c, err := caso.Abrir(args[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Erro abrindo caso: %v\n", err)
			os.Exit(1)
		}
		defer c.Fechar()

		evs, _ := c.ListarEvidencias()
		_, totalEventos, _ := c.BuscarEventos("*", 1, 0)
		bks, _ := c.ListarMarcacoes()
		totalCustodia, ok, _ := c.Custodia.Verificar()

		fmt.Printf("=== STATUS DO CASO ===\n")
		fmt.Printf("ID: %s\nNome: %s\nPerito: %s\nCriado em: %s\n", c.Info.ID, c.Info.Nome, c.Info.Perito, c.Info.CriadoEm)
		fmt.Printf("Evidências registradas: %d\n", len(evs))
		for _, e := range evs {
			fmt.Printf(" - [%d] %s (%d bytes) [SHA256: %s...]\n", e.ID, e.Nome, e.Tamanho, e.SHA256[:12])
		}
		fmt.Printf("Eventos na Timeline: %d\n", totalEventos)
		fmt.Printf("Marcações (Bookmarks): %d\n", len(bks))
		fmt.Printf("Cadeia de Custódia: %d passos (Íntegra: %v)\n", totalCustodia, ok)

	case "importar-iped":
		if len(args) < 3 {
			fmt.Println("Uso: iped-hub importar-iped <pasta_caso> <pasta_saida_iped>")
			os.Exit(1)
		}
		dirCaso := args[1]
		dirIPED := args[2]

		c, err := caso.Abrir(dirCaso)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Erro abrindo caso: %v\n", err)
			os.Exit(1)
		}
		defer c.Fechar()

		fmt.Printf("Importando caso IPED de '%s' para o SQLite do caso...\n", dirIPED)
		res, err := c.ImportarCasoIPED(dirIPED, 1)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Falha na importação: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Importação concluída com sucesso!\nItens indexados: %d\nCategorias: %d\nMarcações: %d\nArquivos apagados: %d\nRecuperados: %d\n",
			res.TotalItens, res.Categorias, res.Marcacoes, res.Apagados, res.Recuperados)

	case "modulos":
		dirModulos := "modulos"
		if _, err := os.Stat(dirModulos); os.IsNotExist(err) {
			if _, err := os.Stat("core-go/modulos"); err == nil {
				dirModulos = "core-go/modulos"
			}
		}
		lista, err := modulo.CarregarModulos(dirModulos)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Erro carregando módulos: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Módulos registrados (%d):\n", len(lista))
		for _, m := range lista {
			fmt.Printf(" - [%s] %s v%s (Consome: %v, Modo: %s, Opcional: %v)\n", m.Nome, m.Titulo, m.Versao, m.Consome, m.Modo, m.Opcional)
		}

	case "preparar":
		dirFerramentas := "ferramentas"
		if len(args) >= 2 {
			dirFerramentas = args[1]
		}
		dirModulos := "modulos"
		if _, err := os.Stat(dirModulos); os.IsNotExist(err) {
			if _, err := os.Stat("core-go/modulos"); err == nil {
				dirModulos = "core-go/modulos"
			}
		}
		modulos, err := modulo.CarregarModulos(dirModulos)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Erro carregando módulos: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Verificando e preparando dependências em '%s'...\n", dirFerramentas)
		inst := instalador.NovoInstalador(dirFerramentas)
		relatorio := inst.PrepararTodos(modulos)
		fmt.Println("=== RELATÓRIO DE PREPARAÇÃO DE FERRAMENTAS ===")
		for _, st := range relatorio {
			statusStr := fmt.Sprintf("PRONTO (%s)", st.Caminho)
			if !st.Instalado {
				statusStr = fmt.Sprintf("FALHA: %s", st.Erro)
			}
			fmt.Printf(" - %s: %s\n", st.Modulo, statusStr)
		}

	case "triagem":
		if len(args) < 3 {
			fmt.Println("Uso: iped-hub triagem <pasta_caso> <evidencia_id> [--profundo]")
			os.Exit(1)
		}
		dirCaso := args[1]
		evID, err := strconv.ParseInt(args[2], 10, 64)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ID de evidência inválido: %v\n", args[2])
			os.Exit(1)
		}
		incluirProfundo := false
		if len(args) >= 4 && (args[3] == "--profundo" || args[3] == "-p") {
			incluirProfundo = true
		}

		c, err := caso.Abrir(dirCaso)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Erro abrindo caso: %v\n", err)
			os.Exit(1)
		}
		defer c.Fechar()

		ev, err := c.ObterEvidencia(evID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Erro buscando evidência %d: %v\n", evID, err)
			os.Exit(1)
		}

		dirModulos := "modulos"
		if _, err := os.Stat(dirModulos); os.IsNotExist(err) {
			if _, err := os.Stat("core-go/modulos"); err == nil {
				dirModulos = "core-go/modulos"
			}
		}
		modulos, err := modulo.CarregarModulos(dirModulos)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Erro carregando módulos: %v\n", err)
			os.Exit(1)
		}

		if ev.Tipo == "" || ev.Tipo == "arquivo" {
			detected, errId := modulo.IdentificarTipo(ev.Origem, modulos)
			if errId == nil && detected != "" {
				ev.Tipo = detected
			}
		}

		plano := motor.MontarPlano(ev.Tipo, modulos, incluirProfundo)
		if len(plano) == 0 {
			fmt.Printf("Nenhum módulo compatível com o tipo da evidência (%s).\n", ev.Tipo)
			return
		}

		dirFerramentas := "ferramentas"
		if _, err := os.Stat(dirFerramentas); os.IsNotExist(err) {
			if _, err := os.Stat("core-go/ferramentas"); err == nil {
				dirFerramentas = "core-go/ferramentas"
			}
		}

		fmt.Printf("Iniciando triagem para evidência [%d] %s (Tipo: %s | %d módulos planejados)...\n", ev.ID, ev.Nome, ev.Tipo, len(plano))
		resPlano, err := motor.ExecutarPlano(context.Background(), c, ev, plano, dirFerramentas)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Erro durante a execução do plano: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("=== RESUMO DA TRIAGEM ===")
		fmt.Printf("Módulos executados: %d\n", resPlano.TotalModulos)
		fmt.Printf("Total de registros consolidados: %d\n", resPlano.TotalRegistros)
		for _, item := range resPlano.ItensExecutados {
			statusStr := "SUCESSO"
			if !item.Sucesso {
				statusStr = fmt.Sprintf("FALHA (%s)", item.Erro)
			}
			fmt.Printf(" - [%s] %s: %s | %d registros (%.2fs)\n", item.Modulo, item.Titulo, statusStr, item.Registros, item.DuracaoSeg)
		}

	case "alertas":
		if len(args) < 2 {
			fmt.Println("Uso: iped-hub alertas <pasta_caso> [severidade]")
			os.Exit(1)
		}
		c, err := caso.Abrir(args[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Erro abrindo caso: %v\n", err)
			os.Exit(1)
		}
		defer c.Fechar()

		severidade := ""
		if len(args) >= 3 {
			severidade = args[2]
		}
		alertas, err := c.BuscarAlertas(severidade, 100)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Erro buscando alertas: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("=== ALERTAS E ACHADOS CRÍTICOS (%d) ===\n", len(alertas))
		for _, a := range alertas {
			fmt.Printf("[%s] %s | %s | %s (%s)\n", a.Severidade, a.Data, a.Fonte, a.Descricao, a.OrigemLocal)
		}

	case "entidades":
		if len(args) < 2 {
			fmt.Println("Uso: iped-hub entidades <pasta_caso> [tipo] [valor]")
			os.Exit(1)
		}
		c, err := caso.Abrir(args[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Erro abrindo caso: %v\n", err)
			os.Exit(1)
		}
		defer c.Fechar()

		tipo := ""
		if len(args) >= 3 {
			tipo = args[2]
		}
		valor := ""
		if len(args) >= 4 {
			valor = args[3]
		}
		entidades, err := c.BuscarEntidades(tipo, valor, 100)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Erro buscando entidades: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("=== ENTIDADES FORENSES EXTRAÍDAS (%d) ===\n", len(entidades))
		for _, e := range entidades {
			fmt.Printf("[%s] %s -> Evento #%d (%s: %s)\n", e.Tipo, e.Valor, e.EventoID, e.Fonte, e.Descricao)
		}

	case "versao":
		fmt.Printf("IPED Lite Hub v%s (arquitetura híbrida Go + Java)\n", versao)

	default:
		fmt.Printf("Comando desconhecido: %s\nComandos disponíveis: mcp, novo, evidencia, verificar, status, importar-iped, modulos, preparar, triagem, alertas, entidades, versao\n", comando)
		os.Exit(1)
	}
}

func executarMCP() {
	cfg := worker.ConfigAutodetectar()
	w := worker.NovoWorker(cfg)
	servidor := mcp.NovoServer(os.Stdin, os.Stdout, w)
	if err := servidor.Executar(); err != nil {
		fmt.Fprintf(os.Stderr, "Erro no servidor MCP: %v\n", err)
		os.Exit(1)
	}
}
