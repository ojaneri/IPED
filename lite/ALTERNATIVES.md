# Alternatives to a lighter IPED

Web research done on 2026-10-31 by a research agent (report not independently re-checked; items marked
[unverified] are from directory listings or background knowledge, not from primary sources that were read).

## What exists

| Option | What it is | Fit for "light + MCP" |
|---|---|---|
| **IPED Web API** (`lib/iped-webapi.jar`) | HTTP server for already processed cases: search, document properties/content/text/thumbnail, bookmarks, categories, selection. Wiki: https://github.com/sepinf-inc/IPED/wiki/Web-API (no auth described). | No IPED MCP server was found, so a thin MCP shim over it is new work, but cheap. Needs a JVM and a processed case. Its heap use was not measured by the researcher (we measure it in this project). |
| **Autopsy MCP** | Built in since Autopsy 4.23.0, read-only, stdio + localhost TCP, results capped at 1 MB. https://sleuthkit.org/autopsy/docs/user-docs/4.23.0/claude_mcp_page.html | Autopsy must be running with the case open: heavy JVM, does not help RAM. |
| **Mulder** (calebevans/mulder, Apache-2.0) | 35+ tools as 140+ MCP operations: Volatility 3, Sleuthkit, Plaso, carvers, EZ Tools, YARA, ALEAPP/iLEAPP/MVT. Linux/SIFT first, Docker needs `--privileged`, about 2.2 GB native setup. | Broad, but Linux-oriented and not Windows-first. |
| sleuthkit-mcp-server (heejung0) | 8 tools wrapping mmls/fls/icat/istat/ifind. Maintenance and license [unverified]. | Light, disk images only. |
| mcp_dfir (androsh7) | Dockerised Volatility3 + Sleuthkit. [unverified] | Memory/disk, not email/mobile. |
| Volatility / Velociraptor MCPs | Several small projects, need a Volatility install or a Velociraptor server. | Different problem (memory forensics, endpoint). |
| **ufedlib_py** (MIT, 1.0.0, 2026-02-22) | Pure Python UFDR/report.xml parser, single pass XML stream. https://pypi.org/project/ufedlib-py/ | Candidate for UFDR without IPED. Coverage across Cellebrite versions unverified. |
| Python mbox/PST | `mailbox` (stdlib), pypff/libratom (PST), extract-msg (.msg only). | Light if you iterate without building lists. |
| SQLite FTS5 / Tantivy | Full-text index with small memory. FTS5 data about 32% of DB size by default, about 7.6% with `detail=none`. | Good for search-only agents; no multi-GB benchmark found. |

## Hard to replace from IPED

Parser coverage (Tika plus mature chat parsers), carving (40+ formats), hash databases (NSRL, LED),
OCR, hash dedup and rule based categories, chain of custody and reporting.

## Easy to replace for an agent that only searches, reads, bookmarks and exports

File system access (Sleuthkit CLI or pytsk3), hashing and dedup (hashlib + SQLite), file signature and
category (libmagic or extension map), mbox and PST iteration, full-text search (FTS5), UFDR via
ufedlib_py or streaming XML, bookmarks and export as SQLite tables plus zip.

## Options identified

- (a) Custom light stack: SQLite + FTS5 + Sleuthkit + Python parsers. Small footprint, but we carry
  maintenance and parser gaps.
- (b) Process once with IPED, serve with its Web API plus a small MCP shim. Keeps parser coverage,
  cheap queries; processing itself still needs a JVM.
- (c) Hybrid: IPED for heavy parsing, light Python for the rest.

Our own measurements for this project are in `RESULTS.md`.

## D:\ForensicsAI (the user's own framework), read-only exploration on 2026-10-31

Report of an exploring agent (verified by reading unless marked inferred). No case data was read.

- **What it is:** a Portuguese language lab for MOBILE forensics (iOS/Android) driven by Claude Code, aimed at the
  Brazilian laudo. Python 3.12, one stdio MCP server (`mcp_server/forensics_mcp/server.py`, 54 tools, FastMCP),
  `CLAUDE.md` (~1,000 lines of rules and field lessons), 7 skills, 7 slash commands, permission tiers in
  `.claude/settings.json` (read-only tools allowed, state-changing tools "ask", writes to acquisition folders and the
  custody ledger denied), and a local web panel (127.0.0.1:8765) that exposes only processing, analysis and report folders.
- **Wraps:** adb, androidqf, libimobiledevice, MVT, iLEAPP, ALEAPP, APOLLO, Sleuth Kit (mmls/fls/icat), PhotoRec,
  exiftool, SQLite, in-house WhatsApp/SMS readers.
- **Does not have:** IPED, full-text index (search is plain scans: ~5 s over 384k messages), UFDR parsing (only
  "import the extracted folder", TODO), mbox/PST, reliable E01 (depends on libewf in the Sleuth Kit build).
- **Storage:** flat JSON/JSONL/CSV per case folder, no central database. SQLite only to read evidence DBs, always
  `immutable=1`.
- **Conventions worth copying:** hash-chained append-only custody ledger (`custody.py`, ~200 lines, no dependencies),
  `runner.py` that logs every external command (argv, exit code, duration, stdout/stderr) and has background jobs with
  `processing_status` tools, chunked hashing with SHA-256+MD5 manifests, SELECT-only immutable SQLite queries, every
  list tool takes a `limite` and returns `truncado`, large results are written to files and only summarized, "an error
  with zeroed stats is a failure, not a clean result", findings immutable with errata, a `doctor` bootstrap tool.
- **Pain points it admits:** a tool output of ~175k chars exceeded the return limit; iLEAPP reached ~5 GB RSS and ~9 GB
  committed on a 16 GB machine (96% use); blocking tools with no background mode; several tools not yet validated on
  real data.
- **Insight for this project:** the two frameworks are complementary. ForensicsAI has the agent discipline
  (custody, limits, permissions, background jobs) but lacks exactly what IPED has (UFDR, mbox/PST, E01, indexed search).
  Lowest-risk integration: run IPED once as a background job into the case's processing folder, record it in the
  ledger, and serve queries from a light process (IPED Web API or a thin MCP), keeping the 2-4 GB JVM out of the agent
  loop. If going Python-only, the missing pieces are SQLite FTS5, Sleuth Kit with an explicit E01 path, and a UFDR reader.

## ufedlib_py, deep dive (2026-10-31)

Report of an agent that downloaded the sdist and read the source without importing or running it. Verified:
PyPI metadata, the source files, absence of a GitHub repo for it, the C# repo stats, and the cited IPED lines.
Inferred: real RAM per object, behaviour on invalid XML characters, completeness of the typed models.

**Verdict: do not depend on it.** It is a thin `report.xml` to dataclass mapper, not a UFDR reader.

- **Provenance:** a single release (1.0.0, 2026-02-22) by an author with no email, no project URLs and no repository
  found. It claims to be a port of the C# UFEDLib (https://github.com/SEilers/UFEDLib, MIT, 5 stars, about 30 commits,
  last push 2026-09-07). No security problem was found: no network, eval, pickle or telemetry; one subprocess
  (`pg_restore`, only if present) and extraction of two fixed names to a temp folder. Tests: only `import ufedlib_py`.
- **What it does:** 45 UFED model types (Contact, Call, Chat, InstantMessage, Email, Location, VisitedPage, InstalledApplication,
  UserAccount, FinancialAccount, ...), `scan_models`, `parse_data`, `parse_all`, a CLI. Stdlib only.
- **What it does not do:** no `<taggedFiles>`/`<file>` entries (so no media files), no attachment to zip entry binding
  (the attachment path is copied as a raw string), no `jumptargets`/`sourcemodels`/Base64 values, quoted/forwarded/reply
  messages only as generic dicts.
- **Memory:** it uses `iterparse` and clears top level elements (good), but returns everything as Python lists
  (RAM grows linearly; estimate 1 to 3 KB per message, not measured), keeps cleared elements attached to the root,
  deep-copies nested models, and `parse_all` re-reads and re-decompresses `report.xml` about 7 times. One helper in
  `metadata/_xml_sections.py` builds the whole tree in memory when a section is absent (OOM risk on a huge report).
- **Robustness:** one XML parse error can silently discard all parsed models; no sanitising of invalid XML characters
  (IPED has `UFEDXMLWrapper` for exactly that); Device Info for report 8.5+ needs `pg_restore`.
- **Versus IPED's `UfedXmlReader` (1613 lines):** IPED additionally handles files and their binding to zip entries,
  MD5 based content reuse, attachment path fixing, contact photos, extraction info and media classification,
  Base64, SMS/MMS/call details, the XML sanitiser, protected report detection, and typed chat models via
  `UfedModelHandler`. ufedlib_py has typed dataclasses for ~45 types (IPED keeps most as generic metadata).
- **Recommendation:** write our own streaming layer (single pass `iterparse` straight into SQLite, an index of the zip
  central directory, attachment to entry mapping, sanitiser, resume after errors) and use ufedlib_py only as a reference
  for the field names, or the C# UFEDLib for the same schema knowledge. The source copy lives in the session scratchpad
  and is not part of the repo.
