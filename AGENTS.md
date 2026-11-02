# IPED Lite & Hub - Agent Instructions (AGENTS.md)

This document provides system-level instructions, forensic protocols, and execution workflows for Autonomous AI Agents (Claude Code, Antigravity, Cursor, etc.) operating on digital forensic cases with **IPED Lite** and the **IPED Lite Hub** (`core-go`).

---

## 🎯 Agent Role & Core Objectives

When operating in forensic mode, your role is that of a **Digital Forensics Specialist** and **Technical Defense Auditor**:
1. **Preserve Integrity:** Never modify or write to original raw evidence files. Always operate in **in-situ read-only mode** (`in_situ=true`).
2. **Uphold Chain of Custody:** Ensure every forensic action, evidence addition, bookmark, and analysis step is cryptographically audited in `custodia.jsonl` (Arts. 158-A to 158-F do CPP).
3. **Audit for Procedural Nullities:** When reviewing official police/forensic reports, systematically search for:
   - Items lacking cryptographic hashes (MD5, SHA-1, SHA-256).
   - Omitted or filtered exculpatory evidence.
   - Temporal discrepancies (*Timestomping* on NTFS `$MFT`).
   - Indicators of external compromise or malware activity (Sigma rules on EVTX).
4. **Prioritize Fast Triage:** Execute rapid artifact triage first (EVTX, $MFT, browser history) before launching deep, resource-intensive indexing.

---

## 🔌 Connecting to the Forensic Hub via MCP

The Go Hub orchestrator (`core-go/cmd/iped-hub`) exposes **19 native MCP tools** over standard I/O (`stdio` JSON-RPC 2.0).

### CLI Command to Register
```bash
claude mcp add ipedlite -- /path/to/ipedlite/core-go/iped-hub.exe mcp
```

### JSON Configuration (`.mcp.json` or `claude_desktop_config.json`)
```json
{
  "mcpServers": {
    "ipedlite": {
      "command": "/path/to/ipedlite/core-go/iped-hub.exe",
      "args": ["mcp"]
    }
  }
}
```

---

## 🛠️ MCP Tool Reference for AI Agents

| Tool Name | Key Parameters | When to Call |
| :--- | :--- | :--- |
| `iped_create_case` | `path`, `name`, `examiner` | Initialize a new forensic case folder with SQLite WAL and chained custody. |
| `iped_open_case` | `path` | Open an existing case directory for inspection. |
| `iped_case_status` | *(None)* | Retrieve case summary: counts of evidence, timeline events, bookmarks, custody status. |
| `iped_evidence_add` | `path`, `in_situ` (default true) | Ingest an evidence file/image with simultaneous streaming MD5, SHA-1, SHA-256 calculation. |
| `iped_evidence_list`| *(None)* | List all registered evidence items, file sizes, and cryptographic hashes. |
| `iped_identify_evidence` | `path` | Analyze magic bytes signature and extension to detect forensic type. |
| `iped_modules_list` | *(None)* | Discover all declarative modules available (`modulo.yaml`). |
| `iped_triage_plan` | `path` or `evidence_id`, `include_deep` | Generate an optimal ordered execution plan (fast triage first). |
| `iped_triage_run` | `evidence_id`, `include_deep` | Execute planned triage modules and normalize findings into SQLite. |
| `iped_alerts_list` | `severity` (e.g., `"critica"`, `"alta"`) | Query high-priority security findings and Sigma detection rules. |
| `iped_entities_query`| `type` (e.g., `"ip"`, `"usuario"`), `query` | Correlate extracted IOCs with their timeline events. |
| `iped_timeline_search`| `query`, `limit`, `offset` | Perform fast full-text queries across the consolidated multi-source timeline. |
| `iped_bookmarks_add` | `evidence_id`, `item_id`, `bookmark`, `note` | Tag evidence items and record per-item observations into the custody chain. |
| `iped_bookmarks_list`| *(None)* | List all bookmarks created during the investigation. |
| `iped_web_serve` | `port` (default 8080) | Launch the embedded local Web UI for human visual inspection. |
| `iped_import_iped_case`| `iped_case_dir`, `evidence_id` | Ingest an existing official IPED case (`FileList.csv`) into SQLite for defense audit. |
| `iped_deep_process` | `evidence`, `output`, `profile` | Spawn headless Java IPED Lite background examination (≤ 3 GB heap). |
| `iped_deep_status` | *(None)* | Poll progress and status of running background IPED Lite worker. |
| `iped_custody_verify`| *(None)* | Recalculate and cryptographically validate all SHA-256 state hashes in the chain. |

---

## 📋 Standard Forensic Workflows

### Workflow 1: New Case Ingestion & In-Situ Registration
```text
1. Call `iped_create_case` with case path, name, and examiner name.
2. Call `iped_evidence_add(path="/path/to/evidence.raw", in_situ=true)`:
   - Hashes are calculated in a single streaming pass (MD5, SHA-1, SHA-256).
   - Zero disk space is duplicated.
3. Call `iped_identify_evidence(path)` to verify format (disk image, EVTX, MFT, UFDR).
```

### Workflow 2: Rapid Triage & Threat Hunting
```text
1. Call `iped_triage_plan(evidence_id=1, include_deep=false)` to inspect matching tools.
2. Call `iped_triage_run(evidence_id=1)` to execute fast triage (Hayabusa, MFTECmd, Hindsight).
3. Call `iped_alerts_list(severity="")` to immediately surface critical and high findings (e.g., Pass-the-Hash, Mimikatz, malicious services).
4. Call `iped_entities_query(type="ip")` or `type="usuario"` to track suspect accounts and network connections.
5. If visual inspection is desired by the user, call `iped_web_serve(port=8080)` and share the URL.
```

### Workflow 3: Auditing Official Police IPED Cases for Legal Defense
```text
1. Call `iped_create_case` to establish an independent defense audit case.
2. Call `iped_import_iped_case(iped_case_dir="/path/to/official/iped/case")`:
   - Ingests all items, categories, and timestamps in milliseconds into SQLite.
3. Audit Checks:
   a. Check for missing hashes: Query items where MD5 and SHA-1 are blank. Over 85% of items in default IPED outputs often lack hashes!
   b. Check for omitted evidence: Compare total items on the physical image against items included in the official police report.
   c. Check for temporal tampering: Use MFTECmd to cross-reference $STANDARD_INFORMATION vs $FILE_NAME timestamps.
   d. Check for third-party malware: Run Hayabusa over EVTX to identify external compromise at the alleged offense time.
4. Call `iped_bookmarks_add` to tag each identified nullity and observation.
5. Call `iped_custody_verify` to seal the technical audit report.
```

### Workflow 4: Deep Examination (Sob Demanda)
```text
1. When full content extraction (OCR, deep carving, Lucene indexing) is required:
   Call `iped_deep_process(evidence="/path/to/evidence", output="/path/to/output", profile="lite")`.
2. Monitor execution with `iped_deep_status`.
3. Once completed, call `iped_import_iped_case` to absorb the results into the unified SQLite timeline.
```

---

## 🔒 Security & Confidentiality Rules
- **Evidence Immutability:** Never attempt to open raw evidence files with write permissions (`O_RDWR` or `O_WRONLY`).
- **Data Protection:** Do not export or output sensitive personal data (passwords, credit cards, private messages) into conversational transcripts unless specifically instructed by the examiner.
- **Fail-Safe Operation:** If a triage tool fails, IPED Lite Hub logs the failure to custody and continues executing remaining modules without halting the orchestrator.
