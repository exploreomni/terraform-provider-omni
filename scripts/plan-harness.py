#!/usr/bin/env python3
"""Evaluate this configuration's expressions without the Omni API.

Why this exists
---------------
`terraform validate` does not evaluate variable validations, and it does not
evaluate module expressions against real values either. Both gaps have shipped
bugs from this repository: a `coalesce(x, "")` that errors on an empty string,
and a validation that passed `validate` and failed `plan`. A real `plan` would
catch them, but planning needs the provider, and the provider needs the API.

So this rewrites the configuration into one that has no providers in it, and
plans that instead. Every `resource` and `data` block becomes a local holding
its evaluated body, keyed the same way the resource is. Anything that reads
`omni_user_group.this["analysts"].id` reads a stub object instead. The
expressions - the `for_each` filters, the merges, the coalesces, the
validations - are the real ones, evaluated against real tfvars.

What it proves
--------------
That the configuration evaluates, that validations fire when they should, and
that the keys and references line up. It is a type and value check.

What it does not prove
----------------------
Anything the API decides. A plan here will happily "create" something that
already exists; only an apply knows. Nested blocks inside a resource body are
dropped, so an expression inside one is not checked. And the stub IDs are not
the real ones, so a reference that resolves here can still be the wrong handle
at apply time.

Usage
-----
    scripts/plan-harness.py                 # expect success
    scripts/plan-harness.py --expect-error 'not in var.groups'
"""

from __future__ import annotations

import argparse
import os
import re
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path

REPO = Path(__file__).resolve().parent.parent

# Computed attributes the configuration reads back off a resource or data
# source. A body that sets one of these overrides the stub.
STUBS = {
    "id": '"stub-id"',
    "name": '"stub-name"',
    "path": '"stub/path"',
}


def strip_blocks(text: str, *heads: str) -> str:
    """Remove top-level blocks whose header starts with any of heads."""
    out, i = [], 0
    while i < len(text):
        line_start = text.rfind("\n", 0, i) + 1
        if text[i] == "{" and any(
            text[line_start:i].lstrip().startswith(h) for h in heads
        ):
            depth, j = 1, i + 1
            while j < len(text) and depth:
                depth += {"{": 1, "}": -1}.get(text[j], 0)
                j += 1
            # Drop the header line too.
            while out and out[-1] != "\n":
                out.pop()
            i = j
            continue
        out.append(text[i])
        i += 1
    return "".join(out)


BLOCK_RE = re.compile(
    r'^(?P<kind>resource|data)\s+"(?P<type>omni_[a-z0-9_]+)"\s+"(?P<name>[a-z0-9_]+)"\s*\{',
    re.MULTILINE,
)


def split_body(body: str) -> tuple[dict[str, str], str | None, str | None]:
    """Top-level `attr = expr` assignments, plus for_each and count."""
    attrs: dict[str, str] = {}
    for_each = count = None

    i, depth = 0, 0
    while i < len(body):
        ch = body[i]
        if ch in "{[(":
            depth += 1
        elif ch in "}])":
            depth -= 1
        elif depth == 0:
            m = re.match(r"(?m)^[ \t]*([a-z0-9_]+)[ \t]*=[ \t]*", body[i:])
            if m and (i == 0 or body[i - 1] == "\n"):
                key = m.group(1)
                j = i + m.end()
                # Read the expression: to the end of the line, unless brackets
                # are still open.
                d, k = 0, j
                while k < len(body):
                    if body[k] in "{[(":
                        d += 1
                    elif body[k] in "}])":
                        d -= 1
                    elif body[k] == "\n" and d <= 0:
                        break
                    k += 1
                expr = body[j:k].strip()
                if key == "for_each":
                    for_each = expr
                elif key == "count":
                    count = expr
                else:
                    attrs[key] = expr
                i = k
                continue
        i += 1
    return attrs, for_each, count


def transform(text: str) -> tuple[str, list[str]]:
    """Rewrite one .tf file. Returns the new text and the locals it defined.

    References are not rewritten here. A module's outputs.tf reads resources
    declared in its main.tf, so that has to happen once per directory, after
    every file in it has been read.
    """
    text = strip_blocks(text, "terraform", "provider ", "moved", "import")

    defined: list[str] = []
    out, pos = [], 0
    for m in BLOCK_RE.finditer(text):
        out.append(text[pos : m.start()])
        depth, j = 1, m.end()
        while j < len(text) and depth:
            depth += {"{": 1, "}": -1}.get(text[j], 0)
            j += 1
        body = text[m.end() : j - 1]
        pos = j

        attrs, for_each, count = split_body(body)
        prefix = "__data_" if m.group("kind") == "data" else "__res_"
        local = f'{prefix}{m.group("type")}__{m.group("name")}'
        defined.append(local)

        stub = ",\n      ".join(
            f"{k} = {v}" for k, v in STUBS.items() if k not in attrs
        )
        set_attrs = ",\n      ".join(f"{k} = {v}" for k, v in attrs.items())
        obj = "{\n      " + ",\n      ".join(x for x in (stub, set_attrs) if x) + "\n    }"

        if for_each is not None:
            out.append(
                f"locals {{\n  {local} = {{\n"
                f"    for __k, __v in {for_each} : __k => {obj}\n  }}\n}}\n"
            )
            # `each` inside the body: bind it to the iteration.
            out[-1] = out[-1].replace("each.key", "__k").replace("each.value", "__v")
        elif count is not None:
            out.append(
                f"locals {{\n  {local} = [\n"
                f"    for __i in range({count}) : {obj}\n  ]\n}}\n"
            )
            out[-1] = out[-1].replace("count.index", "__i")
        else:
            out.append(f"locals {{\n  {local} = {obj}\n}}\n")

    out.append(text[pos:])
    return "".join(out), defined


def retarget(text: str, locals_: list[str]) -> str:
    """Point references at the generated locals, across a whole directory."""
    for local in locals_:
        if local.startswith("__data_"):
            ref = "data." + local[len("__data_") :].replace("__", ".", 1)
        else:
            ref = local[len("__res_") :].replace("__", ".", 1)
        text = re.sub(rf"(?<![\w.]){re.escape(ref)}\b", f"local.{local}", text)
    return text


def build(dest: Path) -> None:
    for sub in ("infra", "modules"):
        shutil.copytree(REPO / sub, dest / sub)

    # Pass one: rewrite the blocks, collecting what each directory declared.
    by_dir: dict[Path, list[str]] = {}
    for tf in sorted(dest.rglob("*.tf")):
        text, defined = transform(tf.read_text())
        tf.write_text(text)
        by_dir.setdefault(tf.parent, []).extend(defined)

    # Pass two: retarget references, now that every file in the directory has
    # been seen, and force evaluation. Terraform evaluates lazily, so an
    # unreferenced local is never computed and its expressions never checked.
    for directory, defined in by_dir.items():
        for tf in sorted(directory.glob("*.tf")):
            tf.write_text(retarget(tf.read_text(), defined))
        if defined:
            forced = "\n".join(
                f'output "__force_{i}" {{\n  value = local.{l}\n  sensitive = true\n}}'
                for i, l in enumerate(defined)
            )
            (directory / "__force.tf").write_text(forced + "\n")

    # And force every root variable, so its validations run even if nothing
    # downstream happens to read it.
    root = dest / "infra"
    names = re.findall(r'^variable "([a-z0-9_]+)"', (root / "variables.tf").read_text(), re.M)
    (root / "__force_vars.tf").write_text(
        "\n".join(
            f'output "__force_var_{n}" {{\n  value = var.{n}\n  sensitive = true\n}}'
            for n in names
        )
        + "\n"
    )


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--expect-error", metavar="SUBSTRING")
    ap.add_argument("--var-file", action="append", default=[])
    ap.add_argument("--keep", action="store_true", help="keep the rewritten copy")
    args = ap.parse_args()

    tofu = shutil.which("tofu") or shutil.which("terraform") or "/tmp/tofubin/tofu"
    work = Path(tempfile.mkdtemp(prefix="plan-harness-"))
    try:
        build(work)
        root = work / "infra"
        for extra in args.var_file:
            shutil.copy(extra, root / f"zz-override-{Path(extra).name}")

        env = {**os.environ, "TF_IN_AUTOMATION": "1"}
        for step in (["init", "-backend=false", "-input=false"], ["plan", "-input=false"]):
            r = subprocess.run(
                [tofu, *step], cwd=root, env=env, capture_output=True, text=True
            )
            if r.returncode:
                combined = r.stdout + r.stderr
                if args.expect_error:
                    if args.expect_error in combined:
                        print(f"expected error seen: {args.expect_error!r}")
                        return 0
                    print(
                        f"FAIL: expected {args.expect_error!r}, got:\n{combined}",
                        file=sys.stderr,
                    )
                    return 1
                print(combined, file=sys.stderr)
                return 1

        if args.expect_error:
            print(f"FAIL: expected {args.expect_error!r}, but it planned", file=sys.stderr)
            return 1
        print("plan ok")
        return 0
    finally:
        if args.keep:
            print(f"rewritten copy: {work}")
        else:
            shutil.rmtree(work, ignore_errors=True)


if __name__ == "__main__":
    sys.exit(main())
