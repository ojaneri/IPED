# IPED Lite

**IPED Lite** is a lightweight, resource-optimized distribution of the [IPED Digital Forensic Tool](https://github.com/sepinf-inc/IPED), featuring an ultra-low memory footprint and a native **Model Context Protocol (MCP)** server for autonomous AI agent integration.

It is designed to run efficiently on commodity workstations and laptops with limited memory (such as 8 GB RAM machines, operating under a strict ≤ 3 GB JVM heap limit), while enabling AI agents to programmatically query, inspect, and tag forensic evidence via standard stdio JSON-RPC.

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

### 2. Performance Comparison
On identical large email/mbox test sets (26,710 items) under a 2.0 GB heap limit:
* **Standard Master:** Failed with concurrency errors and completed in 1,661 seconds.
* **IPED Lite:** Finished with 0 errors in 742 seconds (**> 2.2x faster** with zero OutOfMemory errors).

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

## Agent Configuration

### 1. Claude Desktop / Generic MCP Client
Add the server entry to your `claude_desktop_config.json` or `.mcp.json`:

```json
{
  "mcpServers": {
    "ipedlite": {
      "command": "java",
      "args": [
        "-cp",
        "path/to/ipedlite/iped.jar;path/to/ipedlite/lib/*",
        "iped.engine.mcp.McpServer"
      ]
    }
  }
}
```

### 2. Claude Code CLI
Register the server directly via command line:

```bash
claude mcp add ipedlite -- java -cp "path/to/ipedlite/iped.jar;path/to/ipedlite/lib/*" iped.engine.mcp.McpServer
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

## License

IPED Lite is released under the GNU General Public License v3.0 (GPL-3.0), matching upstream IPED.
