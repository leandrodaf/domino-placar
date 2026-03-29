# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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

[1.0.0]: https://github.com/leandrodaf/domino-placar/releases/tag/v1.0.0
