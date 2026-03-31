package db

import (
	"database/sql"

	"github.com/leandrodaf/domino-placar/internal/game"
)

// GameSQLiteStore implements GameStore using SQLite.
type GameSQLiteStore struct {
	db *sql.DB
}

// NewGameSQLiteStore wraps the given SQLiteStore as a GameStore.
func NewGameSQLiteStore(s *SQLiteStore) *GameSQLiteStore {
	return &GameSQLiteStore{db: s.db}
}

func (s *GameSQLiteStore) CreateGameSession(gs *game.GameSession) error {
	return CreateGameSession(s.db, gs)
}

func (s *GameSQLiteStore) LoadGameSession(id string) (*game.GameSession, error) {
	return LoadGameSession(s.db, id)
}

func (s *GameSQLiteStore) SaveGameSession(gs *game.GameSession) error {
	return SaveGameSession(s.db, gs)
}

func (s *GameSQLiteStore) UpsertGameParticipant(sessionID string, p *game.Participant) error {
	return UpsertGameParticipant(s.db, sessionID, p)
}

func (s *GameSQLiteStore) GetGameParticipants(sessionID string) ([]*game.Participant, error) {
	return GetGameParticipants(s.db, sessionID)
}

func (s *GameSQLiteStore) RecordGameMove(id, sessionID, participantID string, roundNumber int, move game.Move, moveNum int) error {
	return RecordGameMove(s.db, id, sessionID, participantID, roundNumber, move, moveNum)
}

func (s *GameSQLiteStore) GetGameSessionInfo(id string) (*GameSessionInfo, error) {
	return GetGameSessionInfo(s.db, id)
}

func (s *GameSQLiteStore) GetActiveGameSessions() ([]string, error) {
	return GetActiveGameSessions(s.db)
}

func (s *GameSQLiteStore) FindOpenSession(excludeUID, variant string) (string, error) {
	return FindOpenSession(s.db, excludeUID, variant)
}

func (s *GameSQLiteStore) FindMyWaitingSession(uid, variant string) (string, error) {
	return FindMyWaitingSession(s.db, uid, variant)
}

func (s *GameSQLiteStore) FindMyActiveSession(uid string) (string, error) {
	return FindMyActiveSession(s.db, uid)
}

func (s *GameSQLiteStore) RemoveGameParticipant(sessionID, uniqueID string) error {
	return RemoveGameParticipant(s.db, sessionID, uniqueID)
}

func (s *GameSQLiteStore) CleanupZombieWaitingSessions() (int, error) {
	return CleanupZombieWaitingSessions(s.db)
}

func (s *GameSQLiteStore) CleanupZombieActiveSessions() (int, error) {
	return CleanupZombieActiveSessions(s.db)
}
