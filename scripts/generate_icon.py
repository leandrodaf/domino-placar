#!/usr/bin/env python3
"""
Generate a polished 512×512 Play Store icon for Dominó Placar.

Draws a domino tile (4|1) matching the app's original dark
style: glowing edge, 3D dots with specular highlights, subtle
depth gradient on the tile face.
"""

import os
from pathlib import Path
from PIL import Image, ImageDraw, ImageFilter

OUT = Path("play-store/app-icon-512.png")
SIZE = 512

# ── Colors matching icone-domino-placar.png original ──
BG = (6, 6, 14)
TILE_DARK = (18, 18, 36)       # bottom of tile gradient
TILE_LIGHT = (28, 30, 55)      # top of tile gradient
EDGE_GLOW = (80, 80, 140)      # bluish edge glow
DOT_BASE = (220, 225, 245)     # main dot color
DOT_SHINE = (255, 255, 255)    # specular highlight
DOT_SHADOW = (12, 12, 28)      # inset shadow behind dot
DIVIDER_C = (50, 50, 85)
ACCENT = (233, 69, 96)


def _tile_gradient(w, h, top, bot, radius):
    """Create a tile-shaped image with vertical gradient."""
    img = Image.new("RGBA", (w, h), (0, 0, 0, 0))
    draw = ImageDraw.Draw(img)
    for y in range(h):
        t = y / max(h - 1, 1)
        c = tuple(int(top[i] * (1 - t) + bot[i] * t) for i in range(3))
        draw.line([(0, y), (w - 1, y)], fill=(*c, 255))
    # Mask to rounded rect
    mask = Image.new("L", (w, h), 0)
    ImageDraw.Draw(mask).rounded_rectangle([0, 0, w - 1, h - 1], radius=radius, fill=255)
    img.putalpha(mask)
    return img


def _draw_dot(canvas, cx, cy, r):
    """Draw a 3D-style dot: inset shadow + base + gradient + specular."""
    draw = ImageDraw.Draw(canvas)

    # Inset shadow (offset down)
    draw.ellipse([cx - r - 2, cy - r + 2, cx + r + 2, cy + r + 6], fill=DOT_SHADOW)

    # Outer glow ring
    draw.ellipse([cx - r - 3, cy - r - 3, cx + r + 3, cy + r + 3],
                  fill=(50, 55, 90))

    # Base dot
    draw.ellipse([cx - r, cy - r, cx + r, cy + r], fill=DOT_BASE)

    # Inner gradient: brighter toward top-left
    for i in range(r, 2, -1):
        t = 1 - (i / r)
        # Shift center toward top-left for 3D lighting
        ox, oy = -int(r * 0.15 * t), -int(r * 0.2 * t)
        alpha = int(100 * t * t)
        draw.ellipse([cx + ox - i, cy + oy - i, cx + ox + i, cy + oy + i],
                      fill=(*DOT_SHINE, alpha))

    # Strong specular highlight
    sr = int(r * 0.32)
    sx, sy = cx - int(r * 0.28), cy - int(r * 0.32)
    draw.ellipse([sx - sr, sy - sr, sx + sr, sy + sr], fill=(*DOT_SHINE, 230))
    # Tiny hard spec
    tr = int(r * 0.15)
    draw.ellipse([sx - tr, sy - tr, sx + tr, sy + tr], fill=(*DOT_SHINE, 255))


def generate():
    canvas = Image.new("RGBA", (SIZE, SIZE), (*BG, 255))

    # ── Tile dimensions ──
    tile_w, tile_h = 310, 420
    cr = 32
    tx = (SIZE - tile_w) // 2
    ty = (SIZE - tile_h) // 2

    # ── Outer glow (bluish like the original) ──
    glow = Image.new("RGBA", (SIZE, SIZE), (0, 0, 0, 0))
    gd = ImageDraw.Draw(glow)
    # Multiple passes for soft glow
    for expand in [14, 10, 6, 3]:
        alpha = [12, 20, 30, 50][([14, 10, 6, 3].index(expand))]
        gd.rounded_rectangle(
            [tx - expand, ty - expand, tx + tile_w + expand, ty + tile_h + expand],
            radius=cr + expand, fill=(*EDGE_GLOW, alpha))
    glow = glow.filter(ImageFilter.GaussianBlur(8))
    canvas = Image.alpha_composite(canvas, glow)

    # ── Drop shadow ──
    shadow = Image.new("RGBA", (SIZE, SIZE), (0, 0, 0, 0))
    ImageDraw.Draw(shadow).rounded_rectangle(
        [tx + 5, ty + 7, tx + tile_w + 5, ty + tile_h + 7],
        radius=cr, fill=(0, 0, 0, 120))
    shadow = shadow.filter(ImageFilter.GaussianBlur(14))
    canvas = Image.alpha_composite(canvas, shadow)

    # ── Tile edge/border (thin, slightly brighter) ──
    draw = ImageDraw.Draw(canvas)
    draw.rounded_rectangle(
        [tx - 2, ty - 2, tx + tile_w + 2, ty + tile_h + 2],
        radius=cr + 2, fill=EDGE_GLOW)

    # ── Tile face with gradient ──
    tile_face = _tile_gradient(tile_w + 1, tile_h + 1, TILE_LIGHT, TILE_DARK, cr)
    canvas.paste(tile_face, (tx, ty), mask=tile_face)

    # ── Divider line ──
    draw = ImageDraw.Draw(canvas)
    mid_y = ty + tile_h // 2
    margin = 30
    draw.line([(tx + margin, mid_y), (tx + tile_w - margin, mid_y)],
              fill=(*DIVIDER_C, 255), width=3)

    # Subtle divider glow
    div_glow = Image.new("RGBA", (SIZE, SIZE), (0, 0, 0, 0))
    ImageDraw.Draw(div_glow).line(
        [(tx + margin, mid_y), (tx + tile_w - margin, mid_y)],
        fill=(*EDGE_GLOW, 25), width=9)
    div_glow = div_glow.filter(ImageFilter.GaussianBlur(4))
    canvas = Image.alpha_composite(canvas, div_glow)

    # ── Dots ──
    cx_tile = tx + tile_w // 2
    dot_r = 22

    # Top half: 4 dots (2×2)
    top_mid = ty + tile_h // 4
    dx, dy = 72, 66
    top_dots = [
        (cx_tile - dx, top_mid - dy),
        (cx_tile + dx, top_mid - dy),
        (cx_tile - dx, top_mid + dy),
        (cx_tile + dx, top_mid + dy),
    ]
    # Bottom half: 1 dot
    bot_mid = ty + 3 * tile_h // 4
    bot_dots = [(cx_tile, bot_mid)]

    for (dcx, dcy) in top_dots + bot_dots:
        _draw_dot(canvas, dcx, dcy, dot_r)

    # ── Subtle accent glow at bottom ──
    accent = Image.new("RGBA", (SIZE, SIZE), (0, 0, 0, 0))
    ImageDraw.Draw(accent).rounded_rectangle(
        [tx + 20, ty + tile_h - 6, tx + tile_w - 20, ty + tile_h + 2],
        radius=3, fill=(*ACCENT, 25))
    accent = accent.filter(ImageFilter.GaussianBlur(8))
    canvas = Image.alpha_composite(canvas, accent)

    # Save
    OUT.parent.mkdir(parents=True, exist_ok=True)
    canvas.convert("RGB").save(str(OUT), "PNG", optimize=True)
    print(f"✅ {OUT} ({OUT.stat().st_size // 1024}KB)")


if __name__ == "__main__":
    os.chdir(Path(__file__).resolve().parent.parent)
    generate()
