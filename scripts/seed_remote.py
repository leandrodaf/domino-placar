#!/usr/bin/env python3
"""
Create test match data on a running Dominó Placar server using Playwright.
Uses page.request.post() which inherits the browser's cookie jar automatically.

Usage:
  eval $(python3 scripts/seed_remote.py https://dominoplacar.net)
"""

import sys
import time
from playwright.sync_api import sync_playwright


def log(msg):
    print(f"  {msg}", file=sys.stderr)


def extract_match_id(url):
    return url.split("/match/")[1].split("/")[0].split("?")[0]

def extract_round_id(url):
    return url.split("/round/")[1].split("/")[0].split("?")[0]

def get_csrf(page):
    token = page.get_attribute('meta[name="csrf-token"]', "content")
    if not token:
        # fallback: grab from any hidden _csrf input
        token = page.get_attribute('input[name="_csrf"]', "value")
    return token or ""

def get_pid_from_url(url):
    if "player_id=" in url:
        return url.split("player_id=")[1].split("&")[0]
    return None


SCORES_R1 = [8,  15,  3, 22]
SCORES_R2 = [12,  6, 18,  5]
SCORES_R3 = [7,  10, 14,  9]

PLAYERS_ACTIVE   = [("Carlos","sc-c01"), ("Ana","sc-a01"), ("Roberto","sc-r01"), ("Dona Maria","sc-m01")]
PLAYERS_FINISHED = [("Roberto","sf-r01",27), ("Carlos","sf-c01",52), ("Ana","sf-a01",48), ("Dona Maria","sf-m01",53)]
PLAYERS_LOBBY    = [("Carlos","sl-c01"), ("Ana","sl-a01"), ("Roberto","sl-r01")]


def api_post(page, path, data):
    """POST form data using the page's cookie jar (inherits host/player cookies)."""
    resp = page.request.post(
        path if path.startswith("http") else f"https://dominoplacar.net{path}",
        form=data,
    )
    return resp.status, resp.headers.get("location", "")


def join_player(browser, base, match_id, name, unique_id):
    """Join one player in a fresh browser context. Returns player_id."""
    ctx  = browser.new_context()
    page = ctx.new_page()
    page.goto(f"{base}/match/{match_id}/join?lang=pt", wait_until="load")
    page.fill('input[name="name"]',      name)
    page.fill('input[name="unique_id"]', unique_id)
    page.click('button[type="submit"]')
    try:
        page.wait_for_url("**/waiting**", timeout=10000)
    except Exception:
        pass
    pid = get_pid_from_url(page.url)
    ctx.close()
    return pid


def goto_ranking(host_page, base, match_id):
    host_page.goto(f"{base}/match/{match_id}/ranking?lang=pt", wait_until="load")
    host_page.wait_for_timeout(500)


def create_match_and_join(browser, base, players_info):
    """
    Create a new match as host, join all players, return (host_page, host_ctx, match_id, pids).
    players_info: list of (name, unique_id) tuples.
    """
    ctx  = browser.new_context()
    page = ctx.new_page()
    page.goto(f"{base}/?lang=pt", wait_until="networkidle")
    # Click the "Nova Partida" create button
    page.click('form[action="/match"] button[type="submit"]')
    page.wait_for_url("**/lobby**", timeout=15000)
    mid = extract_match_id(page.url)

    pids = []
    for name, uid in players_info:
        pid = join_player(browser, base, mid, name, uid)
        pids.append(pid)
        time.sleep(0.3)

    return page, ctx, mid, pids


def start_match(host_page, base, mid):
    host_page.goto(f"{base}/match/{mid}/lobby?lang=pt", wait_until="load")
    csrf = get_csrf(host_page)
    status, _ = api_post(host_page, f"/match/{mid}/start", {"_csrf": csrf})
    time.sleep(1.0)
    return status


def create_round(host_page, base, mid):
    goto_ranking(host_page, base, mid)
    csrf = get_csrf(host_page)
    status, loc = api_post(host_page, f"/match/{mid}/round", {"_csrf": csrf})
    time.sleep(0.8)
    # Navigate to ranking to pick up the new round link
    goto_ranking(host_page, base, mid)
    # Extract round ID from the "Ver rodada" link
    rid_link = host_page.get_attribute('a[href*="/round/"]', "href")
    if rid_link:
        return extract_round_id(rid_link)
    # Fallback: from the form action
    form_action = host_page.get_attribute('form[action*="/round/"][action*="/starter/"]', "action") or \
                  host_page.get_attribute('form[action*="/round/"][action*="/winner/"]', "action") or ""
    if "/round/" in form_action:
        return extract_round_id(form_action)
    return None


def set_starter(host_page, base, mid, rid, pid):
    goto_ranking(host_page, base, mid)
    csrf = get_csrf(host_page)
    api_post(host_page, f"/match/{mid}/round/{rid}/starter/{pid}", {"_csrf": csrf})
    time.sleep(0.4)


def set_winner(host_page, base, mid, rid, pid):
    goto_ranking(host_page, base, mid)
    csrf = get_csrf(host_page)
    api_post(host_page, f"/match/{mid}/round/{rid}/winner/{pid}", {"_csrf": csrf})
    time.sleep(0.4)


def set_score(host_page, base, mid, pid, score):
    goto_ranking(host_page, base, mid)
    csrf = get_csrf(host_page)
    api_post(host_page, f"/match/{mid}/player/{pid}/score", {"_csrf": csrf, "score": str(score)})
    time.sleep(0.3)


def finish_match(host_page, base, mid):
    goto_ranking(host_page, base, mid)
    csrf = get_csrf(host_page)
    api_post(host_page, f"/match/{mid}/finish", {"_csrf": csrf})
    time.sleep(0.8)


def seed_all(base_url):
    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)

        # ── 1. Lobby match (waiting) ──────────────────────────────────
        log("Creating lobby match…")
        host_page, ctx_l, lid, _ = create_match_and_join(
            browser, base_url, PLAYERS_LOBBY)
        ctx_l.close()
        log(f"  LID = {lid}")

        # ── 2. Active match (3 finished rounds + 1 active) ────────────
        log("Creating active match…")
        host_page, ctx_m, mid, pids = create_match_and_join(
            browser, base_url, [(n, u) for n, u in PLAYERS_ACTIVE])
        log(f"  MID = {mid}")

        s = start_match(host_page, base_url, mid)
        log(f"  start_match HTTP {s}")

        cumulative = [0, 0, 0, 0]
        for rn, scores in enumerate([SCORES_R1, SCORES_R2, SCORES_R3], 1):
            rid = create_round(host_page, base_url, mid)
            if not rid:
                log(f"  ⚠️  Could not get round ID for round {rn}")
                continue
            log(f"  round {rn} RID = {rid}")

            winner_idx = scores.index(min(scores))
            set_starter(host_page, base_url, mid, rid, pids[rn % len(pids)])
            set_winner(host_page, base_url, mid, rid, pids[winner_idx])

            for i, s in enumerate(scores):
                cumulative[i] += s
                set_score(host_page, base_url, mid, pids[i], cumulative[i])

            log(f"  round {rn} done — winner: {PLAYERS_ACTIVE[winner_idx][0]}")

        # Create round 4 (active, not yet finished)
        r4id = create_round(host_page, base_url, mid)
        log(f"  R4ID = {r4id}")
        if r4id:
            set_starter(host_page, base_url, mid, r4id, pids[0])
        ctx_m.close()

        # ── 3. Finished match ─────────────────────────────────────────
        log("Creating finished match…")
        host_page, ctx_f, fid, fpids = create_match_and_join(
            browser, base_url, [(n, u) for n, u, _ in PLAYERS_FINISHED])
        log(f"  FID = {fid}")

        s = start_match(host_page, base_url, fid)
        log(f"  start_match HTTP {s}")

        frid = create_round(host_page, base_url, fid)
        if frid:
            set_starter(host_page, base_url, fid, frid, fpids[0])
            set_winner(host_page, base_url, fid, frid, fpids[0])

        for i, (_, _, score) in enumerate(PLAYERS_FINISHED):
            set_score(host_page, base_url, fid, fpids[i], score)

        finish_match(host_page, base_url, fid)
        log(f"  FID finished")
        ctx_f.close()

        browser.close()

    return mid, r4id or "unknown", fid, lid


def main():
    if len(sys.argv) < 2:
        print(f"Usage: eval $(python3 {sys.argv[0]} <SERVER_URL>)", file=sys.stderr)
        sys.exit(1)

    base_url = sys.argv[1].rstrip("/")
    print(f"Seeding {base_url}…", file=sys.stderr)

    try:
        mid, r4id, fid, lid = seed_all(base_url)
    except Exception as e:
        import traceback
        traceback.print_exc()
        sys.exit(1)

    print(f"export MID={mid}")
    print(f"export R4ID={r4id}")
    print(f"export FID={fid}")
    print(f"export LID={lid}")
    print(f"export SERVER_URL={base_url}")

    print("\nDone! Now run:", file=sys.stderr)
    print("  python3 scripts/take_screenshots.py", file=sys.stderr)
    print("  python3 scripts/record_promo_video.py", file=sys.stderr)


if __name__ == "__main__":
    main()
