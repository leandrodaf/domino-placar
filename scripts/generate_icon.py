#!/usr/bin/env python3
"""
Generate a polished 512×512 Play Store icon for Dominó Placar.

Draws a clean domino tile (4|1) from scratch using Pillow,
matching the app's dark theme.
"""

import os
from pathlib import Path
from PIL import Image, ImageDraw, ImageFilter

OUT = Path("play-store/app-icon-512.png")

# ── Colors (dark theme matching the app) ──
BG = (9, 9, 15)
TILE_FACE = (30, 30, 52)
TILE_EDGE = (55, 55, 88)
DOT_WHITE = (235, 235, 250)
DIVIDER_C = (65, 65, 100)
ACCENT = (233, 69, 96)

SIZE = 512


def generate():
    canvas = Image.new("RGB", (SIZE, SIZE), BG)
    draw = ImageDraw.Draw(canvas)

    # ── Tile dimensions (portrait, centered) ──
    tile_w, tile_h = 300, 410
    cr = 30
    tx = (SIZE - tile_w) // 2
    ty = (SIZE - tile_h) // 2

    # ── Drop shadow ──
    shadow_img = Image.new("RGBA", (SIZE, SIZE), (0, 0, 0, 0))
    sd = ImageDraw.Draw(shadow_img)
    sd.rounded_rectangle(
        [tx + 6, ty + 8, tx + tile_w + 6, ty + tile_h + 8],
        radius=cr, fill=(0, 0, 0, 100))
    shadow_img = shadow_img.filter(ImageFilter.GaussianBlur(18))
    # Composite shadow onto canvas
    canvas = Image.composite(
        Image.merge("RGB", shadow_img.split()[:3]),
        canvas,
        shadow_img.split()[3])
    draw = ImageDraw.Draw(canvas)

    # ── Tile border ──
    draw.rounded_rectangle(
        [tx - 2, ty - 2, tx + tile_w + 2, ty + tile_h + 2],
        radius=cr + 2, fill=TILE_EDGE)

    # ── Tile face ──
    draw.rounded_rectangle(
        [tx, ty, tx + tile_w, ty + tile_h],
        radius=cr, fill=TILE_FACE)

    # ── Divider line ──
    mid_y = ty + tile_h // 2
    margin = 28
    draw.line([(tx + margin, mid_y), (tx + tile_w - margin, mid_y)],
              fill=DIVIDER_C, width=3)

    # ── Dot positions ──
    cx = tx + tile_w // 2
    dot_r = 20

    # Top half: 4 dots
    top_mid = ty + tile_h // 4
    dx, dy = 70, 62
    top_dots = [
        (cx - dx, top_mid - dy),
        (cx + dx, top_mid - dy),
        (cx - dx, top_mid + dy),
        (cx + dx, top_mid + dy),
    ]
    # Bottom half: 1 dot
    bot_mid = ty + 3 * tile_h // 4
    bot_dots = [(cx, bot_mid)]

    for (dcx, dcy) in top_dots + bot_dots:
        # Subtle glow
        draw.ellipse([dcx - dot_r - 6, dcy - dot_r - 6,
                       dcx + dot_r + 6, dcy + dot_r + 6],
                      fill=(60, 60, 90))
        # Main dot
        draw.ellipse([dcx - dot_r, dcy - dot_r,
                       dcx + dot_r, dcy + dot_r],
                      fill=DOT_WHITE)

    # ── Accent glow at bottom edge ──
    accent_layer = Image.new("RGBA", (SIZE, SIZE), (0, 0, 0, 0))
    ImageDraw.Draw(accent_layer).rounded_rectangle(
        [tx, ty + tile_h - 8, tx + tile_w, ty + tile_h + 4],
        radius=4, fill=(*ACCENT, 35))
    accent_layer = accent_layer.filter(ImageFilter.GaussianBlur(10))
    canvas = Image.alpha_composite(canvas.convert("RGBA"), accent_layer).convert("RGB")

    # Save
    OUT.parent.mkdir(parents=True, exist_ok=True)
    canvas.save(str(OUT), "PNG", optimize=True)
    print(f"✅ {OUT} ({OUT.stat().st_size // 1024}KB)")


if __name__ == "__main__":
    os.chdir(Path(__file__).resolve().parent.parent)
    generate()
