#!/usr/bin/env python3
"""Tests for mdfmt.py. Run: python3 -m unittest -v test_mdfmt   (from this directory)

Pure functions only: nothing here reads or writes a real file.
"""
import unittest

from mdfmt import find_runts, format_text, split_row, width_of

W = 120
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


if __name__ == "__main__":
    unittest.main()
