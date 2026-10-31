# IPED Lite: versão de baixo consumo de RAM

## Contexto

Queremos um IPED que rode bem em máquinas de **8 GB de RAM**, tanto no processamento quanto na análise. As decisões tomadas:

- **Linguagem:** Java primeiro, no próprio código do IPED. Um protótipo de triagem em Rust fica para a fase 2.
- **Onde:** um **fork próprio**, o que permite cortes maiores.

O mapeamento do código mostrou que a maior parte da RAM vem de decisões de projeto, não da JVM:
- heap padrão grande (25% da RAM no processamento e 50% na análise, em `Bootstrap.java:70` e `BootstrapUI.java:28`);
- buffers fixos por thread;
- estruturas que crescem sem limite;
- um objeto Java por item nos resultados da interface;
- processos externos (Neo4j, LibreOffice, servidores TSK, Python).

Por isso, corrigir em Java dá o maior ganho com o menor risco. Reescrever em Rust só faz sentido como ferramenta nova e enxuta (fase 2).

**Requisito adicional: controle por agentes de IA.** O IPED lite precisa poder ser rodado e controlado por agentes como o Claude Code, via **MCP**:
- processar evidências;
- acompanhar o progresso;
- buscar, ler itens e texto;
- marcar bookmarks e exportar.

Isso combina com o objetivo de leveza. Um modo de análise headless via MCP não carrega a interface Swing, e portanto nenhum dos consumidores de RAM da UI: galeria, visualizadores, ordenador da tabela e Neo4j.

**Orçamento-alvo numa máquina de 8 GB:**
- **Processamento:** heap de até 3 GB e árvore de processos de até 5 GB.
- **Análise:** heap de até 3 GB abrindo casos de 1 a 2 milhões de itens. Casos de cerca de 10 milhões devem degradar sem dar OutOfMemoryError.

## Etapa 0: medição de referência (antes de qualquer mudança)

- Criar a branch `lite` a partir de `master`, não da `feature/pdf-signatures-v2`. Nada vai para o remoto sem autorização.
- Montar um corpus fixo: uma imagem E01/dd com NTFS, um UFDR, um PST grande e um ZIP aninhado.
- Escrever um script de medição em `lite/bench/` (PowerShell e bash). Ele mede:
  - pico de RSS da árvore de processos;
  - heap após GC (`jcmd <pid> GC.heap_info`), memória nativa (`-XX:NativeMemoryTracking=summary` + `jcmd VM.native_memory`) e um JFR de cada execução;
  - tempo total e número de itens.
- Medir a análise com roteiro fixo: abrir o caso, buscar `*`, ordenar por nome, filtrar uma categoria, ativar a timeline em tabela, abrir o painel de metadados no campo hash e abrir a galeria.

## Etapa 1: perfil `lite` só com configuração (sem código)

Criar `iped-app/resources/config/profiles/lite/`, partindo do perfil `triage` (`profiles/triage/`).

- **`IPEDConfig.txt`**
  - Desligados: `enableGraphGeneration`, `enableLanguageDetect`, `enableVideoThumbs`, `enableHTMLReport`, OCR, IA, transcrição e carving.
  - Mantidos: hash, parsing e expansão de contêineres.
- **`ProcessingPriorityConfig.txt`:** `maxQueueSize = 20000`.
- **`IndexTaskConfig.txt`:** `storeTermVectors = false`, `textSplitSize = 1000000`, `maxTokenLength = 128`.
- **`ParsingTaskConfig.txt`:** `storeTextCacheOnDisk = true`. O triage usa `false`, o que piora o consumo.
- **`FileSystemConfig.txt`:** `numImageReaders = 1`.
- **`ImageThumbsConfig.txt`:** `imgThumbSize` menor (160).
- **Threads:** documentar `numThreads = 4` para máquinas de 8 GB (`LocalConfig.txt`), porque o `LocalConfig` não vem do perfil.

**Medir.** Este perfil sozinho já deve cortar bastante do processamento.

## Etapa 2: JVM e inicialização

- **`Bootstrap.java`:** o fator heap/RAM (`getRAMToHeapFactor`, linha 70) e o teto passam a ser configuráveis.
  - Usar `LocalConfig`, ou uma propriedade como `-Diped.heapFactor`.
  - No perfil lite, o teto é de 3 GB. Fazer o mesmo em `BootstrapUI.java:28`.
- **Flags para devolver memória ao sistema no Java 11 / G1:** `-XX:MinHeapFreeRatio=10 -XX:MaxHeapFreeRatio=30`.
- **Outros limites:** `-XX:ReservedCodeCacheSize=128m`, `-XX:MaxMetaspaceSize=384m`, `-XX:+UseStringDeduplication`.
- **Migrar o target para Java 21.** Passou a ser necessário, porque o SDK Java oficial do MCP (Etapa 5) exige Java 17 ou superior; confirmar a versão mínima no momento da implementação. A versão LTS mais nova fica para avaliar depois. A migração traz:
  - ZGC geracional, que devolve memória ao sistema;
  - na versão 25, `-XX:+UseCompactObjectHeaders`.

  Validar Nashorn, JavaFX (a Liberica full traz JavaFX) e as flags `--add-opens` já usadas no Bootstrap.

## Etapa 3: correções no processamento (em ordem de impacto)

1. **`TextCache.MAX_MEMORY_CHARS`** (`iped-engine/.../TextCache.java:33`), hoje fixo em 10M caracteres por item e por thread:
   - torná-lo configurável em `ParsingTaskConfig`;
   - usar 1M no perfil lite.
2. **Fila de processamento** (`ProcessingQueues.java:55-121`):
   - aplicar `maxQueueSize` também às filas de prioridade acima de 0 e a `addItemFirstNonBlocking`;
   - fazer só um backpressure leve, sem bloquear filhos inline, para não travar.
3. **`DuplicateTask`** (`DuplicateTask.java:35`) usa um `HashMap<IHashValue,IHashValue>` com uma entrada por hash único:
   - trocá-lo por um conjunto primitivo de `long`s (os primeiros 8 bytes do hash), com verificação de colisão no índice, mais um array listrado de locks fixo para o `ExportFileTask` (`ExportFileTask.java:544,877`);
   - assim o mapa deixa de ser usado como trava.
4. **`SleuthkitReader`:**
   - trocar `ArrayList<Integer>` (`sleuthIdToId`, `tskParentIds`, `SleuthkitReader.java:130`) por um array de `int` que cresce. Reaproveitar o `PackedLongValues`/`ArrayUtil` do Lucene, que já é dependência;
   - limpar `idTotrackIDMap` (linha 148) quando cada imagem terminar.
5. **`UfedXmlReader`:**
   - `pathToParent` deixa de guardar `IItem` inteiro e passa a guardar só os campos usados (id, trackId e caminho);
   - `ZIPInputStreamFactory` passa a ter um cache configurável (no lite, 32 MB).
6. **`ParsingTask.idToItemMap`** (linha 177): remover as entradas cujos filhos já foram processados.
7. **Constantes do `ExportFileTask` e do `PreviewRepositoryManager`:** tornar configuráveis
   - `SQLITE_CACHE_SIZE` (16 MB × 16 bancos);
   - `MAX_BUFFER_SIZE` (16 MB);
   - o cache do H2 (64 MB).

   No lite: 2 MB, 4 MB e 16 MB.
8. **`Statistics.java:408`:** usar como mínimo por thread um valor coerente com o perfil lite.

## Etapa 4: correções na interface de análise (em ordem de impacto)

1. **Neo4j só sob demanda.** Hoje `UICaseDataLoader.java:115` inicia o `LoadGraphDatabaseWorker` ao abrir o caso.
   - Iniciar apenas quando a aba de grafo for aberta.
   - Limitar o page cache em `GraphServiceImpl.start` (linha 43) com `GraphDatabaseSettings.pagecache_memory`.
2. **Timeline em modo tabela** (`TimelineResults.expandTimestamps`):
   - eliminar o `new int[Short.MAX_VALUE][1024]` (cerca de 128 MB) alocado a cada chamada;
   - trocar `TimeItemId` e os `Float` por arrays primitivos.
3. **Filtros:** em `FilterManager.java:378-435`, `DynamicDuplicateFilter`, `MetadataSearch.getIdsWithOrd` e `TimelineResults`:
   - trocar `LinkedHashSet<IItemId>` + `ArrayList<Float>` por RoaringBitmap ou arrays primitivos;
   - o RoaringBitmap já é usado em bookmarks.
4. **`MultiSearchResult`:** trocar `IItemId[]` por `int[] sourceIds` + `int[] ids`, ou por um `long[]`, criando o `IItemId` só quando pedido.
   - É o refactor maior, porque mexe na API de `iped-api`. Como é um fork, isso é aceitável.
   - Ajustar `ParallelRowSorter` (o `Row[]`) para índices primitivos.
5. **Ordenação:**
   - `MAX_COMPARATOR_CACHE` (`ResultTableRowSorter.java:40`) passa a ser configurável (no lite, 1);
   - no lite, usar o caminho `RowComparator.setLoadDocValues(false)`, que lê os doc values direto em vez de copiar arrays.
6. **`SlowCompositeReaderWrapper.cachedOrdMaps`** (linhas 89 e 195): passar para LRU ou SoftReference.
7. **Galeria** (`GalleryModel.java:84`):
   - cache configurável (no lite, 200);
   - sempre redimensionar a miniatura (linha 291).
8. **Visualizadores** (`ViewerController.java:90-139`): inicialização sob demanda, com o LibreOffice e as WebViews JavaFX criados no primeiro uso.
9. **Bug encontrado:** `IPEDMultiSource.baseDocCache` é `static` e só cresce (linha 36). Torná-lo um campo de instância.

## Etapa 5: controle por agentes de IA (servidor MCP)

**Base existente:** já há uma API REST em `iped-engine/src/main/java/iped/engine/webapi/` (Jersey/Grizzly, `Main.java`), com:
- `search`, `sources` (abrir casos), `docs/{id}`, `text`, `content`, `thumb`, `categories`, `bookmarks` e `selection`.

O que falta nela:
- **Paginação.** `Search.java:41-55` devolve todos os resultados de uma vez, o que estoura o contexto do agente e a RAM.
- **Controle do processamento.**
- **Estatísticas de campos.**
- **Exportação.**

**Abordagem:** um novo módulo `iped-mcp`, servidor MCP em Java via stdio que roda **no mesmo processo** do motor.
- Usar o SDK Java oficial do MCP (`io.modelcontextprotocol.sdk:mcp`).
- Ele reaproveita `IPEDSource`/`IPEDMultiSource`, `IPEDSearcher` e as classes do `webapi` (`Sources`, `Docs`, `Text`, `Bookmarks`) como camada de serviço.
- Assim evitamos mais um runtime (Python ou Node) e um salto via HTTP, ou seja, uma só JVM e menos RAM.
- O Claude Code inicia o servidor como MCP stdio. O mesmo jar também aceita `--http` para uso remoto, sempre só em localhost.
- **Fallback, caso a migração para Java 21 atrase:** implementar o JSON-RPC mínimo do MCP à mão (`initialize`, `tools/list`, `tools/call`) em Java 11.

**Ferramentas MCP (respostas compactas e sempre paginadas):**

| Ferramenta | Função | Escrita? |
|---|---|---|
| `iped_process` | inicia `iped.jar --nogui` com perfil (padrão `lite`) em subprocesso e devolve `job_id` | sim |
| `iped_process_status` / `iped_process_cancel` | progresso (itens, fase, ETA, RAM) e cancelamento | — / sim |
| `iped_open_case` / `iped_list_cases` | abre caso(s) processado(s) | não |
| `iped_search` | consulta Lucene com `limit` (padrão 50), `offset`, `sort` e `fields`; devolve linhas compactas (id, nome, caminho, categoria, tamanho, datas, hash) | não |
| `iped_get_item` | metadados de um item | não |
| `iped_get_text` | texto extraído paginado (`offset`, `max_chars`) | não |
| `iped_categories` | árvore de categorias com contagens | não |
| `iped_field_stats` | top-N valores de um campo para uma consulta (equivale ao painel de metadados, mas limitado) | não |
| `iped_timeline` | eventos em um intervalo, paginados | não |
| `iped_bookmarks_*` | listar, criar, adicionar e remover | sim |
| `iped_export_item` | grava o arquivo num diretório de saída permitido e devolve caminho + hash | sim |
| `iped_thumbnail` | imagem da miniatura (opcional) | não |

**Saída de progresso legível por máquina:**
- adicionar `--json-progress` ao processamento (no `ProgressConsole`, em `iped-app/.../processing/`);
- o `iped_process_status` lê essa saída em vez de interpretar o log.

**Requisitos forenses:**
- **Somente leitura por padrão.** As ferramentas de escrita só ficam ativas com `--allow-write`.
- **A evidência nunca é aberta para escrita.** A exportação fica restrita a um diretório configurado.
- **Log de auditoria** de cada chamada MCP no diretório do caso (`mcp-audit.log`, com data/hora, ferramenta, parâmetros e hash do resultado). Serve para a cadeia de custódia e para o laudo.

**Integração com o Claude Code:**
- Exemplo de `.mcp.json`.
- Uma skill `iped-agent` com o fluxo típico: processar → aguardar → categorias → buscas → ler texto → marcar → exportar. Ela inclui as boas práticas de consulta Lucene no IPED.

**Memória:** o modo headless MCP vira o modo de análise mais leve. Mesmo assim, as correções da Etapa 4 no `MultiSearchResult` e nos filtros também valem aqui, porque o motor é compartilhado.

## Etapa 6: distribuição enxuta (efeito maior em disco, menor em RAM)

- **Perfil Maven `lite` em `iped-app/pom.xml`:**
  - excluir mplayer, LibreOffice, tesseract, modelos Vosk, `rfdie.dat`, NSFW e Python;
  - gerar um JRE via jlink sem JavaFX para o processamento;
  - estimativa: de cerca de 1,2 GB para menos de 300 MB.
- **Cortar a dependência `iped-engine → iped-viewers-impl`** (`iped-engine/pom.xml:99`): mover `DefaultPolicy`, `LibreOfficeFinder`, `ImageMetadataUtil` e `HtmlLinkViewer` para `iped-utils`.
- **Opcional:** mover o `Main` de processamento e o `ProgressConsole` para um módulo sem `iped.app.ui`, gerando um jar headless de verdade.

- **Incluir o `iped-mcp` na distribuição lite.**

## Etapa 7 (fase 2): protótipo de triagem em Rust (repositório separado, plano próprio)

Só começa depois de medir as etapas 1 a 6.

- **Controle por agentes:** a ferramenta expõe as mesmas ferramentas MCP da Etapa 5, para que o agente use os dois sem diferença.

- **Escopo:** Sleuthkit via bindings, hash, assinatura de arquivos, índice Tantivy e poucos formatos.
- **Saída:** algo que o IPED completo consiga importar, como arquivos exportados + CSV ou DFXML.
- **Critério para seguir:** a ferramenta precisa fazer em menos de 1 GB o que o IPED lite faz em 3 GB, no mesmo corpus.

## Verificação

- **Medição:** depois de cada etapa, rodar o `lite/bench` no corpus fixo e comparar com a referência da Etapa 0 (pico de RSS, heap após GC, NMT e tempo). Registrar os números em `lite/RESULTS.md`.
- **Equivalência funcional:** o mesmo corpus no perfil padrão e no lite deve dar:
  - o mesmo número de itens;
  - os mesmos hashes, conferidos pelo CSV exportado de cada caso com `diff`;
  - a mesma quantidade de resultados para um conjunto de buscas fixas.
- **Testes:** `mvn -pl iped-engine,iped-parsers -am test`. Os testes de `iped-parsers` cobrem os parsers.
- **Análise:** rodar o roteiro da Etapa 0 numa máquina ou VM com 8 GB, com `-Xmx3g`. Não pode haver OutOfMemoryError, e a interface deve responder.
- **MCP:**
  - **Testes automáticos:** JUnit do `iped-mcp` contra um caso pequeno processado do corpus. Cobrir paginação, limites de tamanho das respostas e rejeição de escrita sem `--allow-write`.
  - **Inspeção manual:** `npx @modelcontextprotocol/inspector` apontando para o jar.
  - **Ponta a ponta:** registrar no Claude Code via `.mcp.json` e pedir ao agente: "processe esta imagem com perfil lite, liste as categorias, encontre os PDFs com 'contrato', leia o texto do primeiro e marque como 'relevante'".
    - Conferir o `mcp-audit.log`.
    - Conferir o bookmark na interface gráfica.
    - Medir o pico de RSS do servidor MCP no corpus.

## Ordem sugerida

| Etapa | Esforço |
|---|---|
| 0 e 1 | dias |
| 2 | 1–2 semanas, com o Java 21 |
| 3 | 2–3 semanas |
| 4 | 3–5 semanas (o `MultiSearchResult` é o item mais caro) |
| 5 (MCP) | 2–3 semanas; pode correr em paralelo com 3 e 4 depois da Etapa 2 |
| 6 | 1–2 semanas |
| 7 | depois, com plano separado |

Cada etapa é um commit, ou uma série deles, na branch `lite`.
