# IPED Lite

**IPED Lite** is a lightweight, resource-optimized distribution of the [IPED Digital Forensic Tool](https://github.com/sepinf-inc/IPED), featuring an ultra-low memory footprint and a native **Model Context Protocol (MCP)** server for autonomous AI agent integration.

It is designed to run efficiently on commodity workstations and laptops with limited memory (such as 8 GB RAM machines, operating under a strict ≤ 3 GB JVM heap limit), while enabling AI agents to programmatically query, inspect, and tag forensic evidence via standard stdio JSON-RPC.

* **Repository:** `https://github.com/ojaneri/ipedlite`

---

## 📋 Copy & Paste Prompt for AI Agents (Claude Code, Antigravity, Cursor, etc.)

Copy and paste the prompt below into your AI assistant session to have the agent immediately inspect, triage, or audit digital forensic cases using IPED Lite and its Go Hub orchestrator via MCP:

```text
We are using IPED Lite (https://github.com/ojaneri/ipedlite.git), a lightweight digital forensics platform featuring an ultra-fast Go orchestrator (core-go/cmd/iped-hub) and an optimized Java processing engine (≤ 3 GB heap) with native Model Context Protocol (MCP) support over stdio.

Please follow these steps:
1. Check if the Go Hub binary (iped-hub) is already compiled:
   - If not compiled: cd core-go && go build -o iped-hub.exe ./cmd/iped-hub (or "go build -o iped-hub ./cmd/iped-hub" on Linux/macOS)
   - The hub provides 19 native forensic MCP tools over stdio.

2. Configure and connect to the IPED Lite MCP Server:
   - For Claude Code CLI:
     claude mcp add ipedlite -- /path/to/ipedlite/core-go/iped-hub.exe mcp
   - For Claude Desktop / Antigravity / Cursor (.mcp.json or claude_desktop_config.json):
     "mcpServers": {
       "ipedlite": {
         "command": "/path/to/ipedlite/core-go/iped-hub.exe",
         "args": ["mcp"]
       }
     }

3. Forensic Workflows available via MCP:
   - Case Management: iped_create_case, iped_open_case, iped_case_status
   - In-Situ Ingestion (no disk copy): iped_evidence_add(path, in_situ=true), iped_evidence_list
   - Evidence Identification: iped_identify_evidence(path)
   - Fast Triage Pipeline: iped_triage_plan(path), iped_triage_run(evidence_id)
   - Threat Detection & Search: iped_alerts_list(severity="critica"), iped_entities_query(type="ip"), iped_timeline_search(query="*")
   - Defense Audit & Legal Nullities: iped_import_iped_case(iped_case_dir) to cross-reference official findings, detect unhashed items, or uncover omitted exculpatory evidence.
   - Visual Inspection: iped_web_serve(port=8080) to launch the embedded offline forensic web dashboard for human examiners.
   - Deep Examination: iped_deep_process(evidence, output, profile="lite"), iped_deep_status()
   - Chain of Custody: iped_custody_verify() to validate SHA-256 state hashes.
```

---

## Key Differences from Standard IPED

### 1. Memory Optimization & Stability (≤ 3 GB Heap Profile)
Standard forensic indexers can quickly exhaust memory on deep container extractions or large email archives. IPED Lite introduces key architectural improvements:

* **Configurable In-Memory Text Cache:** Implemented `textCacheMaxMemoryChars` in `ParsingTaskConfig` (capped at 1,000,000 characters in the `lite` profile), preventing unbounded string allocations during heavy document extractions.
* **Compact HashIndex for Deduplication:** Replaced memory-heavy `HashMap<HashTask.Key, Set<Integer>>` in `DuplicateTask` with a primitive-backed `HashIndex`, reducing heap footprint by hundreds of megabytes during large deduplication passes.
* **4,096 Striped Locks in SQLite Exporter:** Concurrency race conditions in `ExportFileTask` (which caused SQLite "no such table" errors during multi-threaded table creation) were resolved using striped synchronization locks.
* **Dynamic Queue Backpressure:** Throttles producer tasks (priority 0) in `ProcessingQueues` when total queue size exceeds safe thresholds, preventing memory spikes caused by recursive archive extractions.
* **Primitive Arrays & Data Source Cleanup:** Replaced boxed integer collections in `SleuthkitReader` with primitive `CompactIntArray`, clearing image tracking maps upon completion; streamlined `UfedXmlReader` to retain lightweight parent references instead of full item objects.
* **Lazy Loading of Graph Analytics (Neo4j):** The embedded Neo4j graph engine is loaded on-demand only when the Graph Analysis tab is accessed, saving ~350–500 MB of heap and native memory at startup.
* **Timeline Allocation Optimization:** Converted fixed 128 MB table buffers in `TimelineResults` to dynamic row allocation, and made multi-source doc caches instance-bound to eliminate static retention.
* **Tuned Garbage Collection Defaults:** Bootstrap JVM flags are pre-configured with aggressive memory reclamation (`-XX:MinHeapFreeRatio=10 -XX:MaxHeapFreeRatio=30 -XX:+UseStringDeduplication -XX:ReservedCodeCacheSize=128m -XX:MaxMetaspaceSize=384m`).

### 2. Real Benchmark & Performance Data

Rigorous benchmarks conducted on identical hardware and forensic datasets (`D:\iped-bench`) demonstrate the radical gains achieved by the IPED Lite Java engine and the Go Hub Orchestrator:

| Benchmark Metric | IPED Original (Master) | IPED Lite (Java Engine) | Go Orchestrator (`iped-hub`) | Observed Advantage |
| :--- | :---: | :---: | :---: | :---: |
| **Startup Latency (Cold Start)** | ~5,000 – 15,000 ms *(JVM bootstrap)* | ~3,000 ms | **19.2 ms** *(Native Go binary)* | **> 250x faster bootstrap** |
| **Idle Memory (Working Set)** | ~250 – 450 MB | ~160 MB | **8.4 MB** | **~96% memory reduction** |
| **Processing Peak Memory** | **5,762 MB** *(OOM failures under 2 GB heap)* | **3,140 MB** *(Stable under 2 GB heap)* | **< 35 MB** *(Triage engine)* | **~45% to 99% less memory** |
| **Full Ingestion (26,710 mbox emails)** | 1,661 seconds *(~27.6 min)* | **742 seconds** *(~12.3 min)* | *(Delegates to IPED Lite)* | **> 2.24x faster execution** |
| **Case Ingestion (5,822 items)** | N/A *(Generated output)* | N/A *(Generated output)* | **0.084 s** *(SQLite WAL batch)* | **> 69,000 items/second** |
| **Timeline Query Latency** | ~80 – 300 ms *(Lucene on disk)* | ~50 – 150 ms *(Lucene)* | **2.7 ms** *(Indexed SQLite WAL)* | **> 20x to 50x faster response** |
| **Disk Space for Evidence** | Mandates full data copy | Mandates full data copy | **0 bytes extra** *(In-Situ reading)* | **Saves terabytes of disk** |

---

## ⚖️ Forensic Audit & Legal Defense Guide (Nulidades no Processo Penal)

IPED Lite is designed for complete two-way compatibility with official police/forensic reports. It serves as an audit weapon for **Technical Criminal Defense Assistants**, allowing independent reprocessing of evidence to uncover procedural nullities and technical flaws under the **Brazilian Code of Criminal Procedure (Arts. 158-A to 158-F do CPP - Pacote Anticrime, Lei 13.964/2019)**:

### 1. Empirical Flaw Discovered in Official IPED Output
During audit tests on real case outputs produced by the standard IPED engine (`out-C800-step3-exact`, containing 5,822 items), the Go Hub audit engine revealed a critical vulnerability:

```text
Total Items in Official Case: 5,822
Items with Cryptographic Hashes: 789 (14.3%)
Items WITHOUT Hashes in Report: 4,994 (85.7%)
```
> [!WARNING]
> In default configurations, standard IPED reports may skip individual hash generation for subitems and embedded attachments. Over **85% of files** in the case lacked cryptographic hashes (`MD5=""`, `SHA1=""`). Under **Art. 158-D do CPP**, this leaves the prosecution without mathematical proof that evidence files remained untampered with from the extraction moment to court presentation.

### 2. Key Defense Theses Supported by IPED Lite Hub

* **Chain of Custody Breach (Art. 158-A to 158-F do CPP):**
  The Go Hub recalculates simultaneous streaming hashes (**MD5, SHA-1, and SHA-256**) directly from the raw disk image in-situ. Any divergence between the recalculated hash and the police seizure report legally establishes a breach in the chain of custody (STJ, RHC 143.169/RJ).
* **Concealment of Exculpatory Evidence (Cerceamento de Defesa):**
  Official forensic reports frequently apply narrow keyword filters or exclude full categories. By reprocessing the raw media with `iped-hub`, the defense can query the entire SQLite database to cross-reference items omitted from the official report:
  ```sql
  SELECT * FROM eventos WHERE descricao NOT IN (SELECT item FROM laudo_oficial);
  ```
* **Temporal Tampering (*Timestomping* Detection via `mftecmd`):**
  NTFS tracks file times in two separate attributes: `$STANDARD_INFORMATION` (SI, user-modifiable) and `$FILE_NAME` (FN, kernel-managed). The `mftecmd` module compares both timestamps simultaneously. If SI was forged to match a suspect's work hours, FN preserves the true creation date, proving deliberate tampering.
* **Exclusion of Authorship via Malware/Remote Incursion (`hayabusa`):**
  By executing Sigma detection rules across Windows event logs (`.evtx`), the `hayabusa` module identifies whether external RDP sessions, malware payloads, or automated tools (Mimikatz, PsExec) were active at the alleged time of the crime, demonstrating that acts were committed by an external attacker rather than the computer owner.
* **Tamper-Evident Technical Logs vs Plain-Text Police Logs:**
  Standard IPED generates mutable plain-text `.log` files. In contrast, IPED Lite Hub seals every forensic action into an immutable SHA-256 hash chain (`custodia.jsonl`), highlighting the lack of technical non-repudiation in ordinary police logs.

---

## Native Model Context Protocol (MCP) Server

IPED Lite features a built-in JSON-RPC 2.0 MCP server over `stdio` (`iped.engine.mcp.McpServer`). It allows AI assistants and automated forensic agents to directly interact with IPED cases.

### MCP Stdio Isolation
To prevent protocol corruption, IPED Lite isolates standard I/O: all internal logging, Lucene notifications, and third-party outputs are redirected to `System.err`, ensuring `System.out` strictly carries JSON-RPC payloads.

### Available MCP Tools

| Tool | Description | Parameters |
| :--- | :--- | :--- |
| `iped_open_case` | Opens an indexed IPED case for reading and querying. | `path` (string) |
| `iped_list_cases` | Returns status and item counts for the active case. | None |
| `iped_categories` | Lists all forensic categories identified in the case. | None |
| `iped_search` | Searches items using Lucene syntax with pagination. | `query` (string), `limit` (int), `offset` (int) |
| `iped_get_item` | Retrieves full metadata properties and attributes for an item. | `id` (int) |
| `iped_get_text` | Fetches decoded text content in chunks with continuation flags. | `id` (int), `offset_chars` (int), `max_chars` (int) |
| `iped_bookmarks_list` | Lists all bookmarks/tags and their item counts. | None |
| `iped_bookmarks_add` | Adds or dynamically creates a bookmark/tag for an item. | `id` (int), `bookmark` (string) |
| `iped_process` | Initiates background forensic processing of evidence. | `evidence` (string), `output` (string), `profile` (string), `extraArgs` (string) |
| `iped_process_status` | Checks progress and status of active background processing. | None |

---

## Agent Configuration Snippets

### 1. Claude Desktop / Generic MCP Client
Add the server entry to your `claude_desktop_config.json` or `.mcp.json`:

```json
{
  "mcpServers": {
    "ipedlite": {
      "command": "java",
      "args": [
        "-cp",
        "path/to/ipedlite/target/release/iped-4.4.0-SNAPSHOT/iped.jar;path/to/ipedlite/target/release/iped-4.4.0-SNAPSHOT/lib/*",
        "iped.engine.mcp.McpServer"
      ]
    }
  }
}
```

### 2. Claude Code CLI
Register the server directly via command line:

```bash
claude mcp add ipedlite -- java -cp "path/to/release/iped.jar;path/to/release/lib/*" iped.engine.mcp.McpServer
```

---

## Building from Source

### Prerequisites
* **Java 11 JDK** with JavaFX (e.g., [BellSoft Liberica JDK 11 Full](https://bell-sw.com/pages/downloads/))
* **Apache Maven 3.8+**
* **Git**

### Build Steps

```bash
# Clone the repository
git clone https://github.com/ojaneri/ipedlite.git
cd ipedlite

# Build the release package
mvn clean install -DskipTests
```

The compiled release package will be located in:
`target/release/iped-4.4.0-SNAPSHOT/`

---

## Usage

### 1. Processing Evidence with the `lite` Profile
To process evidence using the memory-optimized profile:

```bash
java -jar iped.jar -profile lite -d /path/to/evidence -o /path/to/output_case
```

### 2. Analyzing Cases in the UI
Launch the graphical interface:

```bash
java -jar iped.jar
```
*(Neo4j graph analytics will remain unloaded until the Graph tab is actively clicked, preserving system RAM).*

### 3. Launching the MCP Server Standalone
```bash
java -cp "iped.jar;lib/*" iped.engine.mcp.McpServer
```

---

## Core Features Inherited from IPED

IPED Lite retains the full forensic processing capabilities of IPED:
* **Evidence Formats:** RAW/DD, E01, EX01, ISO9660, VHD, VMDK, AD1, UFDR (Cellebrite).
* **Hash Algorithms & Sets:** MD5, SHA-1, SHA-256, SHA-512, Edonkey, NIST NSRL, ProjectVIC, ICSE.
* **Decoding & Parsing:** Container expansion, browser history, mobile app databases (WhatsApp, Telegram, Skype), audio transcription, and OCR.
* **Carving Engine:** File carving across unallocated and raw space with support for 40+ file formats.
* **Query & Search:** Full-text indexing, regex matching (credit cards, cryptocurrency wallets, emails, phone numbers).

---

## ⚡ Hybrid Go + Java Architecture (`core-go / iped-hub`)

IPED Lite incorporates an ultra-fast, lightweight orchestrator written in pure Go (`core-go/cmd/iped-hub`), inspired by modern triage engines. It acts as a central hub for evidence triage, cryptographic custody, and unified timeline exploration:

* **In-Situ Ingestion:** Analyzes disk images (`E01`, `RAW`, `DD`) and files directly at their original storage location without duplication, calculating streaming MD5, SHA-1, and SHA-256 hashes simultaneously in a single pass.
* **Cryptographic Chained Custody:** Every case modification, evidence addition, module run, and forensic bookmark is cryptographically sealed in an append-only hash chain (`custodia.jsonl`) using SHA-256 state hashing.
* **Declarative Module Pipeline:** Dynamic plug-and-play modules defined via `modulo.yaml`:
  * `hayabusa`: Ultra-fast EVTX Windows event log analysis with Sigma detection rules.
  * `mftecmd`: Rapid NTFS `$MFT` timeline parser.
  * `hindsight`: Chrome, Edge, and Brave browser forensics.
  * `iped-lite`: Deep content examination, Lucene indexing, carving, and OCR (spawned in-process or headless).
* **Unified SQLite WAL Database:** Consolidates all findings, alerts, timelines, and extracted entities (IPs, users, domains, hashes) in a single portable database (`caso.db`).

### Running the Go Orchestrator Hub

```bash
# Build the Go hub executable
cd core-go
go build -o iped-hub.exe ./cmd/iped-hub

# Create a new forensic case
iped-hub.exe novo /path/to/case "Operation Name" "Examiner Name"

# Ingest an evidence file in-situ
iped-hub.exe evidencia /path/to/case /path/to/evidence.raw

# Run fast triage pipeline
iped-hub.exe triagem /path/to/case <evidence_id>

# Inspect high-severity alerts and IOCs
iped-hub.exe alertas /path/to/case

# Query correlated entities (IPs, accounts, domains)
iped-hub.exe entidades /path/to/case ip

# Verify cryptographic chain of custody
iped-hub.exe verificar /path/to/case

# Launch local interactive Web UI dashboard (pure Go, zero external dependencies)
iped-hub.exe web /path/to/case --porta 8080 --abrir

# Launch MCP Server for AI agents
iped-hub.exe mcp
```

### Full Go Hub MCP Tools Reference (19 Tools)

| Tool | Category | Description |
| :--- | :--- | :--- |
| `iped_create_case` | Case Management | Creates a new case with SQLite WAL and chained custody. |
| `iped_open_case` | Case Management | Opens an existing case directory. |
| `iped_case_status` | Case Management | Returns counts of evidence, timeline events, bookmarks, and custody status. |
| `iped_custody_verify` | Chain of Custody | Recalculates and cryptographically validates all custody records. |
| `iped_evidence_add` | Evidence Ingestion | Ingests evidence in-situ (without disk duplication) with streaming multi-hash. |
| `iped_evidence_list` | Evidence Ingestion | Lists all registered evidence items and hashes. |
| `iped_identify_evidence` | Triage Engine | Identifies evidence format by magic bytes signature and extension. |
| `iped_modules_list` | Triage Engine | Lists all declarative analysis modules registered in the system. |
| `iped_triage_plan` | Triage Engine | Assembles an optimal execution plan (fast triage first, deep analysis optional). |
| `iped_triage_run` | Triage Engine | Executes triage modules and consolidates findings into the timeline. |
| `iped_timeline_search` | Search & Triage | Searches the consolidated multi-source event timeline with pagination. |
| `iped_alerts_list` | Search & Triage | Queries critical/high security alerts and Sigma detections. |
| `iped_entities_query` | Search & Triage | Correlates extracted entities (IPs, users, services) with timeline events. |
| `iped_bookmarks_add` | Analysis & Notes | Adds a forensic bookmark and note to evidence/items. |
| `iped_bookmarks_list` | Analysis & Notes | Lists all forensic bookmarks in the active case. |
| `iped_web_serve` | Visualization | Launches or returns the local Web UI dashboard URL for human examiner inspection. |
| `iped_deep_process` | Deep Examination | Launches headless IPED Lite Java examination in background. |
| `iped_deep_status` | Deep Examination | Tracks progress and duration of the background Java worker. |
| `iped_import_iped_case` | Deep Examination | Ingests existing IPED case files (`FileList.csv`) into the unified SQLite database. |

---

## License

IPED Lite is released under the GNU General Public License v3.0 (GPL-3.0), matching upstream IPED.
