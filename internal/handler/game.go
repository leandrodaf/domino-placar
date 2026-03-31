package handler

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/leandrodaf/domino-placar/internal/db"
	"github.com/leandrodaf/domino-placar/internal/game"
)

// ─── Session Manager ──────────────────────────────────────────────────────────

// GameSessionManager caches active game sessions in memory, backed by DB.
type GameSessionManager struct {
	mu       sync.RWMutex
	sessions map[string]*game.GameSession
	store    db.GameStore

	// pendingRemoval debounces lobby disconnects so page reloads don't remove players.
	pendingMu      sync.Mutex
	pendingRemoval map[string]*time.Timer // key: sessionID+":"+uid

	// disconnectTimers tracks active-game disconnect timers (1-minute reconnect window).
	disconnectMu    sync.Mutex
	disconnectTimers map[string]*time.Timer // key: sessionID+":"+uid
}

func NewGameSessionManager(store db.GameStore) *GameSessionManager {
	mgr := &GameSessionManager{
		sessions:         make(map[string]*game.GameSession),
		store:            store,
		pendingRemoval:   make(map[string]*time.Timer),
		disconnectTimers: make(map[string]*time.Timer),
	}
	// Clean up zombie sessions from previous runs.
	if n, err := store.CleanupZombieWaitingSessions(); err != nil {
		log.Printf("[startup] cleanup zombie waiting sessions error: %v", err)
	} else if n > 0 {
		log.Printf("[startup] cleaned up %d zombie waiting sessions", n)
	}
	if n, err := store.CleanupZombieActiveSessions(); err != nil {
		log.Printf("[startup] cleanup zombie active sessions error: %v", err)
	} else if n > 0 {
		log.Printf("[startup] cleaned up %d zombie active sessions", n)
	}
	go mgr.evictLoop()
	return mgr
}

// scheduleLobbyRemoval removes a player from a waiting lobby after a grace period.
// If the player reconnects before the timer fires (e.g., page reload), cancelLobbyRemoval
// stops the timer and the player stays in the session.
// Grace period is 10s to handle slow connections triggering reload via game_joined SSE.
func (m *GameSessionManager) scheduleLobbyRemoval(sessionID, uid, channel string, hub *SSEHub) {
	key := sessionID + ":" + uid
	m.pendingMu.Lock()
	// Cancel any previous timer for this player (shouldn't happen, but be safe).
	if old, ok := m.pendingRemoval[key]; ok {
		old.Stop()
	}
	log.Printf("[lobby] scheduled removal uid=%s session=%s (10s grace)", uid, sessionID)
	m.pendingRemoval[key] = time.AfterFunc(10*time.Second, func() {
		m.pendingMu.Lock()
		if _, stillPending := m.pendingRemoval[key]; !stillPending {
			m.pendingMu.Unlock()
			log.Printf("[lobby] removal cancelled (player reconnected) uid=%s session=%s", uid, sessionID)
			return // was cancelled — player reconnected
		}
		delete(m.pendingRemoval, key)
		m.pendingMu.Unlock()

		gs := m.Get(sessionID)
		if gs == nil || gs.Status != game.SessionWaiting {
			log.Printf("[lobby] removal skipped (session gone or not waiting) uid=%s session=%s", uid, sessionID)
			return
		}
		if removed, remaining := gs.RemoveParticipant(uid); removed {
			log.Printf("[lobby] LEFT uid=%s session=%s | remaining=%d players", uid, sessionID, remaining)
			_ = m.store.RemoveGameParticipant(sessionID, uid)
			// Persist the updated session metadata (e.g., new HostID after election).
			m.Persist(gs)
			hub.Broadcast(channel, "game_joined")
			log.Printf("[lobby] broadcast player_left -> session=%s remaining=%d", sessionID, remaining)
		}
	})
	m.pendingMu.Unlock()
}

// cancelLobbyRemoval stops a pending removal — called when a player reconnects.
func (m *GameSessionManager) cancelLobbyRemoval(sessionID, uid string) {
	key := sessionID + ":" + uid
	m.pendingMu.Lock()
	if t, ok := m.pendingRemoval[key]; ok {
		t.Stop()
		delete(m.pendingRemoval, key)
		log.Printf("[lobby] removal cancelled (reconnect) uid=%s session=%s", uid, sessionID)
	}
	m.pendingMu.Unlock()
}

// scheduleGameAbandonment starts a 60-second timer for an active-game disconnect.
// If the player reconnects (cancelGameAbandonment), the timer is cancelled.
// If the timer fires, the session is marked finished and all players are notified.
func (m *GameSessionManager) scheduleGameAbandonment(sessionID, uid, channel string, hub *SSEHub) {
	key := sessionID + ":" + uid
	m.disconnectMu.Lock()
	// Race guard: if the player already reconnected (cancelGameAbandonment ran before us
	// because the new SSE opened between our Unsubscribe and this call), skip scheduling.
	// hub.IsOnline uses its own lock (hub.mu) which is independent from disconnectMu.
	if hub.IsOnline(channel, uid) {
		if old, ok := m.disconnectTimers[key]; ok {
			old.Stop()
			delete(m.disconnectTimers, key)
		}
		m.disconnectMu.Unlock()
		log.Printf("[game] abandonment skipped (uid already online) uid=%s session=%s", uid, sessionID)
		return
	}
	if old, ok := m.disconnectTimers[key]; ok {
		old.Stop()
	}
	log.Printf("[game] scheduled abandonment uid=%s session=%s (60s window)", uid, sessionID)
	m.disconnectTimers[key] = time.AfterFunc(60*time.Second, func() {
		m.disconnectMu.Lock()
		if _, stillPending := m.disconnectTimers[key]; !stillPending {
			m.disconnectMu.Unlock()
			log.Printf("[game] abandonment cancelled (player reconnected) uid=%s session=%s", uid, sessionID)
			return
		}
		delete(m.disconnectTimers, key)
		m.disconnectMu.Unlock()

		gs := m.Get(sessionID)
		if gs == nil || gs.Status != game.SessionActive {
			log.Printf("[game] abandonment skipped (session gone or not active) uid=%s session=%s", uid, sessionID)
			return
		}
		log.Printf("[game] abandoning session=%s due to uid=%s not reconnecting", sessionID, uid)
		gs.Lock()
		gs.Status = game.SessionFinished
		gs.Unlock()
		m.Persist(gs)
		hub.Broadcast(channel, "game_finished")
	})
	m.disconnectMu.Unlock()
}


// cancelGameAbandonment stops a pending abandonment timer — called when the player reconnects.
// If hub is non-nil and the session is still active, broadcasts "game_player_back" so that
// remaining players can dismiss any disconnect countdown overlay.
func (m *GameSessionManager) cancelGameAbandonment(sessionID, uid string, hub *SSEHub) {
	key := sessionID + ":" + uid
	m.disconnectMu.Lock()
	_, wasActive := m.disconnectTimers[key]
	if wasActive {
		m.disconnectTimers[key].Stop()
		delete(m.disconnectTimers, key)
		log.Printf("[game] abandonment cancelled (reconnect) uid=%s session=%s", uid, sessionID)
	}
	m.disconnectMu.Unlock()

	if wasActive && hub != nil {
		gs := m.Get(sessionID)
		if gs != nil {
			gs.Lock()
			active := gs.Status == game.SessionActive
			playerName := ""
			for _, p := range gs.Participants {
				if p.UniqueID == uid {
					playerName = p.Name
					break
				}
			}
			gs.Unlock()
			if active {
				hub.Broadcast("game:"+sessionID, "game_player_back:"+playerName)
			}
		}
	}
}

// evictLoop periodically removes finished and abandoned sessions from the
// in-memory cache and cleans up zombie waiting sessions (0 participants) from DB.
func (m *GameSessionManager) evictLoop() {
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		// Collect sessions to persist outside the map lock to avoid holding
		// m.mu during DB writes (which would block all handlers).
		var zombies []*game.GameSession

		m.mu.Lock()
		for id, gs := range m.sessions {
			if gs.IsFinished() || gs.IsAbandoned() {
				delete(m.sessions, id)
			} else {
				// Check zombie status under gs.mu to avoid a data race.
				gs.Lock()
				isZombie := gs.Status == game.SessionWaiting && len(gs.Participants) == 0
				if isZombie {
					gs.Status = game.SessionFinished
					zombies = append(zombies, gs)
					delete(m.sessions, id)
				}
				gs.Unlock()
			}
		}
		m.mu.Unlock()

		// Persist zombie sessions outside the map lock.
		for _, gs := range zombies {
			_ = m.store.SaveGameSession(gs)
			log.Printf("[evict] zombie waiting session=%s marked finished", gs.ID)
		}
	}
}

func (m *GameSessionManager) Get(id string) *game.GameSession {
	m.mu.RLock()
	gs := m.sessions[id]
	m.mu.RUnlock()
	if gs != nil {
		return gs
	}
	gs, err := m.store.LoadGameSession(id)
	if err != nil {
		return nil
	}
	m.mu.Lock()
	m.sessions[id] = gs
	m.mu.Unlock()
	return gs
}

func (m *GameSessionManager) Set(gs *game.GameSession) {
	m.mu.Lock()
	m.sessions[gs.ID] = gs
	m.mu.Unlock()
}

func (m *GameSessionManager) Persist(gs *game.GameSession) {
	// Acquire the session lock so we read a consistent snapshot of all fields.
	// DB writes are fast (WAL SQLite), so the brief hold is acceptable.
	gs.Lock()
	defer gs.Unlock()
	if err := m.store.SaveGameSession(gs); err != nil {
		log.Printf("persist game session %s: %v", gs.ID, err)
	}
	for _, p := range gs.Participants {
		if err := m.store.UpsertGameParticipant(gs.ID, p); err != nil {
			log.Printf("persist participant %s: %v", p.ID, err)
		}
	}
}

// ─── Cookie helpers ──────────────────────────────────────────────────────────

func getGamePlayerID(r *http.Request, sessionID string) string {
	c, err := r.Cookie("domino_gp_" + sessionID)
	if err != nil {
		return ""
	}
	dot := strings.IndexByte(c.Value, '.')
	if dot < 0 {
		return ""
	}
	pid, sig := c.Value[:dot], c.Value[dot+1:]
	if !macEqual(sig, macSign("gp:"+sessionID+":"+pid)) {
		return ""
	}
	return pid
}

func setGamePlayerCookie(w http.ResponseWriter, sessionID, participantID string) {
	tok := participantID + "." + macSign("gp:"+sessionID+":"+participantID)
	http.SetCookie(w, &http.Cookie{
		Name:     "domino_gp_" + sessionID,
		Value:    tok,
		Path:     "/",
		MaxAge:   86400 * 7,
		HttpOnly: true,
		Secure:   secureCookies,
		SameSite: http.SameSiteLaxMode,
	})
}

// ─── Bot automation ──────────────────────────────────────────────────────────

// scheduleBotMoves triggers bot turns via goroutine after a human action.
// Each iteration sleeps to simulate thinking, then calls ExecuteBotTurn which
// reads state and executes the move under a single lock — eliminating the race
// between decision and execution, and preventing silent stalls on error.
// maxBotIter caps the number of consecutive bot moves in a single goroutine
// invocation, preventing goroutine leaks if the game reaches a pathological state.
const maxBotIter = 500

func scheduleBotMoves(gs *game.GameSession, mgr *GameSessionManager, hub *SSEHub) {
	go func() {
		channel := "game:" + gs.ID
		for iter := 0; iter < maxBotIter; iter++ {
			// Quick pre-check to bail early before sleeping if no bot turn is pending.
			// CurrentPlayer acquires the lock internally; IsFinished does too.
			cp := gs.CurrentPlayer()
			if cp == nil || !cp.IsBot || gs.IsFinished() {
				return
			}

			time.Sleep(game.BotThinkDelay(cp.BotStrategy))

			move, result, err := gs.ExecuteBotTurn()
			if err != nil {
				// Not a bot turn, session ended, or truly invalid state —
				// stop the loop; the next human action will re-trigger if needed.
				log.Printf("bot turn skipped (%s): %v", gs.ID, err)
				return
			}

			moveID := uuid.New().String()

			switch move.Type {
			case game.MovePlay:
				_ = mgr.store.RecordGameMove(moveID, gs.ID, "", result.RoundNumber, move, 0)
				hub.Broadcast(channel, "game_move_played")
			case game.MoveDraw:
				hub.Broadcast(channel, "game_turn_changed")
				// Turn still belongs to this bot — loop to let it play.
				continue
			case game.MovePass:
				_ = mgr.store.RecordGameMove(moveID, gs.ID, "", result.RoundNumber, move, 0)
				hub.Broadcast(channel, "game_turn_changed")
			}

			mgr.Persist(gs)

			if result.Ended {
				hub.Broadcast(channel, "game_round_ended")
				if result.SessionFinished {
					hub.Broadcast(channel, "game_finished")
					return
				}
				time.Sleep(8 * time.Second)
				if gs.IsFinished() {
					return
				}
				gs.StartNextRound()
				mgr.Persist(gs)
				hub.Broadcast(channel, "game_started")
				continue
			}

			// Stop if the next player is human; they will re-trigger the loop.
			next := gs.CurrentPlayer()
			if next == nil || !next.IsBot {
				hub.Broadcast(channel, "game_turn_changed")
				return
			}
		}
		log.Printf("bot loop for session %s hit max iterations (%d), terminating", gs.ID, maxBotIter)
	}()
}

// ─── Create / Lobby / Join / Start ──────────────────────────────────────────

// CreateGamePageHandler handles GET /game/new — redirects to home with the game panel open.
func CreateGamePageHandler(tmpl *Templates) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/?game=1", http.StatusSeeOther)
	}
}

// CreateGameHandler handles POST /game — creates a new online game session.
func CreateGameHandler(mgr *GameSessionManager, hub *SSEHub, tmpl *Templates) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !CheckActionRateLimit(r) {
			http.Error(w, "too many requests", http.StatusTooManyRequests)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		variant := r.FormValue("variant")
		if _, ok := game.Variants[variant]; !ok {
			variant = "pontinho"
		}

		maxPoints := defaultGameMaxPoints(variant)
		if mp, err := strconv.Atoi(r.FormValue("max_points")); err == nil && mp >= 10 && mp <= 500 {
			maxPoints = mp
		}

		teamMode := r.FormValue("team_mode") == "1"
		name := SanitizeInput(r.FormValue("player_name"), 50)
		if name == "" {
			name = "Jogador 1"
		}
		uniqueID := SanitizeInput(r.FormValue("unique_id"), 50)
		if uniqueID == "" {
			uniqueID = uuid.New().String()
		}

		sessionID := uuid.New().String()
		gs := game.NewGameSession(sessionID, variant, maxPoints, teamMode, uniqueID)

		host := &game.Participant{
			ID:       uuid.New().String(),
			Name:     name,
			UniqueID: uniqueID,
			Seat:     0,
			Team:     0,
			IsBot:    false,
		}
		if err := gs.AddParticipant(host); err != nil {
			http.Error(w, "failed to create game", http.StatusInternalServerError)
			return
		}

		if err := mgr.store.CreateGameSession(gs); err != nil {
			log.Printf("CreateGameSession: %v", err)
			http.Error(w, "failed to create game", http.StatusInternalServerError)
			return
		}
		if err := mgr.store.UpsertGameParticipant(sessionID, host); err != nil {
			log.Printf("UpsertGameParticipant: %v", err)
		}
		mgr.Set(gs)

		log.Printf("[create] session=%s uid=%s variant=%s", sessionID, uniqueID, variant)
		SetHostCookie(w, sessionID)
		setGamePlayerCookie(w, sessionID, host.ID)
		http.Redirect(w, r, "/game/"+sessionID+"/lobby", http.StatusSeeOther)
	}
}

// GameLobbyHandler handles GET /game/{id}/lobby — the waiting room.
func GameLobbyHandler(mgr *GameSessionManager, tmpl *Templates) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID := r.PathValue("id")
		gs := mgr.Get(sessionID)
		if gs == nil {
			http.Error(w, "game not found", http.StatusNotFound)
			return
		}

		if gs.Status == game.SessionActive || gs.Status == game.SessionFinished {
			http.Redirect(w, r, "/game/"+sessionID+"/play", http.StatusSeeOther)
			return
		}

		variantInfo := game.GetVariant(gs.Variant)
		myPID := getGamePlayerID(r, sessionID)

		// Auto-grant host cookie when a new host has been elected (e.g. original host left).
		isHost := IsHost(r, sessionID)
		if !isHost && myPID != "" {
			if p := gs.FindParticipantByID(myPID); p != nil && p.UniqueID == gs.HostID {
				SetHostCookie(w, sessionID)
				isHost = true
			}
		}

		humanCount := len(gs.Participants)
		maxBots := 10 - humanCount

		tmpl.Render(w, r, "game_lobby.html", map[string]any{
			"Session":      gs,
			"Participants": gs.Participants,
			"HumanCount":   humanCount,
			"MaxBots":      maxBots,
			"IsHost":       isHost,
			"MyPID":        myPID,
			"VariantName":  variantInfo.Name,
			"CSRFToken":    GenerateCSRFToken(sessionID),
			"IsQuickPlay":  gs.QuickPlay,
		})
	}
}

// GameJoinHandler handles POST /game/{id}/join — adds a player to the lobby.
func GameJoinHandler(mgr *GameSessionManager, hub *SSEHub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID := r.PathValue("id")
		gs := mgr.Get(sessionID)
		if gs == nil {
			http.Error(w, "game not found", http.StatusNotFound)
			return
		}
		if !CheckActionRateLimit(r) {
			http.Error(w, "too many requests", http.StatusTooManyRequests)
			return
		}
		if err := r.ParseForm(); err != nil || !ValidateCSRFToken(r.FormValue("_csrf"), sessionID) {
			http.Error(w, "invalid security token", http.StatusForbidden)
			return
		}

		name := SanitizeInput(r.FormValue("player_name"), 50)
		if name == "" {
			name = "Jogador"
		}
		uniqueID := SanitizeInput(r.FormValue("unique_id"), 50)
		if uniqueID == "" {
			uniqueID = uuid.New().String()
		}

		// Already in game? Restore cookie and redirect to the right page.
		if existing := gs.FindParticipant(uniqueID); existing != nil {
			log.Printf("[join] resume uid=%s session=%s status=%s", uniqueID, sessionID, gs.Status)
			setGamePlayerCookie(w, sessionID, existing.ID)
			if gs.Status == game.SessionActive {
				http.Redirect(w, r, "/game/"+sessionID+"/play", http.StatusSeeOther)
			} else {
				http.Redirect(w, r, "/game/"+sessionID+"/lobby", http.StatusSeeOther)
			}
			return
		}

		seat := len(gs.Participants)
		p := &game.Participant{
			ID:       uuid.New().String(),
			Name:     name,
			UniqueID: uniqueID,
			Seat:     seat,
			Team:     seat % 2,
			IsBot:    false,
		}
		if err := gs.AddParticipant(p); err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		if err := mgr.store.UpsertGameParticipant(sessionID, p); err != nil {
			log.Printf("UpsertGameParticipant: %v", err)
		}

		log.Printf("[join] JOINED uid=%s name=%s session=%s | players=%d", uniqueID, name, sessionID, len(gs.Participants))
		setGamePlayerCookie(w, sessionID, p.ID)
		hub.Broadcast("game:"+sessionID, "game_joined")
		http.Redirect(w, r, "/game/"+sessionID+"/lobby", http.StatusSeeOther)
	}
}

// GameStartHandler handles POST /game/{id}/start — starts the game.
func GameStartHandler(mgr *GameSessionManager, hub *SSEHub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID := r.PathValue("id")
		if !IsHost(r, sessionID) {
			http.Error(w, "access denied", http.StatusForbidden)
			return
		}
		gs := mgr.Get(sessionID)
		if gs == nil {
			http.Error(w, "game not found", http.StatusNotFound)
			return
		}
		if err := r.ParseForm(); err != nil || !ValidateCSRFToken(r.FormValue("_csrf"), sessionID) {
			http.Error(w, "invalid security token", http.StatusForbidden)
			return
		}

		botCount, _ := strconv.Atoi(r.FormValue("bot_count"))
		if botCount < 0 {
			botCount = 0
		}
		if botCount > 9 {
			botCount = 9
		}
		if err := gs.StartGame(botCount); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		log.Printf("[start] session=%s bots=%d players=%d", sessionID, botCount, len(gs.Participants))
		mgr.Persist(gs)
		hub.Broadcast("game:"+sessionID, "game_started")

		if cp := gs.CurrentPlayer(); cp != nil && cp.IsBot {
			scheduleBotMoves(gs, mgr, hub)
		}

		http.Redirect(w, r, "/game/"+sessionID+"/play", http.StatusSeeOther)
	}
}

// GameJoinPageHandler handles GET /game/{id}/join — shows the join form.
func GameJoinPageHandler(mgr *GameSessionManager, tmpl *Templates) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID := r.PathValue("id")
		gs := mgr.Get(sessionID)
		if gs == nil {
			http.Error(w, "game not found", http.StatusNotFound)
			return
		}
		if gs.Status != game.SessionWaiting {
			http.Redirect(w, r, "/game/"+sessionID+"/play", http.StatusSeeOther)
			return
		}
		variantInfo := game.GetVariant(gs.Variant)
		tmpl.Render(w, r, "game_join.html", map[string]any{
			"Session":     gs,
			"VariantName": variantInfo.Name,
			"CSRFToken":   GenerateCSRFToken(sessionID),
		})
	}
}

// GameLeaveHandler handles POST /game/{id}/leave — explicit lobby departure.
// Unlike navigating away (which schedules a 10s grace-period removal), this
// removes the player immediately and redirects to home. Idempotent: safe to call
// even if the player is not in the session.
func GameLeaveHandler(mgr *GameSessionManager, hub *SSEHub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID := r.PathValue("id")
		if err := r.ParseForm(); err != nil || !ValidateCSRFToken(r.FormValue("_csrf"), sessionID) {
			http.Error(w, "invalid security token", http.StatusForbidden)
			return
		}
		participantID := getGamePlayerID(r, sessionID)

		gs := mgr.Get(sessionID)
		if gs != nil && gs.Status == game.SessionWaiting && participantID != "" {
			p := gs.FindParticipantByID(participantID)
			if p != nil {
				uid := p.UniqueID
				mgr.cancelLobbyRemoval(sessionID, uid) // stop any running timer
				if removed, remaining := gs.RemoveParticipant(uid); removed {
					log.Printf("[lobby] EXPLICIT LEAVE uid=%s session=%s | remaining=%d", uid, sessionID, remaining)
					_ = mgr.store.RemoveGameParticipant(sessionID, uid)
					if remaining == 0 {
						gs.Lock()
						gs.Status = game.SessionFinished
						gs.Unlock()
						log.Printf("[lobby] session=%s marked finished (no players left)", sessionID)
					}
					mgr.Persist(gs)
					hub.Broadcast("game:"+sessionID, "game_joined")
				}
			}
		}

		// Clear the participant cookie for this session.
		http.SetCookie(w, &http.Cookie{
			Name:   "domino_gp_" + sessionID,
			Value:  "",
			Path:   "/",
			MaxAge: -1,
		})
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

// defaultGameMaxPoints returns a sensible default max points for a variant.
func defaultGameMaxPoints(variant string) int {
	switch variant {
	case "pontinho":
		return 51
	case "bloqueio":
		return 100
	case "all_fives":
		return 150
	case "com_pedra":
		return 100
	}
	return 51
}

// ─── Quick Play (matchmaking) ─────────────────────────────────────────────────

// QuickPlayHandler handles POST /game/quickplay.
// It finds an open waiting session and joins it, or creates a new one.
func QuickPlayHandler(mgr *GameSessionManager, hub *SSEHub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !CheckActionRateLimit(r) {
			http.Error(w, "too many requests", http.StatusTooManyRequests)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		name := SanitizeInput(r.FormValue("player_name"), 50)
		if name == "" {
			name = "Jogador"
		}
		uniqueID := SanitizeInput(r.FormValue("unique_id"), 50)
		if uniqueID == "" {
			uniqueID = uuid.New().String()
		}
		variant := r.FormValue("variant")
		if _, ok := game.Variants[variant]; !ok {
			variant = "pontinho"
		}
		maxPoints := defaultGameMaxPoints(variant)
		if mp, err := strconv.Atoi(r.FormValue("max_points")); err == nil && mp >= 10 && mp <= 500 {
			maxPoints = mp
		}

		// 0. Check if already in an ACTIVE (in-progress) game — reconnect immediately.
		if activeID, _ := mgr.store.FindMyActiveSession(uniqueID); activeID != "" {
			activeGS := mgr.Get(activeID)
			if activeGS != nil && activeGS.Status == game.SessionActive {
				if ex := activeGS.FindParticipant(uniqueID); ex != nil {
					log.Printf("[quickplay] reconnect active game uid=%s session=%s", uniqueID, activeID)
					mgr.cancelGameAbandonment(activeID, uniqueID, hub)
					setGamePlayerCookie(w, activeID, ex.ID)
					http.Redirect(w, r, "/game/"+activeID+"/play", http.StatusSeeOther)
					return
				}
			}
		}

		// 1. Check if already in a session with OTHER players (e.g. brief disconnect).
		//    Only resume if there are at least 2 humans — solo sessions should not block
		//    matchmaking with other waiting players.
		myID, _ := mgr.store.FindMyWaitingSession(uniqueID, variant)
		var myExParticipant *game.Participant
		if myID != "" {
			myGS := mgr.Get(myID)
			if myGS != nil && myGS.Status == game.SessionWaiting {
				if ex := myGS.FindParticipant(uniqueID); ex != nil {
					humanCount := 0
					for _, p := range myGS.Participants {
						if !p.IsBot {
							humanCount++
						}
					}
					if humanCount > 1 {
						// Has company — always resume this session.
						log.Printf("[quickplay] resume (with company) uid=%s session=%s humans=%d", uniqueID, myID, humanCount)
						mgr.cancelLobbyRemoval(myID, uniqueID)
						setGamePlayerCookie(w, myID, ex.ID)
						http.Redirect(w, r, "/game/"+myID+"/lobby", http.StatusSeeOther)
						return
					}
					// Solo session — remember it as fallback after matchmaking.
					log.Printf("[quickplay] solo session found uid=%s session=%s, trying matchmaking first", uniqueID, myID)
					myExParticipant = ex
				}
			}
		}

		// 2. Find an open session this player has NOT already joined.
		openID, err := mgr.store.FindOpenSession(uniqueID, variant)
		if err != nil {
			log.Printf("FindOpenSession: %v", err)
		}


		if openID != "" {
			// Join the existing session
			gs := mgr.Get(openID)
			if gs != nil && gs.Status == game.SessionWaiting {
				// Already joined? Just restore
				if ex := gs.FindParticipant(uniqueID); ex != nil {
					setGamePlayerCookie(w, openID, ex.ID)
					http.Redirect(w, r, "/game/"+openID+"/lobby", http.StatusSeeOther)
					return
				}
				seat := len(gs.Participants)
				p := &game.Participant{
					ID:       uuid.New().String(),
					Name:     name,
					UniqueID: uniqueID,
					Seat:     seat,
					Team:     seat % 2,
				}
				if addErr := gs.AddParticipant(p); addErr == nil {
					log.Printf("[quickplay] JOINED uid=%s session=%s | players=%d", uniqueID, openID, len(gs.Participants))
					// Clean up own solo session so it doesn't become a zombie.
					// The SSE removal timer may not be running if the player never
					// established an SSE connection to their old session.
					if myID != "" && myExParticipant != nil {
						log.Printf("[quickplay] cleanup solo session=%s uid=%s", myID, uniqueID)
						mgr.cancelLobbyRemoval(myID, uniqueID)
						if myGS := mgr.Get(myID); myGS != nil {
							myGS.RemoveParticipant(uniqueID) // discard (bool,int) return
							_ = mgr.store.RemoveGameParticipant(myID, uniqueID)
						}
					}
					_ = mgr.store.UpsertGameParticipant(openID, p)
					setGamePlayerCookie(w, openID, p.ID)
					hub.Broadcast("game:"+openID, "game_joined")
					http.Redirect(w, r, "/game/"+openID+"/lobby", http.StatusSeeOther)
					return
				}
			}
			// Session disappeared or full — fall through
		}

		// 3. No open session found — resume own solo session if it still exists.
		//    Re-fetch to guard against the removal timer firing between steps 1 and 3.
		if myID != "" {
			myGS := mgr.Get(myID)
			if myGS != nil && myGS.Status == game.SessionWaiting {
				if ex := myGS.FindParticipant(uniqueID); ex != nil {
					log.Printf("[quickplay] resume (solo fallback) uid=%s session=%s", uniqueID, myID)
					mgr.cancelLobbyRemoval(myID, uniqueID)
					setGamePlayerCookie(w, myID, ex.ID)
					http.Redirect(w, r, "/game/"+myID+"/lobby", http.StatusSeeOther)
					return
				}
			}
			log.Printf("[quickplay] solo session expired uid=%s session=%s, creating new", uniqueID, myID)
		}

		// Create a new session with the requested variant
		sessionID := uuid.New().String()
		gs := game.NewGameSession(sessionID, variant, maxPoints, false, uniqueID)
		gs.QuickPlay = true
		host := &game.Participant{
			ID:       uuid.New().String(),
			Name:     name,
			UniqueID: uniqueID,
			Seat:     0,
			Team:     0,
		}
		if addErr := gs.AddParticipant(host); addErr != nil {
			http.Error(w, "failed to create game", http.StatusInternalServerError)
			return
		}
		if err := mgr.store.CreateGameSession(gs); err != nil {
			log.Printf("QuickPlay CreateGameSession: %v", err)
			http.Error(w, "failed to create game", http.StatusInternalServerError)
			return
		}
		log.Printf("[quickplay] created session=%s uid=%s variant=%s", sessionID, uniqueID, variant)
		_ = mgr.store.UpsertGameParticipant(sessionID, host)
		mgr.Set(gs)
		SetHostCookie(w, sessionID)
		setGamePlayerCookie(w, sessionID, host.ID)
		http.Redirect(w, r, "/game/"+sessionID+"/lobby", http.StatusSeeOther)
	}
}

// SoloPlayHandler handles POST /game/solo — instantly starts a game against bots.
func SoloPlayHandler(mgr *GameSessionManager, hub *SSEHub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !CheckActionRateLimit(r) {
			http.Error(w, "too many requests", http.StatusTooManyRequests)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		name := SanitizeInput(r.FormValue("player_name"), 50)
		if name == "" {
			name = "Jogador"
		}
		uniqueID := SanitizeInput(r.FormValue("unique_id"), 50)
		if uniqueID == "" {
			uniqueID = uuid.New().String()
		}
		variant := r.FormValue("variant")
		if _, ok := game.Variants[variant]; !ok {
			variant = "pontinho"
		}
		maxPoints := defaultGameMaxPoints(variant)
		if mp, err := strconv.Atoi(r.FormValue("max_points")); err == nil && mp >= 10 && mp <= 500 {
			maxPoints = mp
		}

		sessionID := uuid.New().String()
		gs := game.NewGameSession(sessionID, variant, maxPoints, false, uniqueID)
		host := &game.Participant{
			ID:       uuid.New().String(),
			Name:     name,
			UniqueID: uniqueID,
			Seat:     0,
			Team:     0,
		}
		if err := gs.AddParticipant(host); err != nil {
			http.Error(w, "failed to create game", http.StatusInternalServerError)
			return
		}
		if err := mgr.store.CreateGameSession(gs); err != nil {
			log.Printf("SoloPlay CreateGameSession: %v", err)
			http.Error(w, "failed to create game", http.StatusInternalServerError)
			return
		}
		_ = mgr.store.UpsertGameParticipant(sessionID, host)

		botCount, _ := strconv.Atoi(r.FormValue("bot_count"))
		if botCount < 1 {
			botCount = 3 // solo default: 1 human + 3 bots = 4 players
		}
		if botCount > 9 {
			botCount = 9
		}
		if err := gs.StartGame(botCount); err != nil {
			http.Error(w, "failed to start game", http.StatusInternalServerError)
			return
		}
		mgr.Set(gs)
		mgr.Persist(gs)

		SetHostCookie(w, sessionID)
		setGamePlayerCookie(w, sessionID, host.ID)

		if cp := gs.CurrentPlayer(); cp != nil && cp.IsBot {
			scheduleBotMoves(gs, mgr, hub)
		}

		http.Redirect(w, r, "/game/"+sessionID+"/play", http.StatusSeeOther)
	}
}

// GameSSEHandler handles GET /game/{id}/events — SSE stream for game events.
func GameSSEHandler(hub *SSEHub, mgr *GameSessionManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID := r.PathValue("id")
		channel := "game:" + sessionID
		uid := r.URL.Query().Get("uid")

		if uid != "" {
			// Cancel any pending removal/abandonment — this uid just (re)connected.
			mgr.cancelLobbyRemoval(sessionID, uid)
			mgr.cancelGameAbandonment(sessionID, uid, hub)
			hub.PresenceJoin(channel, uid)
			online := hub.OnlineUIDs(channel)
			log.Printf("[sse] CONNECT uid=%s session=%s | online=%v", uid, sessionID, online)
			defer func() {
				// PresenceLeave returns the remaining connection count atomically.
				// Using the return value (instead of a separate IsOnline call) prevents
				// the race where two simultaneous disconnects both see count=0.
				remaining := hub.PresenceLeave(channel, uid)
				gs := mgr.Get(sessionID)
				if gs == nil {
					log.Printf("[sse] DISCONNECT uid=%s session=%s (session not found)", uid, sessionID)
					return
				}
				if remaining > 0 {
					log.Printf("[sse] DISCONNECT uid=%s session=%s (page transition, %d connections remain)", uid, sessionID, remaining)
					return
				}
				// Snapshot status under lock to avoid a data race on gs.Status.
				gs.Lock()
				status := gs.Status
				// Snapshot participant name while under lock.
				playerName := ""
				for _, p := range gs.Participants {
					if p.UniqueID == uid {
						playerName = p.Name
						break
					}
				}
				gs.Unlock()
				log.Printf("[sse] DISCONNECT uid=%s name=%q session=%s status=%s", uid, playerName, sessionID, status)
				switch status {
				case game.SessionWaiting:
					// Use a grace period before removing — ordinary page reloads
					// also close the SSE, and we don't want to evict those players.
					mgr.scheduleLobbyRemoval(sessionID, uid, channel, hub)
				case game.SessionActive:
					// Always give the disconnecting player 60s to reconnect.
					// This covers F5, network drops, and the cookie-loss ?uid= redirect.
					// scheduleGameAbandonment skips if the player already reconnected (race guard).
					mgr.scheduleGameAbandonment(sessionID, uid, channel, hub)
					hub.Broadcast(channel, "game_player_left:"+playerName)
				}
			}()
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")

		ch := hub.Subscribe(channel)
		if ch == nil {
			http.Error(w, "too many connections to this session", http.StatusServiceUnavailable)
			return
		}
		defer hub.Unsubscribe(channel, ch)

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		_, _ = fmt.Fprintf(w, "data: connected\n\n")
		flusher.Flush()

		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case event, ok := <-ch:
				if !ok {
					return
				}
				_, _ = fmt.Fprintf(w, "event: update\ndata: %s\n\n", event)
				flusher.Flush()
			case <-ticker.C:
				_, _ = fmt.Fprintf(w, ": heartbeat\n\n")
				flusher.Flush()
			case <-r.Context().Done():
				return
			}
		}
	}
}

// GameBotsPreviewHandler handles POST /game/{id}/bots-preview — host sets pending bot count for the lobby preview.
func GameBotsPreviewHandler(mgr *GameSessionManager, hub *SSEHub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID := r.PathValue("id")
		if !IsHost(r, sessionID) {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		gs := mgr.Get(sessionID)
		if gs == nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		gs.Lock()
		notWaiting := gs.Status != game.SessionWaiting
		gs.Unlock()
		if notWaiting {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if err := r.ParseForm(); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		n, _ := strconv.Atoi(r.FormValue("bot_count"))
		if n < 0 {
			n = 0
		}
		gs.Lock()
		maxBots := 10 - len(gs.Participants)
		if n > maxBots {
			n = maxBots
		}
		gs.PendingBotCount = n
		gs.Unlock()
		hub.Broadcast("game:"+sessionID, fmt.Sprintf("game_bots_count:%d", n))
		w.WriteHeader(http.StatusNoContent)
	}
}
