package db

import (
	"github.com/leandrodaf/domino-placar/internal/game"
	"github.com/leandrodaf/domino-placar/internal/models"
)

// Store define a interface de acesso ao banco de dados.
// Implementada por SQLiteStore e FirebaseStore.
type Store interface {
	// Match
	CreateMatch(id, baseURL, gameType string, maxPoints int) error
	GetMatch(id string) (*models.Match, error)
	UpdateMatchStatus(id, status string) error
	SetMatchWinner(matchID, playerID string) error

	// Player
	CreatePlayer(id, matchID, name, uniqueID string) error
	GetPlayers(matchID string) ([]models.Player, error)
	GetPlayer(playerID string) (*models.Player, error)
	GetPlayerByUniqueID(matchID, uniqueID string) (*models.Player, error)
	UpdatePlayerScore(playerID string, additionalPoints int) error
	UpdatePlayerStatus(playerID, status string) error
	UpdatePlayerName(playerID, name string) error
	SetPlayerScore(playerID string, score, maxPoints int) error
	CountPlayersByMatch(matchID string) (int, error)
	GetRanking(matchID string) ([]models.Player, error)

	// Round
	CreateRound(id, matchID string, roundNumber int) error
	GetCurrentRound(matchID string) (*models.Round, error)
	GetLastFinishedRound(matchID string) (*models.Round, error)
	GetRound(roundID string) (*models.Round, error)
	SetRoundStarter(roundID, playerID string) error
	CountRounds(matchID string) (int, error)
	SetRoundWinner(roundID, playerID string) error
	FinishRound(roundID string) error
	SetTableImage(roundID, imagePath, tilesJSON string) error
	GetRoundTableTiles(roundID string) (string, string, error)

	// HandImage
	CreateHandImage(id, roundID, playerID, imagePath string) error
	GetHandImage(imageID string) (*models.HandImage, error)
	GetHandImageByRoundAndPlayer(roundID, playerID string) (*models.HandImage, error)
	UpdateHandImagePoints(imageID string, points int, confirmed bool, tilesJSON string) error
	GetHandImages(roundID string) ([]models.HandImage, error)

	// Tournament
	CreateTournament(id, name, baseURL, gameType string, maxPoints int) error
	GetTournament(id string) (*models.Tournament, error)
	UpdateTournamentStatus(id, status string) error
	UpdateTournamentGameType(id, gameType string, maxPoints int) error
	CreateTournamentPlayer(id, tournamentID, name, uniqueID string) error
	GetTournamentPlayers(tournamentID string) ([]models.TournamentPlayer, error)
	GetTournamentPlayerByUniqueID(tournamentID, uniqueID string) (*models.TournamentPlayer, error)
	CountTournamentPlayers(tournamentID string) (int, error)
	AssignTournamentPlayer(playerID string, tableNum int, matchID string) error
	CreateTournamentMatch(tournamentID, matchID string, tableNum int) error
	GetTournamentMatches(tournamentID string) ([]models.TournamentMatch, error)
	GetTournamentRanking(tournamentID string) ([]models.TournamentRankEntry, error)

	// Global Stats
	GetGlobalStats() ([]models.GlobalStat, error)
	GetMostRoundsLost() ([]models.ZoeiraStat, error)
	GetPintoKings() ([]models.ZoeiraStat, error)
	GetBrancoKings() ([]models.ZoeiraStat, error)
	GetCloseCallKings() ([]models.ZoeiraStat, error)

	// Nicknames
	CreateNomination(id, nominatedUID, nominatedName, matchID, nickname, proposerUID string) error
	VoteForNomination(nominationID, voterUID string) (bool, error)
	GetNominationsForMatch(matchID string) ([]models.NicknameNomination, error)
	GetNominationsForPlayer(matchID, nominatedUID string) ([]models.NicknameNomination, error)
	GetTopNicknameForPlayer(uniqueID string) string
	GetAllTimeNicknames() ([]models.NicknameNomination, error)

	// Turma
	CreateTurma(turma *models.Turma) error
	GetTurma(id string) (*models.Turma, error)
	GetTurmaByInviteCode(code string) (*models.Turma, error)
	AddTurmaMember(member *models.TurmaMember) error
	GetTurmaMembers(turmaID string) ([]models.TurmaMember, error)
	GetTurmaMember(turmaID, uniqueID string) (*models.TurmaMember, error)
	RemoveTurmaMember(turmaID, uniqueID string) error
	IsTurmaMember(turmaID, uniqueID string) (bool, error)
	GetTurmasByMember(uniqueID string) ([]models.Turma, error)
	GetTurmaMatches(turmaID string) ([]models.Match, error)
	GetTurmaRanking(turmaID string) ([]models.TurmaRankEntry, error)
	CreateMatchInTurma(id, baseURL, turmaID, gameType string, maxPoints int) error
	DeleteMatch(id string) error
}

// GameStore defines the persistence interface for online game sessions.
// It is implemented separately from Store to keep concerns isolated.
type GameStore interface {
	CreateGameSession(gs *game.GameSession) error
	LoadGameSession(id string) (*game.GameSession, error)
	SaveGameSession(gs *game.GameSession) error
	UpsertGameParticipant(sessionID string, p *game.Participant) error
	GetGameParticipants(sessionID string) ([]*game.Participant, error)
	RecordGameMove(id, sessionID, participantID string, roundNumber int, move game.Move, moveNum int) error
	GetGameSessionInfo(id string) (*GameSessionInfo, error)
	GetActiveGameSessions() ([]string, error)
	// FindOpenSession returns the ID of a waiting session with open seats
	// that excludeUID has not already joined and matches the given variant,
	// or "" if none found.
	FindOpenSession(excludeUID, variant string) (string, error)
	// FindMyWaitingSession returns the ID of a waiting session that uid IS
	// already a participant in (for resuming after a page reload or brief disconnect).
	FindMyWaitingSession(uid, variant string) (string, error)
	// FindMyActiveSession returns the ID of an active (in-progress) session that
	// uid is already a participant in (for reconnecting players who left mid-game).
	FindMyActiveSession(uid string) (string, error)
	RemoveGameParticipant(sessionID, uniqueID string) error
	// CleanupZombieWaitingSessions marks waiting sessions as finished on startup.
	CleanupZombieWaitingSessions() (int, error)
	// CleanupZombieActiveSessions marks active sessions as finished on startup.
	// After a restart all in-memory state and SSE connections are gone.
	CleanupZombieActiveSessions() (int, error)
}
