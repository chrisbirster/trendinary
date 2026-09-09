#!/usr/bin/env python3
"""Fail closed when an Atlas dry-run contains destructive schema operations."""

from __future__ import annotations

import argparse
import re
import sys
from pathlib import Path


RULES: tuple[tuple[str, re.Pattern[str]], ...] = (
    ("DROP TABLE", re.compile(r"\bDROP\s+TABLE\b", re.IGNORECASE)),
    ("DROP COLUMN", re.compile(r"\bDROP\s+COLUMN\b", re.IGNORECASE)),
    ("DROP VIEW", re.compile(r"\bDROP\s+VIEW\b", re.IGNORECASE)),
    (
        "table/column rename",
        re.compile(r"\bALTER\s+TABLE\b[^;]*\bRENAME(?:\s+COLUMN|\s+TO)?\b", re.IGNORECASE),
    ),
    (
        "incompatible ALTER TABLE constraint/column change",
        re.compile(
            r"\bALTER\s+TABLE\b[^;]*\b(?:ADD\s+CONSTRAINT|ALTER\s+COLUMN|DROP\s+CONSTRAINT)\b",
            re.IGNORECASE,
        ),
    ),
    (
        "SQLite rebuild / foreign-key disable",
        re.compile(r"\bPRAGMA\s+foreign_keys\s*=\s*(?:OFF|0)\b", re.IGNORECASE),
    ),
    ("TRUNCATE", re.compile(r"\bTRUNCATE(?:\s+TABLE)?\b", re.IGNORECASE)),
)

IDENT = r"[`\"\[]?([A-Za-z_][A-Za-z0-9_$.-]*)[`\"\]]?"
CREATE_TABLE = re.compile(rf"\bCREATE\s+TABLE(?:\s+IF\s+NOT\s+EXISTS)?\s+{IDENT}", re.IGNORECASE)
CREATE_UNIQUE_INDEX = re.compile(
    rf"\bCREATE\s+UNIQUE\s+INDEX(?:\s+IF\s+NOT\s+EXISTS)?\s+{IDENT}\s+ON\s+{IDENT}",
    re.IGNORECASE,
)


def strip_comments(sql: str) -> str:
    sql = re.sub(r"/\*.*?\*/", " ", sql, flags=re.DOTALL)
    sql = re.sub(r"--[^\n]*", " ", sql)
    return sql


def statement_for(text: str, start: int) -> str:
    left = text.rfind(";", 0, start) + 1
    right = text.find(";", start)
    if right == -1:
        right = len(text)
    snippet = " ".join(text[left:right].split())
    return snippet[:500]


def findings(plan: str) -> list[tuple[str, str]]:
    cleaned = strip_comments(plan)
    out: list[tuple[str, str]] = []
    seen: set[tuple[str, str]] = set()

    # Atlas renders UNIQUE constraints as CREATE UNIQUE INDEX statements even
    # for tables created in the same additive plan. Those are safe because no
    # existing rows can violate the new constraint. A new unique index on a
    # pre-existing table remains contract-like and is blocked by default.
    created_tables = {match.group(1).lower() for match in CREATE_TABLE.finditer(cleaned)}

    for label, pattern in RULES:
        for match in pattern.finditer(cleaned):
            item = (label, statement_for(cleaned, match.start()))
            if item not in seen:
                out.append(item)
                seen.add(item)

    for match in CREATE_UNIQUE_INDEX.finditer(cleaned):
        target_table = match.group(2).lower()
        if target_table in created_tables:
            continue
        item = ("new uniqueness constraint on existing table", statement_for(cleaned, match.start()))
        if item not in seen:
            out.append(item)
            seen.add(item)

    return out


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("plan", type=Path, help="Atlas --dry-run output captured to a file")
    parser.add_argument(
        "--allow-destructive",
        action="store_true",
        help="explicit maintenance-only override; ordinary release workflows must never pass this",
    )
    args = parser.parse_args()

    if not args.plan.is_file():
        print(f"::error::Atlas plan file does not exist: {args.plan}", file=sys.stderr)
        return 2

    plan = args.plan.read_text(encoding="utf-8", errors="replace")
    matches = findings(plan)
    if not matches:
        print("Atlas plan policy: additive/non-destructive plan accepted")
        return 0

    prefix = "::warning::" if args.allow_destructive else "::error::"
    print(
        f"{prefix}Atlas plan contains {len(matches)} destructive or backward-incompatible operation(s)",
        file=sys.stderr,
    )
    for label, statement in matches:
        print(f"  - {label}: {statement}", file=sys.stderr)

    if args.allow_destructive:
        print(
            "DESTRUCTIVE SCHEMA OVERRIDE ACCEPTED: this is permitted only from the dedicated audited maintenance workflow",
            file=sys.stderr,
        )
        return 0

    print(
        "Ordinary releases are expand-only. Ship a compatible expand release first; perform contract cleanup later through the dedicated maintenance workflow.",
        file=sys.stderr,
    )
    return 1


if __name__ == "__main__":
    raise SystemExit(main())
