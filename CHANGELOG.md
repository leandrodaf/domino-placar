# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.1.2] - 2026-04-01

### Fixed

- **Peças da mão invisíveis em landscape** — `#my-hand` usava `height: 100%` com `overflow-y: auto` dentro de um flex-column cuja altura vem de `align-self: stretch`. Esse padrão colapsa para `0px` no Android Chrome/WebView. Corrigido usando `flex: 1; min-height: 0` e adicionando `overflow: hidden` ao `#hand-area`.
- **Botão de comprar (boneyard) não aparecia em landscape** — `display: none !important` no media query landscape impedia o JS de exibir o botão. Corrigido com posicionamento absoluto na base da coluna de peças.
- **Seletor de lado (esquerda/direita) não aparecia em landscape** — `display: none !important` foi removido; o seletor agora usa `position: fixed` centralizado.
- **Overlays não visíveis em tela cheia** — `#round-overlay`, `#gameover-overlay` e `#disconnect-banner` movidos para dentro de `#game-root` (a Fullscreen API só renderiza descendentes do elemento fullscreen).
- **Placar final não aparecia em tela cheia** — corrigido pelo mesmo reposicionamento dos overlays acima.
- **`#disconnect-banner` com gap de 56px** — o banner usava `top: 56px` compensando a nav, mas a nav está oculta na página do jogo. Corrigido para `top: 0`.
- **Controles de zoom flutuando no meio da mesa** — `right` revertido para `4px` (posição relativa à área do tabuleiro, não à página).

### Added

- **Badge de pontuação no header** — exibe a pontuação acumulada do jogador em tempo real diretamente no cabeçalho da partida.

[1.1.2]: https://github.com/leandrodaf/domino-placar/compare/v1.1.1...v1.1.2

## [1.1.1] - 2026-03-31

### Fixed

- **Game routes unavailable in Firebase mode** — SQLite is now always initialised at startup regardless of the active tournament store. Previously, when `FIREBASE_DATABASE_URL` was set the `store.(*db.SQLiteStore)` type assertion failed and all `/game/*` routes were silently skipped, causing `405 Method Not Allowed` / `404` responses in production.

[1.1.1]: https://github.com/leandrodaf/domino-placar/compare/v1.1.0...v1.1.1


### Added

- **Pixel-space board layout engine** — `layout_pixel.go` computes absolute `(X, Y, Rotation)` for every tile server-side; the frontend applies `translate(X,Y) rotate(Rotation°)` with zero extra math. Supports bidirectional vertical growth: right side curves down, left side curves up to prevent collision.
- **U-turn curve types** — `CurveEnter` / `CurveExit` tile roles produce a natural two-tile corner for every U-turn; `RenderedTile` exposes `CurveType`, `RowCount`, and `RowDoubles` hints for the renderer.
- **Multi-strategy bot AI** — three bot personalities: *Curinga* 🎲 (random), *Pedreira* 💪 (greedy), *Calculista* 🧠 (smart — plays doubles first, controls board ends, minimises hand sum). Think delays scale per strategy (400 ms – 2 s).
- **2–10 player sessions** — `playerTable` configures tiles-per-player for every group size so the standard 28-tile Double-6 deck is never exceeded.
- **Quickplay matchmaking** — `POST /game/quickplay` matches players into an existing waiting session by variant, or creates a new one automatically.
- **Solo vs Bots** — `POST /game/solo` creates a session, fills all remaining seats with bots, and starts immediately.
- **Dynamic bot selector in lobby** — host can choose how many bots to add via a `−` / count / `+` stepper; start button is disabled until ≥ 2 human players are present.
- **Lobby disconnect grace period** — 10-second grace period before removing a player from a waiting lobby, so page reloads don't accidentally eject players.
- **Active-game reconnect window** — 60-second reconnect window for in-progress games; a disconnect banner counts down for other players.
- **Host re-election** — if the host leaves the lobby, the first remaining participant is automatically promoted to host.
- **Explicit leave action** — `POST /game/{id}/leave` lets a player leave the lobby cleanly.
- **Bots preview** — `POST /game/{id}/bots-preview` returns bot assignments before the host starts, shown in the lobby UI.
- **Session resume** — on page reload, players are matched back to their waiting or active session.
- **Zombie session cleanup** — stale waiting/active sessions from previous server runs are purged at startup.
- **Quickplay variant pills** — home page shows variant pill buttons for one-tap quickplay entry.
- **`Eliminated` flag** — participants gain an `Eliminated` boolean (persisted in DB) for losers-pay game modes.
- **All-Fives rounding** — `roundPoints()` rounds raw pip-sum to the nearest multiple of 5 for All-Fives scoring.
- **Host-header injection fix** — `canonicalHost()` reads `CANONICAL_HOST` env var; `robots.txt` / `sitemap.xml` no longer derive the host from the request.
- **HTTP server hardening** — `ReadHeaderTimeout: 10s` and `IdleTimeout: 120s` guard against Slowloris attacks.

### Changed

- **Board rendering contract** — tiles now carry `x`, `y`, `rotation` instead of `orientation` / `flipped`; all layout logic lives exclusively in `layout_pixel.go`.
- **Lobby redesigned** — replaced the 4-slot seat grid with a scrollable player list supporting up to 10 participants.
- **`showToast()` and alert auto-dismiss** consolidated in `base.html`; no per-page duplication needed.
- **`ShuffleTiles()`** — switched to a package-level `rng` (seeded once at startup) to avoid predictable sequences from per-call seeding.
- **`Deal()`** — tile count derived from `playerTable`; the now-redundant second argument is ignored.
- **`FindOpenSession()`** — accepts a `variant` filter and supports sessions with 1–10 participants.
- **`SaveGameSession()`** — persists `host_unique_id`; `UpsertGameParticipant()` persists `eliminated`.

### Fixed

- **Insecure randomness (CWE-338)** — replaced `Math.random()` with `window.crypto.getRandomValues` for `unique_id` generation in `home.html` and `game_join.html`.
- **`android/gradlew` permission** — restored executable bit (`100755`) lost in a previous commit.
- **`package-lock.json` private registry** — replaced all `resolved` URLs pointing to the internal `furycloud.io` mirror with `registry.npmjs.org` so CI can install packages from the public registry.

### Removed

- **`layout.go` / `layout_test.go`** — dead code superseded by `layout_pixel.go`.
- **`dependency-review` CI job** — removed until the GitHub Dependency Graph feature is enabled on the repository (*Settings → Security Analysis*).

## [1.0.0] - 2026-03-29

### Added

- **Core game engine** — complete Pontinho (Brazilian domino) scoring with 51-point bust limit
- **Real-time updates** — Server-Sent Events (SSE) for instant score synchronization across all players
- **QR Code joining** — host shares a QR code so players can join the room instantly
- **Tile detection via photo** — snap a picture of remaining tiles and the system auto-detects them using Roboflow computer vision
- **Table photo analysis** — photograph the domino table to track played tiles
- **Multi-table tournaments** — tournament mode with automatic player-to-table allocation
- **Turmas (private groups)** — create groups with 6-character invite codes, shared rankings, and one-tap match creation
- **Android app** — native WebView wrapper with FCM push notifications, splash screen, deep links, and pull-to-refresh
- **Push notifications** — Firebase Cloud Messaging keeps background players updated on game events
- **Hall of Fame** — persistent global ranking with fun stats (bust records, close calls, pinto kings, branco kings)
- **Nickname system** — players can nominate and vote on nicknames for each other during matches
- **Internationalization** — Portuguese and English, auto-detected from the browser
- **Dual storage** — SQLite (WAL mode) for local development, Firebase Realtime Database for production
- **Cloud Storage** — Google Cloud Storage support for player photos (falls back to local `uploads/` folder)
- **Docker support** — production-ready Dockerfile for Cloud Run deployment
- **CI/CD pipeline** — GitHub Actions for Go lint/tests, Android build, ESLint, security scanning (Trivy, CodeQL), and automated releases via GoReleaser
- **Mobile-first design** — premium dark theme optimized for phone screens
- **Automatic tile distribution** — calculates tiles per player / draw pile based on player count (Double-6 or Double-9 sets)
- **Round rotation** — automatic starter player rotation across rounds
- **Score correction** — host can manually adjust player scores
- **CSRF protection** — per-match CSRF tokens on all form endpoints
- **Rate limiting** — IP-based rate limiting to prevent abuse
- **Data deletion** — LGPD/GDPR-compliant data deletion request page

[1.1.0]: https://github.com/leandrodaf/domino-placar/compare/v1.0.0...v1.1.0
[1.0.0]: https://github.com/leandrodaf/domino-placar/releases/tag/v1.0.0
