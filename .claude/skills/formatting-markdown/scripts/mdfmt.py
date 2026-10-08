#!/usr/bin/env python3
"""Format Markdown the way PhpStorm / GoLand like it.

- prose, list items and block quotes wrapped at --width (default 120)
- wrapped list items get a hanging indent
- a blank line around headings, fenced code, tables and rules; no runs of blank lines
- tables padded into aligned columns, separator row filled to the column width
- code, front matter, HTML blocks and link definitions are left alone

Usage: mdfmt.py [--width N] [--check] [--reflow] [--runts] FILE...
  --check   change nothing; exit 1 if any file would change.
  --reflow  also join and re-wrap the lines of each paragraph (opt-in; never used by the hook).
  --runts   only report short leftover lines of an old wrap; change nothing.
Standard library only. Idempotent.
"""
import argparse
import re
import sys
import unicodedata

FENCE_RE = re.compile(r"^ {0,3}(`{3,}|~{3,})")
HEADING_RE = re.compile(r"^ {0,3}#{1,6}(\s|$)")
HR_RE = re.compile(r"^ {0,3}([-*_])( *\1){2,} *$")
LIST_RE = re.compile(r"^( {0,3})([-+*]|\d{1,9}[.)])( +|$)(.*)$")
QUOTE_RE = re.compile(r"^ {0,3}> ?")
ORDERED_RE = re.compile(r"^( {0,3})(\d{1,9})([.)])( |$)")
SETEXT_RE = re.compile(r"^ {0,3}(=+|-+) *$")
HTML_RE = re.compile(r"^ {0,3}<[A-Za-z/!?]")
REFDEF_RE = re.compile(r"^ {0,3}\[\^?[^\]]+\]:\s")
SEP_CELL_RE = re.compile(r"^:?-+:?$")
RISKY_START_RE = re.compile(r"^([-+*>#]+|\d{1,9}[.)])$")
PIPE_RE = re.compile(r"(?<!\\)\|")


def width_of(s):
    n = 0
    for ch in s:
        if unicodedata.combining(ch) or ch in "‍️︎":
            continue
        n += 2 if unicodedata.east_asian_width(ch) in ("W", "F") else 1
    return n


def indent_of(line):
    return len(line) - len(line.lstrip(" "))


# ---------------------------------------------------------------- tables

def split_row(line):
    s = line.strip()
    if s.startswith("|"):
        s = s[1:]
    if s.endswith("|") and not s.endswith("\\|"):
        s = s[:-1]
    return [c.strip() for c in PIPE_RE.split(s)]


def is_separator(line):
    if "-" not in line or "|" not in line and ":" not in line and not line.strip().startswith("-"):
        return False
    cells = split_row(line)
    return bool(cells) and all(SEP_CELL_RE.match(c) for c in cells)


def table_starts(lines, i):
    return (
        i + 1 < len(lines)
        and "|" in lines[i]
        and is_separator(lines[i + 1])
        and len(split_row(lines[i])) == len(split_row(lines[i + 1]))
    )


def format_table(rows, aligns):
    n = len(aligns)
    rows = [(r + [""] * n)[:n] for r in rows]
    widths = [max(1, *(width_of(r[c]) for r in rows)) for c in range(n)]

    def pad(text, w, a):
        gap = w - width_of(text)
        if a == "right":
            return " " * gap + text
        if a == "center":
            return " " * (gap // 2) + text + " " * (gap - gap // 2)
        return text + " " * gap

    def line(r):
        return "|" + "|".join(" " + pad(r[c], widths[c], aligns[c]) + " " for c in range(n)) + "|"

    def sep():
        out = []
        for c in range(n):
            w = widths[c] + 2
            a = aligns[c]
            if a == "center":
                out.append(":" + "-" * (w - 2) + ":")
            elif a == "left-explicit":
                out.append(":" + "-" * (w - 1))
            elif a == "right":
                out.append("-" * (w - 1) + ":")
            else:
                out.append("-" * w)
        return "|" + "|".join(out) + "|"

    return [line(rows[0]), sep()] + [line(r) for r in rows[1:]]


def align_of(cell):
    l, r = cell.startswith(":"), cell.endswith(":")
    return "center" if l and r else "right" if r else "left-explicit" if l else "left"


# ------------------------------------------------------------- paragraphs

def tokenize(text):
    """Split on spaces, keeping `code spans` whole."""
    tokens, cur, i, n = [], [], 0, len(text)
    while i < n:
        ch = text[i]
        if ch == "`":
            j = i
            while j < n and text[j] == "`":
                j += 1
            run = text[i:j]
            end = text.find(run, j)
            if end != -1:
                cur.append(text[i:end + len(run)])
                i = end + len(run)
            else:
                cur.append(run)
                i = j
        elif ch == " ":
            if cur:
                tokens.append("".join(cur))
                cur = []
            i += 1
        else:
            cur.append(ch)
            i += 1
    if cur:
        tokens.append("".join(cur))
    merged = []
    for t in tokens:  # never let a wrap start a line with `-`, `1.`, `#`, `>`
        if merged and RISKY_START_RE.match(t):
            merged[-1] += " " + t
        else:
            merged.append(t)
    return merged


def wrap(text, width):
    lines, cur, cur_w = [], [], 0
    for t in tokenize(text):
        tw = width_of(t)
        if cur and cur_w + 1 + tw > width:
            lines.append(" ".join(cur))
            cur, cur_w = [t], tw
        else:
            cur_w += tw + (1 if cur else 0)
            cur.append(t)
    if cur:
        lines.append(" ".join(cur))
    return lines


def format_paragraph(raw, width, reflow=False):
    """Wrap lines that are too long. With reflow, also join the lines of a paragraph and re-wrap them
    (hard breaks stay). Without it no line the author kept apart is ever joined."""
    raw = raw[:-1] + [raw[-1].rstrip()]  # a hard break after the last line breaks nothing
    out = []
    if reflow:
        segments, cur = [], []
        for line in raw:
            cur.append(line.strip())
            if line.endswith("  "):
                segments.append((cur, True))
                cur = []
        if cur:
            segments.append((cur, False))
        for seg, hard in segments:
            wrapped = wrap(" ".join(seg), width)
            if hard and wrapped:
                wrapped[-1] += "  "
            out.extend(wrapped)
        return out
    for line in raw:
        hard = line.endswith("  ")
        wrapped = wrap(line.strip(), width)
        if hard and wrapped:
            wrapped[-1] += "  "
        out.extend(wrapped)
    return out


# ------------------------------------------------------------- blocks

def interrupts_paragraph(lines, i):
    line = lines[i]
    if not line.strip():
        return True
    if FENCE_RE.match(line) or HEADING_RE.match(line) or HR_RE.match(line) or QUOTE_RE.match(line):
        return True
    m = LIST_RE.match(line)
    if m and m.group(4).strip() and (m.group(2) in "-+*" or m.group(2)[:-1] == "1"):
        return True
    return table_starts(lines, i)


def list_numbers(lines, i, ind, delim):
    """Numbers of the ordered items of the list that starts at lines[i] (same indent and delimiter)."""
    nums = []
    for k in range(i, len(lines)):
        line = lines[k].replace("\t", "    ")
        m = ORDERED_RE.match(line)
        if m and len(m.group(1)) == ind and m.group(3) == delim:
            nums.append(int(m.group(2)))
        elif line.strip() and indent_of(line) <= ind:
            break
    return nums


def format_lines(lines, width, reflow=False):
    width = max(width, 30)
    out, i, n = [], 0, len(lines)
    pending_blank = False
    force_blank = False
    olist = None  # the ordered list being numbered: indent, delimiter, first number, same-number style, count

    def begin(separate=False):
        nonlocal pending_blank, force_blank
        if out and (pending_blank or force_blank or separate):
            if out[-1] != "":
                out.append("")
        pending_blank = force_blank = False

    def verbatim_block(block, around):
        begin(around)
        out.extend(block)
        nonlocal force_blank
        force_blank = around

    while i < n:
        line = lines[i].rstrip("\n").replace("\t", "    ") if not FENCE_RE.match(lines[i]) else lines[i].rstrip("\n")
        if not line.strip():
            pending_blank = True
            i += 1
            continue

        if not ORDERED_RE.match(line):
            olist = None  # anything but an ordered item ends the list being numbered

        m = FENCE_RE.match(line)
        if m:
            fence = m.group(1)
            j = i + 1
            while j < n and not re.match(r"^ {0,3}" + fence[0] + "{%d,} *$" % len(fence), lines[j].rstrip("\n")):
                j += 1
            block = [l.rstrip("\n") for l in lines[i:j + 1]]
            verbatim_block(block, True)
            i = j + 1
            continue

        if HEADING_RE.match(line):
            verbatim_block([line.rstrip()], True)
            i += 1
            continue

        if HR_RE.match(line) and not LIST_RE.match(line.replace("*", "-", 0)) or HR_RE.match(line):
            verbatim_block([line.rstrip()], True)
            i += 1
            continue

        if REFDEF_RE.match(line):
            verbatim_block([line.rstrip()], False)
            i += 1
            continue

        if HTML_RE.match(line):
            j = i
            while j < n and lines[j].strip():
                j += 1
            verbatim_block([l.rstrip("\n").rstrip() for l in lines[i:j]], False)
            i = j
            continue

        if table_starts(lines, i):
            header = split_row(lines[i])
            aligns = [align_of(c) for c in split_row(lines[i + 1])]
            rows = [header]
            j = i + 2
            while j < n and lines[j].strip() and "|" in lines[j] and not FENCE_RE.match(lines[j]) \
                    and not HEADING_RE.match(lines[j]):
                rows.append(split_row(lines[j]))
                j += 1
            verbatim_block(format_table(rows, aligns), True)
            i = j
            continue

        if QUOTE_RE.match(line):
            j = i
            inner = []
            while j < n and QUOTE_RE.match(lines[j]):
                inner.append(QUOTE_RE.sub("", lines[j].rstrip("\n"), count=1))
                j += 1
            body = format_lines(inner, width - 2, reflow)
            begin()
            out.extend(("> " + l) if l else ">" for l in body)
            i = j
            continue

        m = LIST_RE.match(line)
        if m:
            ind, marker, spaces, rest = len(m.group(1)), m.group(2), m.group(3), m.group(4)
            offset = ind + len(marker) + (len(spaces) if 1 <= len(spaces) <= 4 else 1)
            inner = [rest]
            j = i + 1
            while j < n:
                cur = lines[j].rstrip("\n").replace("\t", "    ")
                if not cur.strip():
                    k = j
                    while k < n and not lines[k].strip():
                        k += 1
                    if k < n and indent_of(lines[k].replace("\t", "    ")) >= offset:
                        inner.extend([""] * (k - j))
                        j = k
                        continue
                    break
                if indent_of(cur) >= offset:
                    inner.append(cur[offset:])
                    j += 1
                    continue
                if inner[-1] == "" or interrupts_paragraph(lines, j) or LIST_RE.match(cur) or HTML_RE.match(cur):
                    break
                inner.append(cur.lstrip())  # lazy continuation
                j += 1
            out_marker = marker
            om = ORDERED_RE.match(line)
            if om:
                number, delim = int(om.group(2)), om.group(3)
                if olist and olist["ind"] == ind and olist["delim"] == delim:
                    olist["count"] += 1
                else:
                    nums = list_numbers(lines, i, ind, delim)
                    same = len(nums) > 1 and len(set(nums)) == 1  # "1. 1. 1." style: every number alike
                    olist = {"ind": ind, "delim": delim, "first": number, "same": same, "count": 0}
                shown = olist["first"] + (0 if olist["same"] else olist["count"])
                out_marker = f"{shown}{delim}"
            else:
                olist = None
            new_off = ind + len(out_marker) + 1
            body = format_lines(inner, width - new_off, reflow)
            begin()
            head = " " * ind + out_marker
            if not body:
                out.append(head)
            else:
                out.append(head + " " + body[0])
                out.extend((" " * new_off + l) if l else "" for l in body[1:])
            i = j
            continue

        # indented code block (only at block start, after a blank line or the start)
        if indent_of(line) >= 4 and (not out or pending_blank):
            j = i
            while j < n and (not lines[j].strip() or indent_of(lines[j]) >= 4):
                j += 1
            while not lines[j - 1].strip():
                j -= 1
            verbatim_block([l.rstrip("\n") for l in lines[i:j]], False)
            i = j
            continue

        # paragraph (with setext heading check)
        j = i
        raw = []
        while j < n:
            cur = lines[j].rstrip("\n").replace("\t", "    ")
            if j > i and SETEXT_RE.match(cur):
                break
            if j > i and interrupts_paragraph(lines, j):
                break
            raw.append(cur)
            j += 1
        if j < n and SETEXT_RE.match(lines[j]) and j > i and not HR_RE.match(lines[j].rstrip("\n")) or \
                (j < n and j > i and re.match(r"^ {0,3}=+ *$", lines[j])):
            verbatim_block([r.rstrip() for r in raw] + [lines[j].rstrip()], True)
            i = j + 1
            continue
        begin()
        out.extend(format_paragraph(raw, width, reflow))
        i = j

    return out


def decode(data):
    """bytes -> (text with \\n endings, eol, bom). The file's own line-ending style is returned so it can be put back:
    CRLF (Windows), CR (classic Mac) or LF. Mixed endings go to whichever is most common (LF on a tie)."""
    bom = data.startswith(b"\xef\xbb\xbf")
    text = data[3:].decode("utf-8") if bom else data.decode("utf-8")
    crlf = text.count("\r\n")
    lone_cr = text.count("\r") - crlf
    lf = text.count("\n") - crlf
    eol = "\r\n" if crlf > lf and crlf >= lone_cr else "\r" if lone_cr > lf and lone_cr > crlf else "\n"
    return text.replace("\r\n", "\n").replace("\r", "\n"), eol, bom


def encode(text, eol, bom):
    out = text.replace("\n", eol).encode("utf-8")
    return b"\xef\xbb\xbf" + out if bom else out


def format_bytes(data, width, reflow=False):
    """The whole pipeline on raw file contents: keeps the file's line endings and BOM."""
    text, eol, bom = decode(data)
    return encode(format_text(text, width, reflow), eol, bom)


def format_text(text, width, reflow=False):
    lines = text.split("\n")
    head = []
    if lines and lines[0].strip() == "---":  # front matter
        for k in range(1, len(lines)):
            if lines[k].strip() in ("---", "..."):
                head, lines = [l.rstrip() for l in lines[:k + 1]], lines[k + 1:]
                break
    body = format_lines(lines, width, reflow)
    out = head + ([""] if head and body else []) + body
    return "\n".join(out) + "\n" if out else ""


def find_runts(text, width):
    """Lines that look like leftovers of an old wrap: short, no closing punctuation, and the next line carries on
    in lowercase. Returns [(line_number, line)]. Skips code, front matter, tables and headings."""
    lines = text.split("\n")
    found, in_fence, i0 = [], False, 0
    if lines and lines[0].strip() == "---":
        for k in range(1, len(lines)):
            if lines[k].strip() in ("---", "..."):
                i0 = k + 1
                break
    for i in range(i0, len(lines) - 1):
        line, nxt = lines[i], lines[i + 1].strip()
        if FENCE_RE.match(line):
            in_fence = not in_fence
            continue
        s = line.strip()
        if in_fence or not s or not nxt or HEADING_RE.match(line) or s.startswith(("|", ">")):
            continue
        if width_of(s) < 0.6 * width and s[-1] not in ".!?:;" and nxt[0].islower() and not line.endswith("  "):
            found.append((i + 1, s))
    return found


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("files", nargs="+")
    ap.add_argument("--width", type=int, default=120)
    ap.add_argument("--check", action="store_true")
    ap.add_argument("--reflow", action="store_true",
                    help="also join the lines of each paragraph and re-wrap them (review the diff afterwards)")
    ap.add_argument("--runts", action="store_true",
                    help="only report runt lines (short leftovers of an old wrap), as 'runt: FILE:LINE: text'; "
                         "changes nothing")
    args = ap.parse_args()
    dirty = failed = False
    for path in args.files:
        try:
            with open(path, "rb") as f:
                old = f.read()
            text, _, _ = decode(old)
        except (OSError, UnicodeDecodeError) as e:
            print(f"skipped {path}: {e}", file=sys.stderr)
            failed = True
            continue
        if args.runts:
            for no, l in find_runts(format_text(text, args.width), args.width):
                print(f"runt: {path}:{no}: {l}")
            continue
        new = format_bytes(old, args.width, args.reflow)
        if new != old:
            dirty = True
            if args.check:
                print(f"would reformat {path}")
            else:
                with open(path, "wb") as f:  # binary: no newline translation on any platform
                    f.write(new)
                print(f"reformatted {path}")
        for no, l in enumerate(decode(new)[0].split("\n"), 1):
            if width_of(l) > args.width and not FENCE_RE.match(l):
                kind = "table row" if l.lstrip().startswith("|") else "line"
                print(f"  {path}:{no}: {kind} is {width_of(l)} wide (cannot be wrapped)", file=sys.stderr)
    if failed:
        return 2
    return 1 if (dirty and args.check) else 0


if __name__ == "__main__":
    sys.exit(main())
