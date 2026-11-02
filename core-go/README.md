# IPED Lite Hub (core-go)

Núcleo em Go da arquitetura híbrida do **IPED Lite**, inspirado na filosofia do [Panthera Forensics](https://github.com/marianabsctba/Panthera_Forensics).

Ele atua como **maestro ultraleve, custódia e servidor MCP nativo**, gerenciando o ciclo de vida dos casos, persistência em SQLite WAL (`modernc.org/sqlite`, sem dependência de Cgo) e registro criptográfico de custódia encadeada. Para exames profundos de conteúdo (Tika, carvers e Lucene), ele delega sob demanda para o worker Java do **IPED Lite**.

---

## 🚀 Características Principais

* **Pegada Mínima de Memória:** ~15 MB a 40 MB de RSS em repouso.
* **Leitura In-Situ de Evidências:** Registra e calcula hashes (SHA-256, SHA-1, MD5) diretamente na origem via streaming (buffer constante de 64 KB), sem duplicar imagens de gigabytes ou terabytes em disco.
* **Cadeia de Custódia Criptográfica:** Cada ação gera um hash SHA-256 encadeado no elo anterior (`custodia.jsonl`), fornecendo prova matemática de não-adulteração.
* **Servidor MCP Nativo (stdio JSON-RPC):** Expõe 11 ferramentas periciais diretamente para agentes de IA (Claude Code, Antigravity, Cursor, etc.).
* **Worker Java Sob Demanda:** Invocação desacoplada do `iped.jar` (perfil `lite`), mantendo o núcleo seguro mesmo se um arquivo corrompido causar falha de memória no Java.

---

## 🛠️ Compilação e Execução

### Pré-requisitos
* Go 1.22+ instalado (ex: `C:\tools\go\bin\go.exe`).

### Compilar o Executável
```bash
cd core-go
go build -o iped-hub.exe ./cmd/iped-hub
```

### Comandos da CLI
```bash
# 1. Iniciar como servidor MCP para agentes de IA (padrão)
./iped-hub.exe --mcp

# 2. Criar um novo caso pericial
./iped-hub.exe novo "D:\casos\caso-01" "Operação Alpha" "Perito Responsável"

# 3. Adicionar uma evidência (modo in-situ, sem copiar arquivo)
./iped-hub.exe evidencia "D:\casos\caso-01" "D:\imagens\disco.E01"

# 4. Verificar a integridade matemática da cadeia de custódia
./iped-hub.exe verificar "D:\casos\caso-01"

# 5. Visualizar status geral do caso e evidências registradas
./iped-hub.exe status "D:\casos\caso-01"
```

---

## 🤖 Integração com Agentes de IA via MCP

Para conectar o Claude Code ou qualquer cliente MCP ao `iped-hub`:

### Claude Code CLI
```bash
claude mcp add iped-hub -- path/to/core-go/iped-hub.exe --mcp
```

### Configuração JSON (`claude_desktop_config.json` ou `.mcp.json`)
```json
{
  "mcpServers": {
    "iped-hub": {
      "command": "D:\\IPED\\core-go\\iped-hub.exe",
      "args": ["--mcp"]
    }
  }
}
```

---

## 🧪 Testes Unitários

```bash
cd core-go
go test -v ./...
```
