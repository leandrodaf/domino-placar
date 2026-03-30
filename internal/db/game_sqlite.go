package db

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/leandrodaf/domino-placar/internal/game"
)

// ─── Game Session CRUD ─────────────────────────────────────────────────────

// GameSessionRecord is the DB representation of a game session.
type GameSessionRecord struct {
	ID           string
	Variant      string
	MaxPoints    int
	TeamMode     bool
	Status       string
	HostUniqueID string
	TurnIdx      int
	RoundNumber  int
	BoardJSON    string
	BoneyardJSON string
	PassCount    int
}

// CreateGameSession inserts a new game session.
func CreateGameSession(db *sql.DB, gs *game.GameSession) error {
	_, err := db.Exec(`INSERT INTO game_sessions
		(id, variant, max_points, team_mode, status, host_unique_id, turn_idx, round_number, board_json, boneyard_json, pass_count)
		VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		gs.ID, gs.Variant, gs.MaxPoints, boolToInt(gs.TeamMode),
		string(gs.Status), gs.HostID, gs.TurnIdx, gs.RoundNumber,
		game.BoardToJSON(gs.Board), game.BoneyardToJSON(gs.Boneyard), gs.PassCount)
	return err
}

// GetGameSession loads a session record (metadata only, participants loaded separately).
func GetGameSession(db *sql.DB, id string) (*GameSessionRecord, error) {
	row := db.QueryRow(`SELECT id,variant,max_points,team_mode,status,host_unique_id,turn_idx,round_number,board_json,boneyard_json,pass_count
		FROM game_sessions WHERE id=?`, id)
	var r GameSessionRecord
	var teamModeInt int
	if err := row.Scan(&r.ID, &r.Variant, &r.MaxPoints, &teamModeInt, &r.Status, &r.HostUniqueID,
		&r.TurnIdx, &r.RoundNumber, &r.BoardJSON, &r.BoneyardJSON, &r.PassCount); err != nil {
		return nil, err
	}
	r.TeamMode = teamModeInt == 1
	return &r, nil
}

// SaveGameSession persists the current in-memory state of a session to the DB.
func SaveGameSession(db *sql.DB, gs *game.GameSession) error {
	_, err := db.Exec(`UPDATE game_sessions
		SET status=?, turn_idx=?, round_number=?, board_json=?, boneyard_json=?, pass_count=?
		WHERE id=?`,
		string(gs.Status), gs.TurnIdx, gs.RoundNumber,
		game.BoardToJSON(gs.Board), game.BoneyardToJSON(gs.Boneyard), gs.PassCount,
		gs.ID)
	return err
}

// ─── Participants ─────────────────────────────────────────────────────────

// UpsertGameParticipant inserts or updates a participant record.
func UpsertGameParticipant(db *sql.DB, sessionID string, p *game.Participant) error {
	_, err := db.Exec(`INSERT INTO game_participants (id,session_id,name,unique_id,seat,team,is_bot,total_score,hand_json)
		VALUES (?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET total_score=excluded.total_score, hand_json=excluded.hand_json`,
		p.ID, sessionID, p.Name, p.UniqueID, p.Seat, p.Team, boolToInt(p.IsBot),
		p.TotalScore, game.HandToJSON(p.Hand))
	return err
}

// GetGameParticipants returns all participants for a session, ordered by seat.
func GetGameParticipants(db *sql.DB, sessionID string) ([]*game.Participant, error) {
	rows, err := db.Query(`SELECT id,name,unique_id,seat,team,is_bot,total_score,hand_json
		FROM game_participants WHERE session_id=? ORDER BY seat`, sessionID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*game.Participant
	for rows.Next() {
		var p game.Participant
		var isBotInt int
		var handJSON string
		if err := rows.Scan(&p.ID, &p.Name, &p.UniqueID, &p.Seat, &p.Team, &isBotInt, &p.TotalScore, &handJSON); err != nil {
			return nil, err
		}
		p.IsBot = isBotInt == 1
		p.Hand = game.HandFromJSON(handJSON)
		out = append(out, &p)
	}
	return out, rows.Err()
}

// ─── Moves ────────────────────────────────────────────────────────────────

// RecordGameMove saves a move to the DB.
func RecordGameMove(db *sql.DB, id, sessionID, participantID string, roundNumber int, move game.Move, moveNum int) error {
	tileStr := ""
	if move.Type == game.MovePlay {
		tileStr = move.Tile.String()
	}
	_, err := db.Exec(`INSERT INTO game_moves (id,session_id,participant_id,round_number,move_type,tile,side,move_num)
		VALUES (?,?,?,?,?,?,?,?)`,
		id, sessionID, participantID, roundNumber, string(move.Type), tileStr, move.Side, moveNum)
	return err
}

// ─── Restore session from DB ─────────────────────────────────────────────

// LoadGameSession rebuilds a GameSession from the database.
func LoadGameSession(db *sql.DB, id string) (*game.GameSession, error) {
	rec, err := GetGameSession(db, id)
	if err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}
	gs := game.NewGameSession(rec.ID, rec.Variant, rec.MaxPoints, rec.TeamMode, rec.HostUniqueID)
	gs.Status = game.SessionStatus(rec.Status)
	gs.TurnIdx = rec.TurnIdx
	gs.RoundNumber = rec.RoundNumber
	gs.Board = game.BoardFromJSON(rec.BoardJSON)
	gs.Boneyard = game.BoneyardFromJSON(rec.BoneyardJSON)
	gs.PassCount = rec.PassCount

	participants, err := GetGameParticipants(db, id)
	if err != nil {
		return nil, err
	}
	gs.Participants = participants
	return gs, nil
}

// GetActiveGameSessions returns IDs of sessions in waiting or active state.
func GetActiveGameSessions(db *sql.DB) ([]string, error) {
	rows, err := db.Query(`SELECT id FROM game_sessions WHERE status IN ('waiting','active') ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// FindOpenSession returns the ID of a waiting session with fewer than 4 players
// that excludeUID has not already joined. Returns "" if none is found.
func FindOpenSession(db *sql.DB, excludeUID string) (string, error) {
	row := db.QueryRow(`
		SELECT gs.id
		FROM game_sessions gs
		LEFT JOIN game_participants gp ON gs.id = gp.session_id
		WHERE gs.status = 'waiting'
		  AND NOT EXISTS (
		      SELECT 1 FROM game_participants
		      WHERE session_id = gs.id AND unique_id = ?
		  )
		GROUP BY gs.id
		HAVING COUNT(gp.id) < 4
		ORDER BY COUNT(gp.id) DESC, gs.created_at ASC
		LIMIT 1`, excludeUID)
	var id string
	if err := row.Scan(&id); err != nil {
		return "", nil // no open session found
	}
	return id, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// GetGameSessionPublicState returns a map suitable for JSON serialization of a session's public state.
func GetGameSessionPublicState(db *sql.DB, sessionID, viewerUID string) (map[string]any, error) {
	gs, err := LoadGameSession(db, sessionID)
	if err != nil {
		return nil, err
	}
	return gs.StateForPlayer(viewerUID), nil
}

// ─── Session JSON for lobby ───────────────────────────────────────────────

// GameSessionInfo holds summary info for the lobby.
type GameSessionInfo struct {
	ID          string
	Variant     string
	MaxPoints   int
	TeamMode    bool
	Status      string
	PlayerCount int
	HostName    string
}

func GetGameSessionInfo(db *sql.DB, id string) (*GameSessionInfo, error) {
	row := db.QueryRow(`SELECT gs.id, gs.variant, gs.max_points, gs.team_mode, gs.status,
		COUNT(gp.id) as player_count
		FROM game_sessions gs
		LEFT JOIN game_participants gp ON gp.session_id = gs.id
		WHERE gs.id=?
		GROUP BY gs.id`, id)
	var info GameSessionInfo
	var teamModeInt int
	if err := row.Scan(&info.ID, &info.Variant, &info.MaxPoints, &teamModeInt, &info.Status, &info.PlayerCount); err != nil {
		return nil, err
	}
	info.TeamMode = teamModeInt == 1
	return &info, nil
}

// SerializeBlockedIDs converts the blocked map to JSON for storage.
func SerializeBlockedIDs(blocked map[string]bool) string {
	ids := make([]string, 0, len(blocked))
	for id := range blocked {
		ids = append(ids, id)
	}
	data, _ := json.Marshal(ids)
	return string(data)
}
