#!/usr/bin/env python3
"""PreToolUse/Bash hook: refuse git commands that stage or commit the whole working tree.

Several agent sessions share this tree, so a broad `git add -A` or `git commit -a` sweeps up other sessions'
uncommitted work into the wrong commit. Blocked: `git add` with -A/--all/-u/--update or a bare `.`, `*`, `:/`;
`git commit` with -a/--all (including clusters such as -am).

A guardrail against accidents, not a security boundary. Fails open: any parsing problem allows the command.
To run one on purpose, type it yourself with a leading `!` in the prompt.
"""
import json
import re
import shlex
import sys

GLOBAL_OPTS_WITH_ARG = {"-C", "-c", "--git-dir", "--work-tree", "--namespace", "--exec-path"}
COMMIT_OPTS_WITH_ARG = {"-m", "-F", "-C", "-c", "-t", "--message", "--file", "--author", "--date", "--template",
                        "--reuse-message", "--reedit-message", "--fixup", "--squash", "-S", "--cleanup"}
WRAPPERS = {"env", "command", "sudo", "nohup", "time", "exec", "nice"}
BROAD_PATHS = {".", "./", "*", ":/", ":(top)", ".."}


def git_args(tokens):
    """Return (subcommand, args) if the tokens are a git invocation, else None."""
    i = 0
    while i < len(tokens) and (tokens[i] in WRAPPERS or re.match(r"^[A-Za-z_][A-Za-z0-9_]*=", tokens[i])):
        i += 1
    if i >= len(tokens) or tokens[i].rsplit("/", 1)[-1] != "git":
        return None
    i += 1
    while i < len(tokens) and tokens[i].startswith("-"):
        i += 2 if tokens[i] in GLOBAL_OPTS_WITH_ARG else 1
    if i >= len(tokens):
        return None
    return tokens[i], tokens[i + 1:]


def check_add(args):
    for a in args:
        if a == "--":
            continue
        if a in ("--all", "--update") or a in BROAD_PATHS:
            return f"`git add {a}`"
        if re.match(r"^-[A-Za-z]+$", a) and ("A" in a or "u" in a):
            return f"`git add {a}`"
    return None


def check_commit(args):
    skip = False
    for a in args:
        if skip:
            skip = False
            continue
        if a in COMMIT_OPTS_WITH_ARG:
            skip = True
        elif a == "--all" or (re.match(r"^-[A-Za-z]+$", a) and "a" in a):
            return f"`git commit {a}`"
    return None


def offending(cmd):
    for segment in re.split(r"&&|\|\||[;|\n]", cmd):
        try:
            tokens = shlex.split(segment, posix=True)
        except ValueError:
            tokens = segment.split()
        found = git_args(tokens)
        if not found:
            continue
        sub, args = found
        hit = check_add(args) if sub == "add" else check_commit(args) if sub == "commit" else None
        if hit:
            return hit
    return None


def main():
    try:
        cmd = json.load(sys.stdin).get("tool_input", {}).get("command", "")
        hit = offending(cmd) if cmd else None
    except Exception:
        return 0
    if hit:
        print(json.dumps({"hookSpecificOutput": {
            "hookEventName": "PreToolUse",
            "permissionDecision": "deny",
            "permissionDecisionReason": (
                f"{hit} is blocked: several agent sessions share this working tree and a broad stage or commit "
                "sweeps their uncommitted work into your commit. Stage only the paths you changed "
                "(`git add <paths>`), check `git status --short` and `git diff --cached --stat`, then "
                "`git commit <paths>`. If you really need it, the user can type the command with a leading !."),
        }}))
    return 0


if __name__ == "__main__":
    sys.exit(main())
