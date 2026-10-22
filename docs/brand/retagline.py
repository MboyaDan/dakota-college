"""Derive "Turning Potential into Progress" tagline logos from the supplied
artwork, which reads "...into Progression" (owner, 2026-10-22; D-86).

Nothing is redrawn. The supplied tagline is one path made of letter outlines;
this script drops the outlines of the final "ion" and moves the shortened line
so it stays centred on the gold rule. Every other element is copied unchanged.

    python3 docs/brand/retagline.py

Reads   docs/brand/dakota-logo/dakota-logo-{stacked,horizontal}-tagline.svg
Writes  docs/brand/dakota-logo/dakota-logo-{stacked,horizontal}-tagline-progress.svg
"""

import re
from pathlib import Path

BRAND = Path(__file__).parent / "dakota-logo"
SOURCES = ["dakota-logo-stacked-tagline.svg", "dakota-logo-horizontal-tagline.svg"]


def outline_points(subpath: str) -> list[tuple[float, float]]:
    """End and control points of a subpath that uses absolute M, L, H, Q, Z."""
    tokens = re.findall(r"[A-Za-z]|-?\d+(?:\.\d+)?", subpath)
    points, cur, cmd, i = [], (0.0, 0.0), None, 0
    while i < len(tokens):
        if tokens[i].isalpha():
            cmd = tokens[i]
            i += 1
            if cmd not in "MLHQZ":
                raise ValueError(f"unexpected path command {cmd}")
            if cmd == "Z":
                continue
        if cmd in ("M", "L"):
            cur = (float(tokens[i]), float(tokens[i + 1]))
            points.append(cur)
            i += 2
        elif cmd == "H":
            cur = (float(tokens[i]), cur[1])
            points.append(cur)
            i += 1
        elif cmd == "Q":
            control = (float(tokens[i]), float(tokens[i + 1]))
            cur = (float(tokens[i + 2]), float(tokens[i + 3]))
            points += [control, cur]
            i += 4
    return points


def letter_groups(d: str) -> list[dict]:
    """Outlines grouped left to right; outlines whose x ranges overlap (a letter
    and its counter or dot, or touching italic letters) share a group."""
    outlines = []
    for sp in (p for p in re.split(r"(?=M)", d) if p.strip()):
        xs = [x for x, _ in outline_points(sp)]
        outlines.append({"x0": min(xs), "x1": max(xs), "paths": [sp]})
    outlines.sort(key=lambda o: o["x0"])
    groups: list[dict] = []
    for o in outlines:
        if groups and o["x0"] < groups[-1]["x1"] - 1:
            groups[-1]["x1"] = max(groups[-1]["x1"], o["x1"])
            groups[-1]["paths"] += o["paths"]
        else:
            groups.append(o)
    return groups


def retagline(svg: str) -> str:
    tag = re.search(r'<path id="tagline"[^>]*/>', svg)
    rule = re.search(r'<path id="rule"[^>]* d="M([\d.]+) [\d.]+H([\d.]+)', svg)
    if not tag or not rule:
        raise ValueError("tagline or rule path not found")
    d = re.search(r' d="([^"]*)"', tag.group(0)).group(1)
    groups = letter_groups(d)
    # Checked by eye against the artwork (2026-10-23): 18 groups, the last
    # two are "io" (stem, dot, bowl, counter) and "n".
    if len(groups) != 18 or len(groups[-2]["paths"]) != 4 or len(groups[-1]["paths"]) != 1:
        raise ValueError("the tagline artwork is not the expected one; check before changing it")
    kept = groups[:-2]
    centre = (float(rule.group(1)) + float(rule.group(2))) / 2
    shift = centre - (kept[0]["x0"] + kept[-1]["x1"]) / 2
    new_d = "".join(p for g in kept for p in g["paths"])
    new_tag = tag.group(0).replace(d, new_d).replace("/>", f' transform="translate({shift:.1f} 0)"/>')
    return svg.replace(tag.group(0), new_tag)


if __name__ == "__main__":
    for name in SOURCES:
        out = BRAND / name.replace("-tagline.svg", "-tagline-progress.svg")
        out.write_text(retagline((BRAND / name).read_text()))
        print("wrote", out.relative_to(Path.cwd()) if out.is_relative_to(Path.cwd()) else out)
