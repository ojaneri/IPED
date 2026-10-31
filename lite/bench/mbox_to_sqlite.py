"""Prototype of the "direct" strategy: index a mbox into SQLite FTS5 with plain Python, no IPED.

It is only a measurement tool to compare memory/time with an IPED run on the same input. It indexes
headers, plain text of the bodies and attachment names/sizes/sha256, and runs a few queries. It prints
only counts, sizes and timings (never message content).

Usage: python -I mbox_to_sqlite.py <mbox file> <db file>
"""
import ctypes
import email
import email.policy
import hashlib
import mailbox
import re
import sqlite3
import sys
import time
from ctypes import wintypes


class PMC(ctypes.Structure):
    _fields_ = [("cb", wintypes.DWORD), ("PageFaultCount", wintypes.DWORD),
                ("PeakWorkingSetSize", ctypes.c_size_t), ("WorkingSetSize", ctypes.c_size_t),
                ("QuotaPeakPagedPoolUsage", ctypes.c_size_t), ("QuotaPagedPoolUsage", ctypes.c_size_t),
                ("QuotaPeakNonPagedPoolUsage", ctypes.c_size_t), ("QuotaNonPagedPoolUsage", ctypes.c_size_t),
                ("PagefileUsage", ctypes.c_size_t), ("PeakPagefileUsage", ctypes.c_size_t)]


def mem_mb():
    pmc = PMC()
    pmc.cb = ctypes.sizeof(PMC)
    h = ctypes.windll.kernel32.GetCurrentProcess()
    ctypes.windll.psapi.GetProcessMemoryInfo(h, ctypes.byref(pmc), pmc.cb)
    return pmc.WorkingSetSize / 2**20, pmc.PeakWorkingSetSize / 2**20


TAGS = re.compile(r"<[^>]+>")


def body_text(msg):
    part = msg.get_body(preferencelist=("plain", "html"))
    if part is None:
        return ""
    try:
        text = part.get_content()
    except Exception:
        return ""
    if part.get_content_type() == "text/html":
        text = TAGS.sub(" ", text)
    return text


def main(mbox_path, db_path):
    t0 = time.time()
    con = sqlite3.connect(db_path)
    con.execute("PRAGMA journal_mode=OFF")
    con.execute("PRAGMA synchronous=OFF")
    con.execute("PRAGMA cache_size=-65536")  # 64 MB page cache
    con.execute("CREATE TABLE msg(id INTEGER PRIMARY KEY, msgid TEXT, date TEXT, sender TEXT, recipients TEXT, "
                "subject TEXT, offset INTEGER, nattach INTEGER)")
    con.execute("CREATE TABLE attach(msg INTEGER, name TEXT, mime TEXT, size INTEGER, sha256 TEXT)")
    con.execute("CREATE TABLE bookmark(name TEXT, msg INTEGER)")
    con.execute("CREATE VIRTUAL TABLE fts USING fts5(subject, sender, recipients, body, content='')")

    mb = mailbox.mbox(mbox_path, factory=lambda f: email.message_from_binary_file(f, policy=email.policy.default))
    n = 0
    attachments = 0
    errors = 0
    for key in mb.iterkeys():
        try:
            msg = mb[key]
            subject = str(msg.get("subject", "") or "")
            sender = str(msg.get("from", "") or "")
            rcpt = " ".join(str(msg.get(h, "") or "") for h in ("to", "cc"))
            body = body_text(msg)
            atts = []
            for part in msg.iter_attachments():
                payload = part.get_payload(decode=True) or b""
                atts.append((part.get_filename() or "", part.get_content_type(), len(payload),
                             hashlib.sha256(payload).hexdigest()))
            cur = con.execute("INSERT INTO msg(msgid,date,sender,recipients,subject,offset,nattach) VALUES(?,?,?,?,?,?,?)",
                              (str(msg.get("message-id", "") or ""), str(msg.get("date", "") or ""), sender, rcpt,
                               subject, key, len(atts)))
            rid = cur.lastrowid
            con.executemany("INSERT INTO attach VALUES(?,?,?,?,?)", [(rid,) + a for a in atts])
            attachments += len(atts)
            att_names = " ".join(a[0] for a in atts)
            con.execute("INSERT INTO fts(rowid,subject,sender,recipients,body) VALUES(?,?,?,?,?)",
                        (rid, subject, sender, rcpt, body + " " + att_names))
            n += 1
        except Exception:
            errors += 1
        mb._toc  # keep the index (offsets only) alive
        if n % 2000 == 0 and n:
            con.commit()
    con.commit()
    t1 = time.time()
    cur_mb, peak_mb = mem_mb()
    print(f"ingest: messages={n} attachments={attachments} errors={errors} time_s={t1 - t0:.0f} "
          f"ws_mb={cur_mb:.0f} peak_ws_mb={peak_mb:.0f}")

    for term in ("contrato", "pagamento", "documento", "reuniao", "anexo"):
        q0 = time.time()
        rows = con.execute("SELECT rowid FROM fts WHERE fts MATCH ? LIMIT 1000", (term,)).fetchall()
        print(f"query '{term}': hits(limit 1000)={len(rows)} ms={(time.time() - q0) * 1000:.0f}")
    cur_mb, peak_mb = mem_mb()
    print(f"after queries: ws_mb={cur_mb:.0f} peak_ws_mb={peak_mb:.0f}")
    con.close()


if __name__ == "__main__":
    main(sys.argv[1], sys.argv[2])
