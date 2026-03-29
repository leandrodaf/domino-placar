#!/usr/bin/env python3
"""
Record a promotional video for the Dominó Placar Play Store listing.

Uses Playwright's built-in video recording for smooth animations,
then assembles with Pillow title cards and ffmpeg.

Prerequisites:
  - Server running: SESSION_SECRET=<secret> go run main.go
  - pip install playwright Pillow numpy
  - playwright install chromium

Usage:
  python scripts/record_promo_video.py             # full run
  python scripts/record_promo_video.py --compose   # skip recording, reuse existing webms
"""

import argparse
import hashlib
import hmac as hmac_mod
import json
import os
import sqlite3
import subprocess
import sys
import uuid
from pathlib import Path

from PIL import Image, ImageDraw, ImageFont
from playwright.sync_api import sync_playwright

# ── Config ───────────────────────────────────────────────────────────
SERVER_URL    = os.environ.get("SERVER_URL", "http://localhost:8080")
DISPLAY_URL   = "https://dominoplacar.net"
DB_PATH       = "domino.db"
SESSION_SECRET = os.environ.get("SESSION_SECRET", "screenshot-secret-2024")

# Remote match IDs from seed_remote.py (override local DB seeding)
_ENV_MID  = os.environ.get("MID")
_ENV_R4ID = os.environ.get("R4ID")
_ENV_FID  = os.environ.get("FID")
_ENV_LID  = os.environ.get("LID")

OUT_DIR      = Path("play-store")
VIDEO_DIR    = OUT_DIR / "video-clips"
PARTS_DIR    = VIDEO_DIR / "parts"
FINAL_VIDEO  = OUT_DIR / "promo-video.mp4"

W, H         = 1080, 1920   # Full HD portrait (9:16)
VP_W, VP_H   = 540, 960     # Playwright viewport (×2 DPR = W×H)

FPS          = 30            # Standard for YouTube / Play Store
FADE_DUR     = 0.0           # hard cuts — fast & modern
TITLE_DUR    = 1.2            # just enough to read
INTRO_DUR    = 2.0            # grab attention fast
OUTRO_DUR    = 2.5

BG     = (9, 9, 15)
ACCENT = (233, 69, 96)
TEXT_W = (237, 237, 248)
TEXT_M = (152, 152, 180)

# ── Fonts ─────────────────────────────────────────────────────────────
_fc = {}
def font(size, bold=True):
    key = (size, bold)
    if key not in _fc:
        candidates = (
            ["/usr/share/fonts/truetype/ubuntu/Ubuntu-Bold.ttf",
             "/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf"]
            if bold else
            ["/usr/share/fonts/truetype/ubuntu/Ubuntu-Regular.ttf",
             "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf"]
        )
        for p in candidates:
            if os.path.exists(p):
                _fc[key] = ImageFont.truetype(p, size)
                break
        else:
            _fc[key] = ImageFont.load_default()
    return _fc[key]


# ── ffmpeg helpers ────────────────────────────────────────────────────
_ffmpeg_exe = None
def get_ffmpeg():
    global _ffmpeg_exe
    if not _ffmpeg_exe:
        try:
            import imageio_ffmpeg
            _ffmpeg_exe = imageio_ffmpeg.get_ffmpeg_exe()
        except ImportError:
            _ffmpeg_exe = "ffmpeg"
    return _ffmpeg_exe

def get_ffprobe():
    ffmpeg = get_ffmpeg()
    probe = Path(ffmpeg).parent / "ffprobe"
    return str(probe) if probe.exists() else "ffprobe"

def get_duration(path):
    """Return video duration in seconds."""
    result = subprocess.run(
        [get_ffprobe(), "-v", "quiet", "-print_format", "json",
         "-show_streams", str(path)],
        capture_output=True, text=True,
    )
    try:
        data = json.loads(result.stdout)
        for s in data.get("streams", []):
            if s.get("codec_type") == "video":
                dur = s.get("duration")
                if dur:
                    return float(dur)
    except Exception:
        pass
    return 5.0  # safe fallback


# ── Crypto ────────────────────────────────────────────────────────────
def _app_secret():
    return hashlib.sha256(SESSION_SECRET.encode()).digest()

def _mac_sign(data):
    return hmac_mod.new(_app_secret(), data.encode(), hashlib.sha256).hexdigest()

def host_cookie(eid):
    return {"name": f"domino_h_{eid}",
            "value": _mac_sign(f"host:{eid}"),
            "domain": "localhost", "path": "/"}


# ── DB seeding ────────────────────────────────────────────────────────
PLAYERS = [
    ("Carlos",     "vid-carlos-001"),
    ("Ana",        "vid-ana-002"),
    ("Roberto",    "vid-roberto-003"),
    ("Dona Maria", "vid-dona-maria-004"),
]

def seed_if_needed():
    """Return (mid, r4id, fid, lid), seeding if no matches exist."""
    conn = sqlite3.connect(DB_PATH)
    c = conn.cursor()
    count = c.execute("SELECT COUNT(*) FROM matches").fetchone()[0]

    if count > 0:
        mid = fid = lid = r4id = None
        for rid, st in c.execute("SELECT id, status FROM matches ORDER BY rowid"):
            if st == "active"   and not mid: mid = rid
            elif st == "finished" and not fid: fid = rid
            elif st == "waiting"  and not lid: lid = rid
        if mid:
            row = c.execute(
                "SELECT id FROM rounds WHERE match_id=? AND status='active' LIMIT 1", (mid,)
            ).fetchone()
            if not row:
                row = c.execute(
                    "SELECT id FROM rounds WHERE match_id=? ORDER BY round_number DESC LIMIT 1", (mid,)
                ).fetchone()
            r4id = row[0] if row else None
        conn.close()
        return mid, r4id, fid, lid

    # Fresh seed
    SCORES_R1 = [8, 15, 3, 22]
    SCORES_R2 = [12,  6, 18,  5]
    SCORES_R3 = [7,  10, 14,  9]

    mid = str(uuid.uuid4())
    c.execute("INSERT INTO matches (id,status,base_url) VALUES (?,'active',?)", (mid, DISPLAY_URL))
    pids = []
    for i, (name, uid) in enumerate(PLAYERS):
        pid = str(uuid.uuid4())
        pids.append(pid)
        c.execute(
            "INSERT INTO players (id,match_id,name,unique_identifier,total_score,status) "
            "VALUES (?,?,?,?,?,'active')",
            (pid, mid, name, uid, SCORES_R1[i]+SCORES_R2[i]+SCORES_R3[i])
        )
    for rn in range(1, 4):
        sc = [SCORES_R1, SCORES_R2, SCORES_R3][rn - 1]
        c.execute(
            "INSERT INTO rounds (id,match_id,round_number,status,starter_player_id,winner_player_id) "
            "VALUES (?,?,?,'finished',?,?)",
            (str(uuid.uuid4()), mid, rn, pids[rn % len(pids)], pids[sc.index(min(sc))])
        )
    r4id = str(uuid.uuid4())
    c.execute("INSERT INTO rounds (id,match_id,round_number,status,starter_player_id) "
              "VALUES (?,?,4,'active',?)", (r4id, mid, pids[0]))
    conn.commit()

    fid = str(uuid.uuid4())
    c.execute("INSERT INTO matches (id,status,base_url) VALUES (?,'finished',?)", (fid, DISPLAY_URL))
    fpids = []
    for name, uid, score in [("Roberto","vr01",27),("Carlos","vc01",52),
                              ("Ana","va01",48),("Dona Maria","vd01",53)]:
        pid = str(uuid.uuid4())
        fpids.append(pid)
        c.execute(
            "INSERT INTO players (id,match_id,name,unique_identifier,total_score,status) "
            "VALUES (?,?,?,?,?,?)",
            (pid, fid, name, uid, score, "active" if score <= 51 else "estourou")
        )
    c.execute("UPDATE matches SET winner_player_id=? WHERE id=?", (fpids[0], fid))
    conn.commit()

    lid = str(uuid.uuid4())
    c.execute("INSERT INTO matches (id,status,base_url) VALUES (?,'waiting',?)", (lid, DISPLAY_URL))
    for name, uid in [("Carlos","vlc"),("Ana","vla"),("Roberto","vlr")]:
        c.execute("INSERT INTO players (id,match_id,name,unique_identifier,total_score,status) "
                  "VALUES (?,?,?,?,0,'active')", (str(uuid.uuid4()), lid, name, uid))
    conn.commit()
    conn.close()
    return mid, r4id, fid, lid


# ── Title card frames ─────────────────────────────────────────────────
def _ctxt(draw, y, text, fnt, fill, max_width=None):
    """Draw centred text, wrapping to multiple lines if wider than max_width."""
    if max_width is None:
        max_width = W - 120  # default: leave 60px margin on each side
    # Word-wrap
    words = text.split()
    lines, cur = [], ""
    for w in words:
        test = f"{cur} {w}".strip()
        bb = draw.textbbox((0, 0), test, font=fnt)
        if bb[2] - bb[0] <= max_width:
            cur = test
        else:
            if cur:
                lines.append(cur)
            cur = w
    if cur:
        lines.append(cur)
    total_h = 0
    for line in lines:
        bb = draw.textbbox((0, 0), line, font=fnt)
        tw, lh = bb[2] - bb[0], bb[3] - bb[1]
        draw.text(((W - tw) // 2, y + total_h), line, fill=fill, font=fnt)
        total_h += lh + 12  # 12px line gap
    return total_h - 12 if lines else 0  # total text block height

def make_title_frame(text, subtitle=""):
    canvas = Image.new("RGB", (W, H), BG)
    draw   = ImageDraw.Draw(canvas)
    tf, sf = font(96, bold=True), font(48, bold=False)
    cy     = H // 2 - 80
    th     = _ctxt(draw, cy, text, tf, TEXT_W)
    bar_w  = 80
    by     = cy + th + 30
    bx     = (W - bar_w) // 2
    draw.rounded_rectangle([bx, by, bx + bar_w, by + 6], radius=3, fill=ACCENT)
    if subtitle:
        _ctxt(draw, by + 40, subtitle, sf, TEXT_M)
    return canvas

def make_intro_frame():
    canvas = Image.new("RGB", (W, H), BG)
    draw   = ImageDraw.Draw(canvas)
    icon_path = OUT_DIR / "app-icon-512.png"
    if icon_path.exists():
        icon_size = 220
        icon = Image.open(icon_path).convert("RGBA").resize((icon_size, icon_size), Image.LANCZOS)
        mask = Image.new("L", (icon_size, icon_size), 0)
        ImageDraw.Draw(mask).rounded_rectangle([0, 0, icon_size-1, icon_size-1], radius=44, fill=255)
        canvas.paste(icon, ((W - icon_size) // 2, H // 2 - 220), mask=mask)
    tf, sf = font(108, bold=True), font(44, bold=False)
    cy = H // 2 + 50
    _ctxt(draw, cy,       "Dominó", tf, TEXT_W)
    _ctxt(draw, cy + 120, "Placar", tf, ACCENT)
    _ctxt(draw, cy + 280, "Marcador de pontos fácil e rápido", sf, TEXT_M)
    return canvas

def make_outro_frame():
    canvas = Image.new("RGB", (W, H), BG)
    draw   = ImageDraw.Draw(canvas)
    icon_path = OUT_DIR / "app-icon-512.png"
    if icon_path.exists():
        icon_size = 160
        icon = Image.open(icon_path).convert("RGBA").resize((icon_size, icon_size), Image.LANCZOS)
        mask = Image.new("L", (icon_size, icon_size), 0)
        ImageDraw.Draw(mask).rounded_rectangle([0, 0, icon_size-1, icon_size-1], radius=32, fill=255)
        canvas.paste(icon, ((W - icon_size) // 2, H // 2 - 260), mask=mask)
    tf, sf, uf = font(96, bold=True), font(52, bold=False), font(44, bold=False)
    cy = H // 2 - 40
    _ctxt(draw, cy, "Baixe Agora!", tf, TEXT_W)
    by = cy + 110
    bx = (W - 80) // 2
    draw.rounded_rectangle([bx, by, bx + 80, by + 6], radius=3, fill=ACCENT)
    _ctxt(draw, by + 50,  "Grátis na Play Store", sf, TEXT_M)
    _ctxt(draw, by + 130, "dominoplacar.net",      uf, ACCENT)
    return canvas


# ── Phone frame overlay ───────────────────────────────────────────────
def make_phone_frame_overlay():
    """
    Create a 1080×1920 RGBA image: solid phone bezel with a transparent
    "screen" cutout in the middle.  Overlay this on top of scene videos.
    """
    try:
        import numpy as np
    except ImportError:
        print("  ⚠️  numpy not found — skipping phone frame overlay")
        return None

    BEZEL   = 24          # side/bottom border px
    TOP     = 48          # status-bar area
    RADIUS  = 72          # outer corner radius
    FRAME_C = (18, 18, 26, 255)

    frame = Image.new("RGBA", (W, H), (0, 0, 0, 0))
    draw  = ImageDraw.Draw(frame)
    draw.rounded_rectangle([0, 0, W - 1, H - 1], radius=RADIUS, fill=FRAME_C)

    # Punch transparent screen area
    arr = np.array(frame)
    arr[TOP : H - BEZEL, BEZEL : W - BEZEL, :] = 0
    return Image.fromarray(arr)


# ── Video clip generation ─────────────────────────────────────────────
def image_to_mp4(img, duration, output_path):
    """Convert a PIL Image into a short mp4 clip."""
    ffmpeg = get_ffmpeg()
    tmp    = output_path.with_suffix(".tmp.png")
    img.save(str(tmp))
    vf_parts = [f"fps={FPS}"]
    if FADE_DUR > 0:
        fade_f = int(FADE_DUR * FPS)
        vf_parts.append(f"fade=in:0:{fade_f}")
        vf_parts.append(f"fade=out:st={duration - FADE_DUR:.3f}:d={FADE_DUR}")
    vf = ",".join(vf_parts)
    cmd = [
        ffmpeg, "-y",
        "-loop", "1", "-framerate", str(FPS), "-i", str(tmp),
        "-vf", vf,
        "-t", str(duration),
        "-c:v", "libx264", "-preset", "slow", "-crf", "18",
        "-pix_fmt", "yuv420p", "-movflags", "+faststart",
        str(output_path),
    ]
    subprocess.run(cmd, check=True, stderr=subprocess.DEVNULL)
    tmp.unlink(missing_ok=True)


def webm_to_mp4(webm_path, output_path, frame_png=None):
    """
    Convert a Playwright webm scene to H.264 mp4 with fade in/out.
    Optionally composites an RGBA phone-frame overlay (frame_png).
    """
    ffmpeg   = get_ffmpeg()
    duration = get_duration(webm_path)
    bg_hex   = f"{BG[0]:02x}{BG[1]:02x}{BG[2]:02x}"

    scale_pad = (
        f"scale={W}:{H}:force_original_aspect_ratio=decrease,"
        f"pad={W}:{H}:(ow-iw)/2:(oh-ih)/2:color=#{bg_hex}"
    )
    fade_filter = ""
    if FADE_DUR > 0:
        fade_f = int(FADE_DUR * FPS)
        fade_filter = (
            f",fade=in:0:{fade_f},"
            f"fade=out:st={max(0.0, duration - FADE_DUR):.3f}:d={FADE_DUR}"
        )

    if frame_png and Path(frame_png).exists():
        filter_complex = (
            f"[0:v]{scale_pad},fps={FPS}{fade_filter}[vid];"
            f"[vid][1:v]overlay=0:0[out]"
        )
        cmd = [
            ffmpeg, "-y",
            "-i", str(webm_path),
            "-i", str(frame_png),
            "-filter_complex", filter_complex,
            "-map", "[out]",
            "-c:v", "libx264", "-preset", "slow", "-crf", "18",
            "-pix_fmt", "yuv420p", "-movflags", "+faststart",
            str(output_path),
        ]
    else:
        vf = f"{scale_pad},fps={FPS}{fade_filter}"
        cmd = [
            ffmpeg, "-y",
            "-i", str(webm_path),
            "-vf", vf,
            "-c:v", "libx264", "-preset", "slow", "-crf", "18",
            "-pix_fmt", "yuv420p", "-movflags", "+faststart",
            str(output_path),
        ]

    result = subprocess.run(cmd, capture_output=True, text=True)
    if result.returncode != 0:
        print(f"  ⚠️  ffmpeg error for {webm_path.name}:\n{result.stderr[-300:]}")
        raise subprocess.CalledProcessError(result.returncode, cmd)


# ── Scene recording ───────────────────────────────────────────────────
SCENE_TITLES = [
    ("Crie Partidas",       "Com um toque, pronto pra jogar"),
    ("Convide por QR Code", "Compartilhe com os amigos"),
    ("Mesa de Jogo",        "Pontuação rodada a rodada"),
    ("Placar ao Vivo",      "Acompanhe quem lidera"),
    ("Vitória!",            "O boi é seu, campeão!"),
]


def _build_scene_list(mid, r4id, fid, lid):
    return [
        ("scene-01-home",
         f"{SERVER_URL}/?lang=pt", "networkidle",
         [("wait", 800), ("scroll", 400), ("wait", 600),
          ("scroll_top", None), ("wait", 500)]),

        ("scene-02-lobby",
         f"{SERVER_URL}/match/{lid}/lobby?lang=pt", "load",
         [("wait", 800), ("scroll", 250), ("wait", 600),
          ("scroll_top", None), ("wait", 500)]),

        ("scene-03-game",
         f"{SERVER_URL}/match/{mid}/round/{r4id}/game?lang=pt", "load",
         [("wait", 800), ("scroll", 200), ("wait", 600),
          ("scroll_top", None), ("wait", 500)]),

        ("scene-04-ranking",
         f"{SERVER_URL}/match/{mid}/ranking?lang=pt", "load",
         [("wait", 800), ("scroll", 400), ("wait", 600),
          ("scroll_top", None), ("wait", 500)]),

        ("scene-05-winner",
         f"{SERVER_URL}/match/{fid}/ranking?lang=pt", "load",
         [("wait", 800), ("scroll", 300), ("wait", 600),
          ("scroll_top", None), ("wait", 500)]),
    ]


def record_scene(browser, scene_name, url, wait_mode, actions, cookies):
    """
    Record one app scene using Playwright's built-in video recording.
    Returns the path to the generated webm file.
    Smooth animations — no frame-by-frame screenshot capture.
    """
    scene_dir = VIDEO_DIR / scene_name
    scene_dir.mkdir(parents=True, exist_ok=True)

    ctx = browser.new_context(
        viewport             = {"width": VP_W, "height": VP_H},
        device_scale_factor  = 2,
        record_video_dir     = str(scene_dir),
        record_video_size    = {"width": VP_W, "height": VP_H},  # match viewport; ffmpeg scales to W×H
        locale               = "pt-BR",
        color_scheme         = "dark",
    )
    for cookie in cookies:
        ctx.add_cookies([cookie])

    page = ctx.new_page()
    page.goto(url, wait_until=wait_mode)
    page.wait_for_timeout(800)   # let page stabilise after load

    for action, val in actions:
        if action == "wait":
            page.wait_for_timeout(val)
        elif action == "scroll":
            page.evaluate(f"window.scrollBy({{top:{val},behavior:'smooth'}})")
            page.wait_for_timeout(800)
        elif action == "scroll_top":
            page.evaluate("window.scrollTo({top:0,behavior:'smooth'})")
            page.wait_for_timeout(700)
        elif action == "click":
            page.click(val)
            page.wait_for_timeout(500)

    # video.path() is only valid while the page is still open
    video_path = Path(page.video.path())
    page.close()
    ctx.close()
    return video_path


def record_all_scenes():
    """Record every scene with Playwright and return list of (name, webm_path)."""
    print("🎬 Recording scenes with Playwright (smooth video)...")
    if _ENV_MID:
        print(f"🌐 Using remote data from {SERVER_URL}")
        mid, r4id, fid, lid = _ENV_MID, _ENV_R4ID, _ENV_FID, _ENV_LID
    else:
        print("🎮 Seeding local database...")
        mid, r4id, fid, lid = seed_if_needed()
    VIDEO_DIR.mkdir(parents=True, exist_ok=True)

    scene_list = _build_scene_list(mid, r4id, fid, lid)
    cookies    = [host_cookie(m) for m in (mid, fid, lid) if m]
    webm_paths = []

    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        for scene_name, url, wait_mode, actions in scene_list:
            print(f"  🎥 {scene_name}...", end=" ", flush=True)
            webm = record_scene(browser, scene_name, url, wait_mode, actions, cookies)
            dur  = get_duration(webm)
            print(f"✅  {dur:.1f}s  →  {webm}")
            webm_paths.append((scene_name, webm))
        browser.close()

    return webm_paths


def find_existing_webms():
    """Find previously recorded webm files (for --compose mode)."""
    webm_paths = []
    # scene dirs are scene-01-home, scene-02-lobby, …
    for scene_name, _, _, _ in _build_scene_list("x", "x", "x", "x"):
        scene_dir = VIDEO_DIR / scene_name
        if not scene_dir.exists():
            print(f"  ⚠️  Missing {scene_dir}")
            continue
        webms = sorted(scene_dir.glob("*.webm"))
        if not webms:
            print(f"  ⚠️  No webm in {scene_dir}")
            continue
        # Take the most recently modified
        webm = max(webms, key=lambda p: p.stat().st_mtime)
        dur  = get_duration(webm)
        print(f"  ♻️  {scene_name}  {dur:.1f}s  →  {webm.name}")
        webm_paths.append((scene_name, webm))
    return webm_paths


# ── Final composition ─────────────────────────────────────────────────
def compose_final(webm_paths):
    """
    Build the final promo video:
      intro → [title → scene]×5 → outro
    Each part is a separate mp4 (fade in/out), then concat with ffmpeg.
    """
    print("\n🎬 Composing final video...")
    PARTS_DIR.mkdir(parents=True, exist_ok=True)

    # Generate phone-frame overlay (optional, needs numpy)
    frame_png = PARTS_DIR / "phone-frame.png"
    if not frame_png.exists():
        print("  🖼️  Generating phone-frame overlay...")
        pf = make_phone_frame_overlay()
        if pf is not None:
            pf.save(str(frame_png), "PNG")
        else:
            frame_png = None
    use_frame = frame_png and frame_png.exists()

    clips = []

    # ── Intro ──
    print("  📝 Generating intro...")
    intro = PARTS_DIR / "00-intro.mp4"
    image_to_mp4(make_intro_frame(), INTRO_DUR, intro)
    clips.append(intro)

    # ── Scenes ──
    for i, (scene_name, webm_path) in enumerate(webm_paths):
        title, sub = SCENE_TITLES[i] if i < len(SCENE_TITLES) else (scene_name, "")

        t_out = PARTS_DIR / f"{i+1:02d}-title.mp4"
        print(f"  📝 Title card: {title}")
        image_to_mp4(make_title_frame(title, sub), TITLE_DUR, t_out)
        clips.append(t_out)

        s_out = PARTS_DIR / f"{i+1:02d}-scene.mp4"
        print(f"  🎞️  Converting {scene_name}...")
        webm_to_mp4(webm_path, s_out, frame_png=frame_png if use_frame else None)
        clips.append(s_out)

    # ── Outro ──
    print("  📝 Generating outro...")
    outro = PARTS_DIR / "99-outro.mp4"
    image_to_mp4(make_outro_frame(), OUTRO_DUR, outro)
    clips.append(outro)

    # ── Concat ──
    ffmpeg      = get_ffmpeg()
    concat_file = PARTS_DIR / "concat.txt"
    concat_file.write_text(
        "\n".join(f"file '{c.absolute()}'" for c in clips) + "\n"
    )

    total_dur = sum(get_duration(c) for c in clips)
    print(f"\n  📊 {len(clips)} clips  ≈  {total_dur:.0f}s total")
    print("  🔗 Concatenating all clips...")

    cmd = [
        ffmpeg, "-y",
        "-f", "concat", "-safe", "0",
        "-i", str(concat_file),
        "-c:v", "libx264", "-preset", "slow", "-crf", "16",
        "-pix_fmt", "yuv420p", "-movflags", "+faststart",
        "-an",          # no audio track
        str(FINAL_VIDEO),
    ]
    result = subprocess.run(cmd, capture_output=True, text=True)
    if result.returncode != 0:
        print(f"  ⚠️  ffmpeg concat error:\n{result.stderr[-600:]}")
        sys.exit(1)

    if FINAL_VIDEO.exists():
        size_mb = FINAL_VIDEO.stat().st_size / 1024 / 1024
        print(f"\n✅ {FINAL_VIDEO}  ({size_mb:.1f} MB  •  {W}×{H}  •  {FPS}fps  •  ~{total_dur:.0f}s)")
    else:
        print("\n❌ Final video not created")
        sys.exit(1)


# ── Entry point ───────────────────────────────────────────────────────
def main():
    parser = argparse.ArgumentParser(description="Record Dominó Placar promo video")
    parser.add_argument(
        "--compose", action="store_true",
        help="Skip Playwright recording; reuse existing webm files in video-clips/"
    )
    args = parser.parse_args()

    os.chdir(Path(__file__).resolve().parent.parent)

    if args.compose:
        print("♻️  Compose-only mode — reusing existing webm files\n")
        webm_paths = find_existing_webms()
        if not webm_paths:
            print("❌ No webm files found. Run without --compose first.")
            sys.exit(1)
    else:
        webm_paths = record_all_scenes()

    compose_final(webm_paths)
    print("\n🎉 Done! Upload to YouTube and paste the URL in Google Play Console.")


if __name__ == "__main__":
    main()
