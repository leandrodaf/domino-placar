package game

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// SessionStatus is the lifecycle state of a game session.
type SessionStatus string

const (
	SessionWaiting  SessionStatus = "waiting"
	SessionActive   SessionStatus = "active"
	SessionFinished SessionStatus = "finished"
)

// Participant is a player in the game (human or bot).
type Participant struct {
	ID         string
	Name       string
	UniqueID   string
	Seat       int
	Team       int // 0 or 1
	IsBot      bool
	TotalScore int
	Hand       Hand
}

// GameSession holds the full in-memory state of one game.
type GameSession struct {
	mu sync.Mutex

	ID        string
	Variant   string
	Rules     VariantRules
	MaxPoints int
	TeamMode  bool
	Status    SessionStatus
	HostID    string // UniqueID of the host

	Participants []*Participant // ordered by seat
	TurnIdx      int            // current turn index into Participants

	// Round state
	RoundNumber int
	Board       BoardState
	Boneyard    []Tile
	PassCount   int             // consecutive passes in current round
	BlockedIDs  map[string]bool

	CreatedAt time.Time
}

// NewGameSession creates a fresh session.
func NewGameSession(id, variant string, maxPoints int, teamMode bool, hostUID string) *GameSession {
	return &GameSession{
		ID:         id,
		Variant:    variant,
		Rules:      GetVariant(variant),
		MaxPoints:  maxPoints,
		TeamMode:   teamMode,
		Status:     SessionWaiting,
		HostID:     hostUID,
		BlockedIDs: map[string]bool{},
		CreatedAt:  time.Now(),
	}
}

// AddParticipant adds a player to the lobby. Returns error if full or already joined.
func (gs *GameSession) AddParticipant(p *Participant) error {
	gs.mu.Lock()
	defer gs.mu.Unlock()
	if gs.Status != SessionWaiting {
		return fmt.Errorf("game already started")
	}
	if len(gs.Participants) >= 4 {
		return fmt.Errorf("game is full")
	}
	for _, existing := range gs.Participants {
		if existing.UniqueID == p.UniqueID {
			return fmt.Errorf("already in game")
		}
	}
	p.Seat = len(gs.Participants)
	gs.Participants = append(gs.Participants, p)
	return nil
}

// FindParticipant returns a participant by UniqueID, or nil.
func (gs *GameSession) FindParticipant(uniqueID string) *Participant {
	for _, p := range gs.Participants {
		if p.UniqueID == uniqueID {
			return p
		}
	}
	return nil
}

// FindParticipantByID returns a participant by ID, or nil.
func (gs *GameSession) FindParticipantByID(id string) *Participant {
	for _, p := range gs.Participants {
		if p.ID == id {
			return p
		}
	}
	return nil
}

// StartGame deals tiles and begins the first round.
// Fills empty seats with bots up to minPlayers (2 minimum).
func (gs *GameSession) StartGame(addBots bool) error {
	gs.mu.Lock()
	defer gs.mu.Unlock()
	if gs.Status != SessionWaiting {
		return fmt.Errorf("already started")
	}
	if len(gs.Participants) < 1 {
		return fmt.Errorf("need at least 1 player")
	}
	// Add bots to fill up to 4 players (or at least 2)
	if addBots {
		target := 4
		if len(gs.Participants) >= 2 {
			target = len(gs.Participants) // no need to add bots if 2+ players
			if target < 2 {
				target = 2
			}
		}
		for len(gs.Participants) < target {
			botIdx := len(gs.Participants) + 1
			gs.Participants = append(gs.Participants, &Participant{
				ID:       fmt.Sprintf("bot-%s-%d", gs.ID, botIdx),
				Name:     fmt.Sprintf("Bot %d", botIdx),
				UniqueID: fmt.Sprintf("bot-%s-%d", gs.ID, botIdx),
				Seat:     len(gs.Participants) - 1,
				IsBot:    true,
			})
		}
	}
	if len(gs.Participants) < 2 {
		return fmt.Errorf("need at least 2 players")
	}
	gs.Status = SessionActive
	gs.RoundNumber = 1
	gs.dealRound()
	return nil
}

// dealRound distributes tiles and sets the starting player.
func (gs *GameSession) dealRound() {
	n := len(gs.Participants)
	hands, boneyard := Deal(n, gs.Rules.TilesPerPlayer)
	for i, p := range gs.Participants {
		p.Hand = hands[i]
	}
	gs.Boneyard = boneyard
	gs.Board = BoardState{}
	gs.PassCount = 0
	gs.BlockedIDs = map[string]bool{}

	handSlice := make([]Hand, n)
	for i, p := range gs.Participants {
		handSlice[i] = p.Hand
	}
	gs.TurnIdx = FindFirstPlayer(handSlice)
}

// CurrentPlayer returns the participant whose turn it is.
func (gs *GameSession) CurrentPlayer() *Participant {
	gs.mu.Lock()
	defer gs.mu.Unlock()
	if len(gs.Participants) == 0 {
		return nil
	}
	return gs.Participants[gs.TurnIdx%len(gs.Participants)]
}

// PlayTile executes a play move. Returns RoundEndResult.
func (gs *GameSession) PlayTile(participantID string, tile Tile, side string) (RoundEndResult, error) {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	p := gs.FindParticipantByID(participantID)
	if p == nil {
		return RoundEndResult{}, fmt.Errorf("participant not found")
	}
	if gs.Participants[gs.TurnIdx%len(gs.Participants)].ID != participantID {
		return RoundEndResult{}, fmt.Errorf("not your turn")
	}
	if !p.Hand.Contains(tile) {
		return RoundEndResult{}, fmt.Errorf("tile not in hand")
	}
	if !ValidateMove(gs.Board, tile, side) {
		return RoundEndResult{}, fmt.Errorf("invalid move: tile %s does not fit on %s", tile, side)
	}

	gs.Board = ApplyMove(gs.Board, tile, side)
	p.Hand = p.Hand.Remove(tile)
	gs.PassCount = 0
	delete(gs.BlockedIDs, participantID)

	// Check round end
	hands := gs.allHands()
	result := CheckRoundEnd(hands, false)
	if result.Ended {
		gs.applyRoundResult(result)
		return result, nil
	}

	gs.advanceTurn()
	return RoundEndResult{Ended: false}, nil
}

// Pass marks a player as passing (only when truly blocked).
func (gs *GameSession) Pass(participantID string) (RoundEndResult, error) {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	p := gs.FindParticipantByID(participantID)
	if p == nil {
		return RoundEndResult{}, fmt.Errorf("participant not found")
	}
	if gs.Participants[gs.TurnIdx%len(gs.Participants)].ID != participantID {
		return RoundEndResult{}, fmt.Errorf("not your turn")
	}
	if !IsBlocked(p.Hand, gs.Board, gs.Rules.HasBoneyard, len(gs.Boneyard)) {
		return RoundEndResult{}, fmt.Errorf("cannot pass: you have playable tiles or can draw")
	}

	gs.BlockedIDs[participantID] = true
	gs.PassCount++

	// Check if all blocked
	allBlocked := len(gs.BlockedIDs) >= len(gs.Participants)
	hands := gs.allHands()
	result := CheckRoundEnd(hands, allBlocked)
	if result.Ended {
		gs.applyRoundResult(result)
		return result, nil
	}

	gs.advanceTurn()
	return RoundEndResult{Ended: false}, nil
}

// DrawTile draws from boneyard (for variants with boneyard).
func (gs *GameSession) DrawTile(participantID string) (Tile, error) {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	p := gs.FindParticipantByID(participantID)
	if p == nil {
		return Tile{}, fmt.Errorf("participant not found")
	}
	if gs.Participants[gs.TurnIdx%len(gs.Participants)].ID != participantID {
		return Tile{}, fmt.Errorf("not your turn")
	}
	if !gs.Rules.HasBoneyard || len(gs.Boneyard) == 0 {
		return Tile{}, fmt.Errorf("no tiles to draw")
	}

	tile := gs.Boneyard[0]
	gs.Boneyard = gs.Boneyard[1:]
	p.Hand = append(p.Hand, tile)
	return tile, nil
}

// StartNextRound begins a new round after a round has ended.
func (gs *GameSession) StartNextRound() {
	gs.mu.Lock()
	defer gs.mu.Unlock()
	gs.RoundNumber++
	gs.dealRound()
}

// CheckGameOver returns true if any participant (or team) has reached MaxPoints.
func (gs *GameSession) CheckGameOver() (bool, *Participant) {
	for _, p := range gs.Participants {
		if p.TotalScore >= gs.MaxPoints {
			return true, p
		}
	}
	return false, nil
}

// StateForPlayer returns a JSON-serializable snapshot of the game state
// from the perspective of the given uniqueID (hides other players' hands).
func (gs *GameSession) StateForPlayer(uniqueID string) map[string]any {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	var myHand Hand
	players := make([]map[string]any, 0, len(gs.Participants))
	currentPlayerID := ""
	if len(gs.Participants) > 0 {
		currentPlayerID = gs.Participants[gs.TurnIdx%len(gs.Participants)].ID
	}

	for _, p := range gs.Participants {
		entry := map[string]any{
			"id":          p.ID,
			"name":        p.Name,
			"unique_id":   p.UniqueID,
			"seat":        p.Seat,
			"team":        p.Team,
			"is_bot":      p.IsBot,
			"total_score": p.TotalScore,
			"hand_count":  len(p.Hand),
			"is_turn":     p.ID == currentPlayerID,
		}
		if p.UniqueID == uniqueID {
			myHand = p.Hand
			entry["hand"] = handToStrings(p.Hand)
		}
		players = append(players, entry)
	}

	boardTiles := make([]map[string]any, 0, len(gs.Board.Chain))
	for _, pt := range gs.Board.Chain {
		boardTiles = append(boardTiles, map[string]any{
			"tile":    pt.Tile.String(),
			"high":    pt.Tile.High,
			"low":     pt.Tile.Low,
			"flipped": pt.Flipped,
		})
	}

	var playable []string
	if uniqueID != "" {
		pl := gs.FindParticipant(uniqueID)
		if pl != nil && currentPlayerID == pl.ID {
			for _, t := range GetPlayableTiles(pl.Hand, gs.Board) {
				playable = append(playable, t.String())
			}
		}
	}

	return map[string]any{
		"id":             gs.ID,
		"variant":        gs.Variant,
		"max_points":     gs.MaxPoints,
		"team_mode":      gs.TeamMode,
		"status":         string(gs.Status),
		"round_number":   gs.RoundNumber,
		"current_player": currentPlayerID,
		"board": map[string]any{
			"tiles":      boardTiles,
			"left_open":  gs.Board.LeftOpen,
			"right_open": gs.Board.RightOpen,
			"tile_count": len(gs.Board.Chain),
		},
		"boneyard_count": len(gs.Boneyard),
		"players":        players,
		"my_hand":        handToStrings(myHand),
		"playable_tiles": playable,
	}
}

func (gs *GameSession) allHands() map[string]Hand {
	m := map[string]Hand{}
	for _, p := range gs.Participants {
		m[p.ID] = p.Hand
	}
	return m
}

func (gs *GameSession) advanceTurn() {
	gs.TurnIdx = (gs.TurnIdx + 1) % len(gs.Participants)
}

func (gs *GameSession) applyRoundResult(result RoundEndResult) {
	winner := gs.FindParticipantByID(result.WinnerID)
	if winner != nil {
		winner.TotalScore += result.PointsAwarded
	}
	if over, _ := gs.CheckGameOver(); over {
		gs.Status = SessionFinished
	}
}

func handToStrings(h Hand) []string {
	if h == nil {
		return []string{}
	}
	s := make([]string, len(h))
	for i, t := range h {
		s[i] = t.String()
	}
	return s
}

// BoardToJSON serializes the board for storage.
func BoardToJSON(board BoardState) string {
	data, _ := json.Marshal(board)
	return string(data)
}

// BoardFromJSON deserializes a board.
func BoardFromJSON(s string) BoardState {
	var b BoardState
	if s != "" {
		_ = json.Unmarshal([]byte(s), &b)
	}
	return b
}

// BoneyardToJSON serializes the boneyard for storage.
func BoneyardToJSON(tiles []Tile) string {
	data, _ := json.Marshal(tiles)
	return string(data)
}

// BoneyardFromJSON deserializes a boneyard.
func BoneyardFromJSON(s string) []Tile {
	var tiles []Tile
	if s != "" {
		_ = json.Unmarshal([]byte(s), &tiles)
	}
	return tiles
}

// HandToJSON serializes a hand.
func HandToJSON(h Hand) string {
	data, _ := json.Marshal(h)
	return string(data)
}

// HandFromJSON deserializes a hand.
func HandFromJSON(s string) Hand {
	var h Hand
	if s != "" {
		_ = json.Unmarshal([]byte(s), &h)
	}
	return h
}
