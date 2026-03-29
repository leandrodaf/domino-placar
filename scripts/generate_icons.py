#!/usr/bin/env python3
"""
Gera todos os ícones Android a partir da tile 3_5 real do app.
Recorta a metade inferior do dominó (5 pontos) e gera:
  - ic_launcher.png (mipmap-*dpi) — ícone quadrado com cantos arredondados
  - ic_launcher_round.png (mipmap-*dpi) — ícone circular
  - ic_launcher_foreground.png (drawable-*dpi) — camada foreground para ícone adaptativo
"""

from PIL import Image, ImageDraw, ImageFilter
import os

# Caminhos
BASE_DIR = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
TILE_PATH = os.path.join(BASE_DIR, "static", "tiles", "tile_3_5.png")
RES_DIR = os.path.join(BASE_DIR, "android", "app", "src", "main", "res")

# Tamanhos Android oficiais
MIPMAP_SIZES = {
    "mdpi": 48,
    "hdpi": 72,
    "xhdpi": 96,
    "xxhdpi": 144,
    "xxxhdpi": 192,
}

# Foreground do ícone adaptativo: 108dp (com safe zone 66dp no centro)
FOREGROUND_SIZES = {
    "mdpi": 108,
    "hdpi": 162,
    "xhdpi": 216,
    "xxhdpi": 324,
    "xxxhdpi": 432,
}


def create_rounded_mask(size, radius):
    """Cria máscara com cantos arredondados."""
    mask = Image.new("L", (size, size), 0)
    draw = ImageDraw.Draw(mask)
    draw.rounded_rectangle([(0, 0), (size - 1, size - 1)], radius=radius, fill=255)
    return mask


def create_circle_mask(size):
    """Cria máscara circular."""
    mask = Image.new("L", (size, size), 0)
    draw = ImageDraw.Draw(mask)
    draw.ellipse([(0, 0), (size - 1, size - 1)], fill=255)
    return mask


def extract_icon_from_tile(tile_path):
    """
    Extrai a metade inferior da tile (lado com 5 pontos) e faz um crop quadrado.
    Retorna imagem quadrada RGBA em alta resolução.
    """
    tile = Image.open(tile_path).convert("RGBA")
    w, h = tile.size

    # Pega a metade de baixo (lado com 5 pontos)
    half_h = h // 2
    bottom_half = tile.crop((0, half_h, w, h))

    # Faz quadrado (crop centralizado)
    bw, bh = bottom_half.size
    side = min(bw, bh)
    left = (bw - side) // 2
    top = (bh - side) // 2
    square = bottom_half.crop((left, top, left + side, top + side))

    return square


def generate_foreground(source_img, size):
    """
    Gera foreground para ícone adaptativo.
    O conteúdo fica centralizado na safe zone (66/108 = ~61% do centro).
    """
    # Canvas transparente no tamanho do foreground
    canvas = Image.new("RGBA", (size, size), (0, 0, 0, 0))

    # A safe zone é 66/108 do tamanho total
    safe_zone = int(size * 66 / 108)
    margin = (size - safe_zone) // 2

    # Resize a imagem fonte para caber na safe zone
    icon = source_img.resize((safe_zone, safe_zone), Image.LANCZOS)

    # Cria cantos arredondados na imagem
    radius = int(safe_zone * 0.15)
    mask = create_rounded_mask(safe_zone, radius)
    
    # Aplica máscara
    icon_with_mask = Image.new("RGBA", (safe_zone, safe_zone), (0, 0, 0, 0))
    icon_with_mask.paste(icon, (0, 0), mask)

    # Cola no canvas centralizado
    canvas.paste(icon_with_mask, (margin, margin), icon_with_mask)

    return canvas


def generate_launcher_icon(source_img, size, rounded=True):
    """Gera ícone do launcher (quadrado com cantos ou circular)."""
    icon = source_img.resize((size, size), Image.LANCZOS)

    if rounded:
        # Cantos arredondados (~22% do tamanho, padrão Android)
        radius = int(size * 0.22)
        mask = create_rounded_mask(size, radius)
    else:
        mask = create_circle_mask(size)

    result = Image.new("RGBA", (size, size), (0, 0, 0, 0))
    result.paste(icon, (0, 0), mask)
    return result


def main():
    print(f"📦 Lendo tile: {TILE_PATH}")
    source = extract_icon_from_tile(TILE_PATH)
    print(f"   Extraído quadrado: {source.size}")

    # Gera ic_launcher.png e ic_launcher_round.png para cada dpi
    for dpi, size in MIPMAP_SIZES.items():
        folder = os.path.join(RES_DIR, f"mipmap-{dpi}")
        os.makedirs(folder, exist_ok=True)

        # ic_launcher.png (quadrado com cantos arredondados)
        launcher = generate_launcher_icon(source, size, rounded=True)
        path = os.path.join(folder, "ic_launcher.png")
        launcher.save(path, "PNG", optimize=True)
        print(f"   ✅ {path} ({size}x{size})")

        # ic_launcher_round.png (circular)
        launcher_round = generate_launcher_icon(source, size, rounded=False)
        path_round = os.path.join(folder, "ic_launcher_round.png")
        launcher_round.save(path_round, "PNG", optimize=True)
        print(f"   ✅ {path_round} ({size}x{size})")

    # Gera ic_launcher_foreground.png para cada dpi
    for dpi, size in FOREGROUND_SIZES.items():
        folder = os.path.join(RES_DIR, f"drawable-{dpi}")
        os.makedirs(folder, exist_ok=True)

        fg = generate_foreground(source, size)
        path = os.path.join(folder, "ic_launcher_foreground.png")
        fg.save(path, "PNG", optimize=True)
        print(f"   ✅ {path} ({size}x{size})")

    print("\n🎯 Todos os ícones gerados com sucesso!")
    print("   → Faça Build > Clean Project no Android Studio")
    print("   → Desinstale o app do celular antes de rodar")


if __name__ == "__main__":
    main()
