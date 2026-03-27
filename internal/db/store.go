package db

import "github.com/leandrodaf/domino-placar/internal/models"

// Store define a interface de acesso ao banco de dados.
// Implementada por SQLiteStore e FirebaseStore.
type Store interface {
	// Match
	CreateMatch(id, baseURL string) error
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
	SetPlayerScore(playerID string, score int) error
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
	CreateTournament(id, name, baseURL string) error
	GetTournament(id string) (*models.Tournament, error)
	UpdateTournamentStatus(id, status string) error
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
}
