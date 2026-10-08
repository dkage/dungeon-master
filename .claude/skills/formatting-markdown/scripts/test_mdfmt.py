#!/usr/bin/env python3
"""Tests for mdfmt.py. Run: python3 -m unittest -v test_mdfmt   (from this directory)

Pure functions only: nothing here reads or writes a real file.
"""
import os
import subprocess
import sys
import tempfile
import unittest

from mdfmt import decode, find_runts, format_bytes, format_text, split_row, width_of

SCRIPT = os.path.join(os.path.dirname(os.path.abspath(__file__)), "mdfmt.py")

W = 120


def put(path, data):
    with open(path, "wb") as f:
        f.write(data)


def get(path):
    with open(path, "rb") as f:
        return f.read()

LONG = ("word " * 40).strip()  # 199 columns, wraps at 120


def fmt(text, width=W, reflow=False):
    return format_text(text, width, reflow)


class Wrapping(unittest.TestCase):
    def test_long_paragraph_wraps_at_width(self):
        out = fmt(LONG + "\n")
        self.assertTrue(all(width_of(l) <= W for l in out.split("\n")))
        self.assertEqual(" ".join(out.split()), LONG)

    def test_list_item_gets_hanging_indent(self):
        out = fmt("- " + LONG + "\n").split("\n")
        self.assertTrue(out[0].startswith("- "))
        self.assertTrue(out[1].startswith("  word"))

    def test_ordered_item_hangs_under_text(self):
        out = fmt("1. " + LONG + "\n10. " + LONG + "\n").split("\n")
        self.assertTrue(out[1].startswith("   word"))  # width of "1. "
        ten = next(i for i, l in enumerate(out) if l.startswith("10. "))
        self.assertTrue(out[ten + 1].startswith("    word"))  # width of "10. "

    def test_nested_list_keeps_structure(self):
        out = fmt("- a\n  - b " + LONG + "\n")
        self.assertIn("  - b ", out)
        self.assertTrue(all(width_of(l) <= W for l in out.split("\n")))

    def test_block_quote_wraps_with_prefix(self):
        out = fmt("> " + LONG + "\n").rstrip("\n").split("\n")
        self.assertTrue(all(l.startswith("> ") for l in out))
        self.assertGreater(len(out), 1)

    def test_code_span_is_never_split(self):
        text = "x " * 55 + "`a b c d e f` tail\n"
        for line in fmt(text).split("\n"):
            self.assertEqual(line.count("`") % 2, 0, line)

    def test_unbreakable_token_is_left_alone(self):
        url = "https://example.com/" + "a" * 150
        self.assertIn(url, fmt("see " + url + "\n"))

    def test_wrap_never_starts_a_line_with_a_block_marker(self):
        text = "x" * 117 + " - not a list\n"
        lines = fmt(text).rstrip("\n").split("\n")
        self.assertFalse(any(l.startswith("- ") for l in lines), lines)

    def test_custom_width(self):
        out = fmt(LONG + "\n", width=60)
        self.assertTrue(all(width_of(l) <= 60 for l in out.split("\n")))

    def test_cjk_counts_double_width(self):
        self.assertEqual(width_of("漢字"), 4)
        self.assertEqual(width_of("é"), 1)


class Blocks(unittest.TestCase):
    def test_blank_line_after_heading_and_none_doubled(self):
        self.assertEqual(fmt("# T\ntext\n\n\n\nmore\n"), "# T\n\ntext\n\nmore\n")

    def test_blank_line_before_heading(self):
        self.assertEqual(fmt("text\n## H\nbody\n"), "text\n\n## H\n\nbody\n")

    def test_single_trailing_newline_and_no_trailing_spaces(self):
        self.assertEqual(fmt("a   \n\n\n"), "a\n")
        self.assertEqual(fmt("a  \nb  \n"), "a  \nb\n")  # only a hard break before another line survives

    def test_fenced_code_untouched(self):
        code = "```go\n" + "x := 1 " * 40 + "\n\n\n| a | b |\n```\n"
        self.assertIn(code, fmt("# T\n" + code))

    def test_tilde_fence_untouched(self):
        code = "~~~\n" + "y " * 100 + "\n~~~\n"
        self.assertIn(code, fmt(code))

    def test_front_matter_untouched(self):
        fm = "---\nname: x\ndescription: " + LONG + "\n---\n"
        out = fmt(fm + "# T\ntext\n")
        self.assertTrue(out.startswith(fm))
        self.assertIn("\n# T\n\ntext\n", out)

    def test_html_block_untouched(self):
        html = "<details>\n<summary>" + LONG + "</summary>\n</details>\n"
        self.assertIn(html, fmt(html))

    def test_link_reference_untouched(self):
        ref = "[id]: https://example.com/" + "a" * 150 + " \"title\"\n"
        self.assertIn(ref, fmt(ref))

    def test_horizontal_rule_and_list_marker_not_confused(self):
        out = fmt("para\n\n---\n\n- item\n")
        self.assertIn("\n---\n", out)
        self.assertIn("- item", out)

    def test_hard_break_is_kept(self):
        out = fmt("line one  \nline two\n")
        self.assertEqual(out, "line one  \nline two\n")

    def test_indented_code_untouched(self):
        self.assertIn("\n    code   here\n", fmt("text\n\n    code   here\n"))


class Tables(unittest.TestCase):
    def test_aligned_with_full_width_separator(self):
        out = fmt("|a|bb|\n|-|-|\n|long cell|1|\n")
        self.assertEqual(out, "| a         | bb |\n|-----------|----|\n| long cell | 1  |\n")

    def test_alignment_markers_are_kept(self):
        out = fmt("|left|right|mid|plain|\n|:--|--:|:-:|---|\n|1|2|3|4|\n").split("\n")
        self.assertEqual(out[1], "|:-----|------:|:---:|-------|")
        self.assertEqual(out[2], "| 1    |     2 |  3  | 4     |")

    def test_short_row_is_padded_with_empty_cells(self):
        out = fmt("|a|b|\n|-|-|\n|only|\n")
        self.assertIn("| only |   |", out)

    def test_escaped_pipe_stays_inside_the_cell(self):
        self.assertEqual(split_row("| a \\| b | c |"), ["a \\| b", "c"])

    def test_cjk_cells_align_by_display_width(self):
        lines = fmt("|a|b|\n|-|-|\n|漢字|x|\n|abcd|y|\n").rstrip("\n").split("\n")
        self.assertEqual(len({width_of(l) for l in lines}), 1)

    def test_blank_line_around_table(self):
        out = fmt("text\n|a|\n|-|\n|1|\nafter\n")
        self.assertEqual(out, "text\n\n| a |\n|---|\n| 1 |\n\nafter\n")

    def test_table_inside_list_item(self):
        out = fmt("- item\n\n  |a|b|\n  |-|-|\n  |1|2|\n")
        self.assertIn("  | a | b |\n  |---|---|\n  | 1 | 2 |", out)

    def test_pipe_in_prose_is_not_a_table(self):
        self.assertEqual(fmt("a | b\nc\n"), "a | b\nc\n")


class JoiningLines(unittest.TestCase):
    """Default: never join lines the author kept apart. --reflow: join and re-wrap."""

    PSEUDO = "**M0 - Skeleton**\nReplace the hello-world.\n"
    RUNT = "is done except the deferred SQLite\ncopy\nand write queue (phases C and D).\n"

    def test_default_keeps_separate_lines(self):
        self.assertEqual(fmt(self.PSEUDO), self.PSEUDO)
        self.assertEqual(fmt(self.RUNT), self.RUNT)

    def test_reflow_joins_paragraph_lines(self):
        self.assertEqual(fmt(self.RUNT, reflow=True),
                         "is done except the deferred SQLite copy and write queue (phases C and D).\n")

    def test_reflow_rewraps_at_width(self):
        out = fmt("\n".join(["word " * 10] * 6) + "\n", reflow=True)
        self.assertTrue(all(width_of(l) <= W for l in out.split("\n")))
        self.assertEqual(len(out.rstrip("\n").split("\n")), 3)

    def test_reflow_respects_hard_breaks(self):
        self.assertEqual(fmt("a\nb  \nc\nd\n", reflow=True), "a b  \nc d\n")

    def test_reflow_joins_list_item_continuations(self):
        self.assertEqual(fmt("- one\n  two\n  three\n- four\n", reflow=True), "- one two three\n- four\n")

    def test_reflow_does_not_touch_code_tables_or_headings(self):
        text = "# H\n\n```\na\nb\n```\n\n| a |\n|---|\n| 1 |\n"
        self.assertEqual(fmt(text, reflow=True), text)


class Invariants(unittest.TestCase):
    SAMPLES = [
        "# T\ntext " + LONG + "\n- a " + LONG + "\n  - b\n1. x\n2. y\n> q " + LONG + "\n|a|b|\n|-|-|\n|1|2|\n",
        "---\nk: v\n---\n# T\n```\ncode\n```\n",
        "para one  \nline two\n\n\n\npara " + LONG + "\n",
        "- [ ] task " + LONG + "\n- [x] done\n",
        "",
        "\n\n",
    ]

    def test_idempotent(self):
        for s in self.SAMPLES:
            for reflow in (False, True):
                once = fmt(s, reflow=reflow)
                self.assertEqual(fmt(once, reflow=reflow), once, s)

    def test_no_words_lost_or_reordered(self):
        for s in self.SAMPLES:
            strip = lambda t: [w for w in t.replace("|", " ").split() if not set(w) <= set("-:>")]
            self.assertEqual(strip(fmt(s)), strip(s), s)

    def test_empty_input_stays_empty(self):
        self.assertEqual(fmt(""), "")


class Runts(unittest.TestCase):
    def test_flags_the_leftover_short_line(self):
        first = "Status: M3.5 is done except the " + "deferred " * 8 + "SQLite"  # a full-width line, as after a wrap
        text = first + "\ncopy\nand write queue (phases C and D). Plenty more text follows here to make it long.\n"
        self.assertEqual([(l, t) for l, t in find_runts(text, W)], [(2, "copy")])

    def test_does_not_flag_a_pseudo_heading(self):
        self.assertEqual(find_runts("**M0 - Skeleton**\nReplace the hello-world.\n", W), [])

    def test_does_not_flag_a_short_sentence(self):
        self.assertEqual(find_runts("Short sentence.\nand lowercase after.\n", W), [])

    def test_does_not_flag_a_hard_break(self):
        self.assertEqual(find_runts("short line  \nlowercase next\n", W), [])

    def test_ignores_code_front_matter_tables_and_headings(self):
        text = "---\nname: x\ndescription: y\n---\n# head\nlowercase\n```\nshort\nlower\n```\n| a |\n|---|\n"
        self.assertEqual(find_runts(text, W), [])

    def test_reflow_removes_the_runt(self):
        text = "is done except the deferred SQLite\ncopy\nand write queue (phases C and D).\n"
        self.assertEqual(find_runts(fmt(text, reflow=True), W), [])


class LineEndings(unittest.TestCase):
    """Files from Windows (CRLF), classic Mac (CR), mixed files, BOMs: same result, original style kept."""

    DOC = "# T\ntext " + LONG + "\n- a\n- b\n|a|b|\n|-|-|\n|1|2|\n```\ncode  \n```\nend\n"

    def fmt_b(self, text, eol="\n", bom=False, reflow=False):
        data = text.replace("\n", eol).encode("utf-8")
        return format_bytes((b"\xef\xbb\xbf" if bom else b"") + data, W, reflow)

    def test_crlf_in_crlf_out_same_content_as_lf(self):
        lf = self.fmt_b(self.DOC)
        crlf = self.fmt_b(self.DOC, "\r\n")
        self.assertEqual(crlf, lf.replace(b"\n", b"\r\n"))

    def test_crlf_output_has_no_bare_lf_or_cr(self):
        out = self.fmt_b(self.DOC, "\r\n")
        self.assertEqual(out.count(b"\n"), out.count(b"\r\n"))
        self.assertEqual(out.count(b"\r"), out.count(b"\r\n"))

    def test_lf_stays_lf(self):
        self.assertNotIn(b"\r", self.fmt_b(self.DOC))

    def test_classic_mac_cr_stays_cr(self):
        out = self.fmt_b(self.DOC, "\r")
        self.assertNotIn(b"\n", out)
        self.assertEqual(out.replace(b"\r", b"\n"), self.fmt_b(self.DOC))

    def test_crlf_does_not_create_hard_breaks_or_stray_cr(self):
        out, eol, _ = decode(self.fmt_b("a\nb\n", "\r\n"))
        self.assertEqual((out, eol), ("a\nb\n", "\r\n"))

    def test_mixed_endings_go_to_the_most_common(self):
        data = b"# T\r\n\r\ntext\r\nmore\nlast\r\n"
        out = format_bytes(data, W)
        self.assertEqual(out.count(b"\n"), out.count(b"\r\n"))
        data = b"# T\n\ntext\nmore\r\nlast\n"
        self.assertNotIn(b"\r", format_bytes(data, W))

    def test_bom_is_kept_and_front_matter_still_recognised(self):
        doc = "---\nname: x\ndescription: " + LONG + "\n---\n# T\ntext\n"
        out = self.fmt_b(doc, bom=True)
        self.assertTrue(out.startswith(b"\xef\xbb\xbf---\n"))
        self.assertIn(b"description: " + LONG.encode(), out)  # front matter not wrapped
        self.assertIn(b"\n# T\n\ntext\n", out)

    def test_bom_with_crlf(self):
        out = self.fmt_b("# T\ntext\n", "\r\n", bom=True)
        self.assertEqual(out, b"\xef\xbb\xbf# T\r\n\r\ntext\r\n")

    def test_missing_final_newline_gets_one_in_the_files_style(self):
        self.assertEqual(format_bytes(b"a\r\nb", W), b"a\r\nb\r\n")
        self.assertEqual(format_bytes(b"a\nb", W), b"a\nb\n")

    def test_idempotent_in_every_style(self):
        for eol in ("\n", "\r\n", "\r"):
            for bom in (False, True):
                once = self.fmt_b(self.DOC, eol, bom)
                self.assertEqual(format_bytes(once, W), once)

    def test_crlf_code_block_stays_crlf_and_untouched(self):
        out = self.fmt_b("```\n" + "x " * 100 + "\n\n\n```\n", "\r\n")
        self.assertEqual(out, ("```\n" + "x " * 100 + "\n\n\n```\n").replace("\n", "\r\n").encode())

    def test_whitespace_only_lines_count_as_blank(self):
        self.assertEqual(fmt("a\n \t \n\t\nb\n"), "a\n\nb\n")

    def test_runts_work_on_crlf_text(self):
        text, _, _ = decode(("is " + "x " * 55 + "end\r\ncopy\r\nand more\r\n").encode())
        self.assertEqual([t for _, t in find_runts(text, W)], ["copy"])

    def test_non_breaking_space_is_not_a_break_point(self):
        nbsp = "a\u00a0b"
        self.assertIn(nbsp, fmt("x " * 58 + nbsp + " tail\n"))

    def test_invalid_utf8_is_rejected_not_mangled(self):
        with self.assertRaises(UnicodeDecodeError):
            decode(b"caf\xe9\n")


class CommandLine(unittest.TestCase):
    """The real script on real (temporary) files: bytes in, bytes out."""

    def run_cli(self, *args):
        return subprocess.run([sys.executable, "-B", SCRIPT, *args], capture_output=True, text=True)

    def test_crlf_file_is_formatted_in_place_and_stays_crlf(self):
        with tempfile.TemporaryDirectory() as d:
            p = os.path.join(d, "a.md")
            put(p, b"# T\r\ntext\r\n|a|b|\r\n|-|-|\r\n|1|2|\r\n")
            self.assertEqual(self.run_cli(p).returncode, 0)
            self.assertEqual(get(p),
                             b"# T\r\n\r\ntext\r\n\r\n| a | b |\r\n|---|---|\r\n| 1 | 2 |\r\n")

    def test_lf_file_stays_lf_and_bom_survives(self):
        with tempfile.TemporaryDirectory() as d:
            p = os.path.join(d, "a.md")
            put(p, b"\xef\xbb\xbf# T\ntext\n")
            self.run_cli(p)
            self.assertEqual(get(p), b"\xef\xbb\xbf# T\n\ntext\n")

    def test_check_does_not_write_and_reports_with_exit_1(self):
        with tempfile.TemporaryDirectory() as d:
            p = os.path.join(d, "a.md")
            put(p, b"# T\r\ntext\r\n")
            r = self.run_cli("--check", p)
            self.assertEqual(r.returncode, 1)
            self.assertEqual(get(p), b"# T\r\ntext\r\n")

    def test_clean_crlf_file_is_not_touched_at_all(self):
        with tempfile.TemporaryDirectory() as d:
            p = os.path.join(d, "a.md")
            put(p, b"# T\r\n\r\ntext\r\n")
            before = os.stat(p).st_mtime_ns
            r = self.run_cli(p)
            self.assertEqual((r.returncode, r.stdout), (0, ""))
            self.assertEqual(os.stat(p).st_mtime_ns, before)

    def test_non_utf8_file_is_left_alone_with_exit_2(self):
        with tempfile.TemporaryDirectory() as d:
            p = os.path.join(d, "a.md")
            put(p, b"caf\xe9\n")
            r = self.run_cli(p)
            self.assertEqual(r.returncode, 2)
            self.assertEqual(get(p), b"caf\xe9\n")

    def test_missing_file_does_not_stop_the_others(self):
        with tempfile.TemporaryDirectory() as d:
            p = os.path.join(d, "a.md")
            put(p, b"# T\ntext\n")
            r = self.run_cli(os.path.join(d, "nope.md"), p)
            self.assertEqual(r.returncode, 2)
            self.assertEqual(get(p), b"# T\n\ntext\n")

    def test_runts_flag_reports_without_writing(self):
        with tempfile.TemporaryDirectory() as d:
            p = os.path.join(d, "a.md")
            body = ("is " + "x " * 55 + "end\r\ncopy\r\nand more\r\n").encode()
            put(p, body)
            r = self.run_cli("--runts", p)
            self.assertIn(":2: copy", r.stdout)
            self.assertEqual(get(p), body)


if __name__ == "__main__":
    unittest.main()
