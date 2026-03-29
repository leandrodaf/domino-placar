#!/usr/bin/env python3
"""
Take real screenshots of the Domino Placar app using Playwright,
then compose polished Play Store listing assets.

Requirements:
  - Server: SESSION_SECRET=<your-secret> go run main.go
  - pip install playwright Pillow
  - playwright install chromium
  - sudo apt install fonts-noto-color-emoji
"""

import hashlib
import hmac as hmac_mod
import sqlite3
import uuid
import os
from pathlib import Path
from PIL import Image, ImageDraw, ImageFont, ImageFilter
from playwright.sync_api import sync_playwright

SERVER_URL = "http://localhost:8080"
DISPLAY_URL = "https://dominoplacar.net"
DB_PATH = "domino.db"
OUT_DIR = Path("play-store")
RAW_DIR = OUT_DIR / "raw"

SESSION_SECRET = os.environ.get("SESSION_SECRET", "screenshot-secret-2024")
PHONE_VP = {"width": 412, "height": 915}

PLAYERS = [
    ("Carlos", "carlos-uid-001"),
    ("Ana", "ana-uid-002"),
    ("Roberto", "roberto-uid-003"),
    ("Dona Maria", "dona-maria-uid-004"),
]
SCORES_R1 = [8, 15, 3, 22]
SCORES_R2 = [12, 6, 18, 5]
SCORES_R3 = [7, 10, 14, 9]

# ── Colors (matching app theme) ──
BG = (9, 9, 15)
ACCENT = (233, 69, 96)
GOLD = (245, 197, 24)
TEXT_W = (237, 237, 248)
TEXT_M = (152, 152, 180)


# ═══════════════════════════════════════════════════════════════
# FONTS
# ═══════════════════════════════════════════════════════════════

_fc = {}


def font(size, bold=True):
    key = (size, bold)
    if key not in _fc:
        p = ("/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf" if bold
             else "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf")
        _fc[key] = ImageFont.truetype(p, size) if os.path.exists(p) else ImageFont.load_default()
    return _fc[key]


# ═══════════════════════════════════════════════════════════════
# CRYPTO — replicate Go HMAC host cookies
# ═══════════════════════════════════════════════════════════════

def _app_secret():
    return hashlib.sha256(SESSION_SECRET.encode()).digest()


def _mac_sign(data):
    return hmac_mod.new(_app_secret(), data.encode(), hashlib.sha256).hexdigest()


def host_cookie(eid):
    return {"name": f"domino_h_{eid}", "value": _mac_sign(f"host:{eid}"),
            "domain": "localhost", "path": "/"}


# ═══════════════════════════════════════════════════════════════
# DB SEEDING
# ═══════════════════════════════════════════════════════════════

def seed_database():
    conn = sqlite3.connect(DB_PATH)
    c = conn.cursor()

    # Active match (mid-game, round 4)
    mid = str(uuid.uuid4())
    c.execute("INSERT INTO matches (id,status,base_url) VALUES (?,'active',?)", (mid, DISPLAY_URL))
    pids = []
    for i, (name, uid) in enumerate(PLAYERS):
        pid = str(uuid.uuid4())
        pids.append(pid)
        total = SCORES_R1[i] + SCORES_R2[i] + SCORES_R3[i]
        c.execute("INSERT INTO players (id,match_id,name,unique_identifier,total_score,status) "
                  "VALUES (?,?,?,?,?,'active')", (pid, mid, name, uid, total))
    for rn in range(1, 4):
        rid = str(uuid.uuid4())
        sc = [SCORES_R1, SCORES_R2, SCORES_R3][rn - 1]
        c.execute("INSERT INTO rounds (id,match_id,round_number,status,starter_player_id,winner_player_id) "
                  "VALUES (?,?,?,'finished',?,?)",
                  (rid, mid, rn, pids[rn % len(pids)], pids[sc.index(min(sc))]))
    r4id = str(uuid.uuid4())
    c.execute("INSERT INTO rounds (id,match_id,round_number,status,starter_player_id) "
              "VALUES (?,?,4,'active',?)", (r4id, mid, pids[0]))
    for i in range(2):
        c.execute("INSERT INTO hand_images (id,round_id,player_id,image_path,points_calculated,confirmed) "
                  "VALUES (?,?,?,'',?,?)", (str(uuid.uuid4()), r4id, pids[i], [12, 8][i], [1, 0][i]))
    conn.commit()

    # Finished match
    fid = str(uuid.uuid4())
    c.execute("INSERT INTO matches (id,status,base_url) VALUES (?,'finished',?)", (fid, DISPLAY_URL))
    fpids = []
    for name, uid, score in [("Roberto", "r01", 27), ("Carlos", "c01", 52),
                              ("Ana", "a01", 48), ("Dona Maria", "d01", 53)]:
        pid = str(uuid.uuid4())
        fpids.append(pid)
        st = "active" if score <= 51 else "estourou"
        c.execute("INSERT INTO players (id,match_id,name,unique_identifier,total_score,status) "
                  "VALUES (?,?,?,?,?,?)", (pid, fid, name, uid, score, st))
    c.execute("UPDATE matches SET winner_player_id=? WHERE id=?", (fpids[0], fid))
    c.execute("INSERT INTO rounds (id,match_id,round_number,status,starter_player_id,winner_player_id) "
              "VALUES (?,?,6,'finished',?,?)", (str(uuid.uuid4()), fid, fpids[0], fpids[0]))
    conn.commit()

    # Lobby match
    lid = str(uuid.uuid4())
    c.execute("INSERT INTO matches (id,status,base_url) VALUES (?,'waiting',?)", (lid, DISPLAY_URL))
    for name, uid in [("Carlos", "lc"), ("Ana", "la"), ("Roberto", "lr")]:
        c.execute("INSERT INTO players (id,match_id,name,unique_identifier,total_score,status) "
                  "VALUES (?,?,?,?,0,'active')", (str(uuid.uuid4()), lid, name, uid))
    conn.commit()
    conn.close()
    return mid, r4id, fid, lid


# ═══════════════════════════════════════════════════════════════
# PLAYWRIGHT SCREENSHOTS
# ═══════════════════════════════════════════════════════════════

def take_screenshots():
    print("🎮 Seeding database...")
    mid, r4id, fid, lid = seed_database()
    RAW_DIR.mkdir(parents=True, exist_ok=True)

    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)

        for lang, folder in [("pt", "pt-BR"), ("en", "en-US")]:
            print(f"\n📸 {folder}...")
            out = RAW_DIR / folder
            out.mkdir(parents=True, exist_ok=True)

            ctx = browser.new_context(
                viewport=PHONE_VP, device_scale_factor=2.625,
                locale="pt-BR" if lang == "pt" else "en-US",
                color_scheme="dark")
            for m in [mid, fid, lid]:
                ctx.add_cookies([host_cookie(m)])
            page = ctx.new_page()

            shots = [
                ("01-home",           f"{SERVER_URL}/?lang={lang}",                                       "networkidle"),
                ("02-lobby",          f"{SERVER_URL}/match/{lid}/lobby?lang={lang}",                      "load"),
                ("03-game",           f"{SERVER_URL}/match/{mid}/round/{r4id}/game?lang={lang}",          "load"),
                ("04-ranking-active", f"{SERVER_URL}/match/{mid}/ranking?lang={lang}",                    "load"),
                ("05-ranking-winner", f"{SERVER_URL}/match/{fid}/ranking?lang={lang}",                    "load"),
                ("06-join",           f"{SERVER_URL}/match/{lid}/join?lang={lang}",                       "networkidle"),
            ]
            for name, url, wait in shots:
                print(f"  📱 {name}")
                page.goto(url, wait_until=wait)
                page.wait_for_timeout(1200)
                page.screenshot(path=str(out / f"{name}.png"), full_page=False)
            ctx.close()
        browser.close()

    print("\n✅ Raw screenshots done")
    return mid, r4id, fid, lid


# ═══════════════════════════════════════════════════════════════
# IMAGE COMPOSITION
# ═══════════════════════════════════════════════════════════════

def _gradient(w, h, top, bot):
    """Fast vertical gradient via line drawing."""
    img = Image.new("RGB", (w, h), bot)
    draw = ImageDraw.Draw(img)
    for y in range(h):
        t = y / max(h - 1, 1)
        draw.line([(0, y), (w - 1, y)],
                  fill=(int(top[0]*(1-t)+bot[0]*t),
                        int(top[1]*(1-t)+bot[1]*t),
                        int(top[2]*(1-t)+bot[2]*t)))
    return img


def _ctxt(draw, y, text, fnt, fill, w):
    """Centered text."""
    bb = draw.textbbox((0, 0), text, font=fnt)
    draw.text(((w - bb[2] + bb[0]) // 2, y), text, fill=fill, font=fnt)


def compose_phone(raw_path, title, subtitle, w=1080, h=1920):
    """
    Compose a clean, well-aligned Play Store phone screenshot.

    Layout (1080×1920):
      ┌──────────────────────┐
      │                      │  60px top margin
      │      TITLE (bold)    │  ~60px
      │      ── bar ──       │  ~30px
      │     subtitle         │  ~40px
      │                      │  40px gap
      │  ┌──────────────┐    │
      │  │               │   │
      │  │  screenshot   │   │  fills rest
      │  │               │   │
      │  └──────────────┘    │
      │                      │  30px bottom
      └──────────────────────┘
    """
    # Gradient bg: subtle accent tint at top → pure dark at bottom
    top_c = (int(ACCENT[0]*0.10+BG[0]*0.90),
             int(ACCENT[1]*0.10+BG[1]*0.90),
             int(ACCENT[2]*0.10+BG[2]*0.90))
    canvas = _gradient(w, h, top_c, BG)
    draw = ImageDraw.Draw(canvas)

    # ── Text ──
    tf = font(56, bold=True)
    sf = font(30, bold=False)

    text_top = 60
    _ctxt(draw, text_top, title, tf, TEXT_W, w)

    # Accent bar (centered, below title)
    bar_y = text_top + 74
    bar_w = 50
    bx = (w - bar_w) // 2
    draw.rounded_rectangle([bx, bar_y, bx + bar_w, bar_y + 4], radius=2, fill=ACCENT)

    _ctxt(draw, bar_y + 20, subtitle, sf, TEXT_M, w)

    # ── Screenshot ──
    img = Image.open(raw_path).convert("RGB")

    # Calculate dimensions: 90% width, fill remaining height
    scr_margin_top = bar_y + 70
    scr_margin_bottom = 30
    scr_margin_x = int(w * 0.05)

    avail_w = w - 2 * scr_margin_x
    avail_h = h - scr_margin_top - scr_margin_bottom

    # Scale to fit: prioritize filling width
    scale = avail_w / img.width
    scaled_h = int(img.height * scale)
    if scaled_h > avail_h:
        scale = avail_h / img.height
    scaled_w = int(img.width * scale)
    scaled_h = int(img.height * scale)

    img = img.resize((scaled_w, scaled_h), Image.LANCZOS)

    sx = (w - scaled_w) // 2
    sy = h - scaled_h - scr_margin_bottom  # pin to bottom

    # Drop shadow
    shadow = Image.new("RGBA", (scaled_w + 30, scaled_h + 30), (0, 0, 0, 0))
    ImageDraw.Draw(shadow).rounded_rectangle(
        [0, 0, shadow.width - 1, shadow.height - 1], radius=24, fill=(0, 0, 0, 70))
    shadow = shadow.filter(ImageFilter.GaussianBlur(12))
    canvas.paste(Image.merge("RGB", shadow.split()[:3]),
                 (sx - 15, sy - 5), mask=shadow.split()[3])

    # Rounded corners mask
    mask = Image.new("L", (scaled_w, scaled_h), 0)
    ImageDraw.Draw(mask).rounded_rectangle(
        [0, 0, scaled_w - 1, scaled_h - 1], radius=20, fill=255)
    canvas.paste(img, (sx, sy), mask=mask)

    # Border
    draw.rounded_rectangle([sx - 1, sy - 1, sx + scaled_w, sy + scaled_h],
                           radius=20, outline=(ACCENT[0], ACCENT[1], ACCENT[2], 80), width=2)

    return canvas


def make_feature_graphic():
    """Create 1024×500 feature graphic."""
    w, h = 1024, 500
    canvas = _gradient(w, h, BG, BG)
    draw = ImageDraw.Draw(canvas)

    # Subtle accent glow (ellipse)
    glow = Image.new("RGBA", (w, h), (0, 0, 0, 0))
    ImageDraw.Draw(glow).ellipse([-200, -100, 500, h + 100], fill=(*ACCENT, 18))
    canvas = Image.alpha_composite(canvas.convert("RGBA"), glow).convert("RGB")
    draw = ImageDraw.Draw(canvas)

    # Icon (use the generated Play Store icon)
    icon_src = OUT_DIR / "app-icon-512.png"
    if not icon_src.exists():
        icon_src = Path("images/icone-domino-placar.png")
    text_x = 100
    if icon_src.exists():
        icon = Image.open(icon_src).convert("RGBA")
        icon = icon.resize((160, 160), Image.LANCZOS)
        mask = Image.new("L", (160, 160), 0)
        ImageDraw.Draw(mask).rounded_rectangle([0, 0, 159, 159], radius=32, fill=255)
        canvas.paste(icon, (80, (h - 160) // 2), mask=mask)
        text_x = 280

    bf = font(76, bold=True)
    mf = font(28, bold=False)
    cy = h // 2

    # Accent vertical bar
    draw.rectangle([text_x - 16, cy - 85, text_x - 12, cy + 70], fill=ACCENT)

    draw.text((text_x, cy - 90), "Dominó", fill=TEXT_W, font=bf)
    draw.text((text_x, cy - 10), "Placar", fill=ACCENT, font=bf)
    draw.text((text_x, cy + 90), "Marcador de pontos fácil e rápido", fill=TEXT_M, font=mf)

    # Faded screenshot preview on right
    raw = RAW_DIR / "pt-BR" / "03-game.png"
    if raw.exists():
        prev = Image.open(raw).convert("RGBA")
        pw = 300
        ph = int(prev.height * pw / prev.width)
        prev = prev.resize((pw, ph), Image.LANCZOS)
        fade = Image.new("L", (pw, ph), 0)
        fd = ImageDraw.Draw(fade)
        for x in range(pw):
            a = int(140 * max(0, (x - pw * 0.3)) / (pw * 0.7))
            fd.line([(x, 0), (x, ph)], fill=min(a, 130))
        canvas.paste(prev, (w - pw - 40, (h - ph) // 2), mask=fade)

    return canvas


# ═══════════════════════════════════════════════════════════════
# METADATA
# ═══════════════════════════════════════════════════════════════

METADATA = {
    "pt-BR": {
        "title": "Dominó Placar - Marcador",
        "short_description": "Marcador de pontos para dominó. Crie partidas, convide amigos por QR Code e acompanhe o placar!",
        "full_description": """Dominó Placar é o marcador de pontos definitivo para suas partidas de dominó!

🎮 CRIE PARTIDAS INSTANTÂNEAS
Crie uma partida com um toque e convide seus amigos escaneando o QR Code ou compartilhando o link pelo WhatsApp.

📊 PLACAR EM TEMPO REAL
Acompanhe a pontuação de todos os jogadores em tempo real. A mesa interativa mostra quem já jogou, quem ganhou a rodada e quem está na frente.

🏆 TORNEIOS
Organize torneios com múltiplas mesas. O sistema distribui os jogadores automaticamente e gera o ranking geral.

👥 TURMAS
Crie seu grupo de jogadores recorrentes. Acompanhe o ranking histórico da turma e organize partidas fácil.

📸 CONTAGEM AUTOMÁTICA DE PEDRAS
Tire uma foto das pedras na mão e o app conta os pontos automaticamente usando inteligência artificial.

🐂 SISTEMA TRADICIONAL DO BOI
Quem estourar (passar de 51 pontos) está fora. O último sobrevivente ganha o boi!

✨ RECURSOS
• Marcador de pontos intuitivo
• Convite por QR Code e link
• Placar em tempo real via SSE
• Torneios com múltiplas mesas
• Turmas com ranking histórico
• Contagem automática com IA
• Interface escura elegante
• 100% gratuito, sem anúncios
• Funciona em celular e tablet

Dominó Placar é feito por jogadores de dominó, para jogadores de dominó. Baixe agora e organize sua próxima partida!""",
    },
    "en-US": {
        "title": "Domino Scorekeeper",
        "short_description": "Domino score tracker. Create matches, invite friends via QR Code and track scores in real time!",
        "full_description": """Domino Scorekeeper is the ultimate score tracker for your domino matches!

🎮 INSTANT MATCH CREATION
Create a match with one tap and invite friends by scanning a QR Code or sharing the link via WhatsApp.

📊 REAL-TIME SCOREBOARD
Track every player's score in real time. The interactive table shows who has played, who won the round, and who's in the lead.

🏆 TOURNAMENTS
Organize tournaments with multiple tables. The system distributes players automatically and generates the overall ranking.

👥 GROUPS
Create recurring player groups. Track your group's historical ranking and easily organize matches.

📸 AUTOMATIC TILE COUNTING
Take a photo of the tiles in your hand and the app counts the points automatically using artificial intelligence.

🐂 TRADITIONAL "OX" SYSTEM
If you bust (go over 51 points), you're out. The last player standing wins the ox!

✨ FEATURES
• Intuitive score tracker
• QR Code and link invitations
• Real-time scores via SSE
• Multi-table tournaments
• Groups with historical ranking
• Automatic counting with AI
• Elegant dark interface
• 100% free, no ads
• Works on phone and tablet

Domino Scorekeeper is made by domino players, for domino players. Download now and organize your next match!""",
    },
}


# ═══════════════════════════════════════════════════════════════
# TITLES PER SCREENSHOT
# ═══════════════════════════════════════════════════════════════

PHONE_TITLES = {
    "pt-BR": [
        ("01-home",           "Tela Inicial",        "Crie partidas com um toque"),
        ("02-lobby",          "Sala de Espera",       "Convide jogadores via QR Code"),
        ("03-game",           "Mesa de Jogo",         "Pontuação rodada a rodada"),
        ("04-ranking-active", "Placar ao Vivo",       "Acompanhe quem lidera"),
        ("05-ranking-winner", "Vitória!",             "O boi é seu, campeão!"),
        ("06-join",           "Entre na Partida",     "Escaneie o código e jogue"),
    ],
    "en-US": [
        ("01-home",           "Home Screen",          "Create matches instantly"),
        ("02-lobby",          "Waiting Room",          "Invite players via QR Code"),
        ("03-game",           "Game Table",            "Round-by-round scoring"),
        ("04-ranking-active", "Live Scoreboard",       "See who's leading"),
        ("05-ranking-winner", "Victory!",              "The ox is yours, champion!"),
        ("06-join",           "Join a Match",          "Scan the code and play"),
    ],
}


# ═══════════════════════════════════════════════════════════════
# GENERATE ALL ASSETS
# ═══════════════════════════════════════════════════════════════

def generate_all():
    print("\n🎨 Composing Play Store assets...")

    for lang in ["pt-BR", "en-US"]:
        phone_dir = OUT_DIR / lang / "phone"
        phone_dir.mkdir(parents=True, exist_ok=True)

        for i, (base, title, sub) in enumerate(PHONE_TITLES[lang]):
            raw = RAW_DIR / lang / f"{base}.png"
            if not raw.exists():
                print(f"  ⚠️  Missing {raw}")
                continue
            img = compose_phone(str(raw), title, sub)
            dest = phone_dir / f"{i+1:02d}_{base}.png"
            img.save(str(dest), "PNG", optimize=True)
            print(f"  ✅ {dest} ({dest.stat().st_size//1024}KB)")

    fg = make_feature_graphic()
    fg_path = OUT_DIR / "feature-graphic-1024x500.png"
    fg.save(str(fg_path), "PNG", optimize=True)
    print(f"  ✅ {fg_path}")

    icon_src = Path("images/icone-domino-placar.png")
    if icon_src.exists():
        icon = Image.open(icon_src).convert("RGBA").resize((512, 512), Image.LANCZOS)
        bg = Image.new("RGBA", (512, 512), (*BG, 255))
        bg.paste(icon, (0, 0), icon)
        ip = OUT_DIR / "app-icon-512.png"
        bg.convert("RGB").save(str(ip), "PNG", optimize=True)
        print(f"  ✅ {ip}")

    print("\n📝 Metadata...")
    for lang, data in METADATA.items():
        d = OUT_DIR / lang
        d.mkdir(parents=True, exist_ok=True)
        for key, val in data.items():
            p = d / f"{key}.txt"
            p.write_text(val, encoding="utf-8")
            print(f"  ✅ {p}")


if __name__ == "__main__":
    os.chdir(Path(__file__).resolve().parent.parent)
    take_screenshots()
    generate_all()
    print("\n🎉 Done! Assets in play-store/")
