package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ojaneri/ipedlite/core-go/internal/caso"
	"github.com/ojaneri/ipedlite/core-go/internal/mcp"
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

	case "versao":
		fmt.Printf("IPED Lite Hub v%s (arquitetura híbrida Go + Java)\n", versao)

	default:
		fmt.Printf("Comando desconhecido: %s\nComandos disponíveis: mcp, novo, evidencia, verificar, status, versao\n", comando)
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
