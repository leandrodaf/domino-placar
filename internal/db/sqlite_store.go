package db

import (
	"database/sql"

	"github.com/leandrodaf/domino-placar/internal/models"
)

// SQLiteStore implementa Store usando SQLite.
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore cria um SQLiteStore a partir de um *sql.DB já aberto.
func NewSQLiteStore(db *sql.DB) *SQLiteStore {
	return &SQLiteStore{db: db}
}

func (s *SQLiteStore) CreateMatch(id, baseURL string) error {
	return CreateMatch(s.db, id, baseURL)
}

func (s *SQLiteStore) GetMatch(id string) (*models.Match, error) {
	return GetMatch(s.db, id)
}

func (s *SQLiteStore) UpdateMatchStatus(id, status string) error {
	return UpdateMatchStatus(s.db, id, status)
}

func (s *SQLiteStore) SetMatchWinner(matchID, playerID string) error {
	return SetMatchWinner(s.db, matchID, playerID)
}

func (s *SQLiteStore) CreatePlayer(id, matchID, name, uniqueID string) error {
	return CreatePlayer(s.db, id, matchID, name, uniqueID)
}

func (s *SQLiteStore) GetPlayers(matchID string) ([]models.Player, error) {
	return GetPlayers(s.db, matchID)
}

func (s *SQLiteStore) GetPlayer(playerID string) (*models.Player, error) {
	return GetPlayer(s.db, playerID)
}

func (s *SQLiteStore) GetPlayerByUniqueID(matchID, uniqueID string) (*models.Player, error) {
	return GetPlayerByUniqueID(s.db, matchID, uniqueID)
}

func (s *SQLiteStore) UpdatePlayerScore(playerID string, additionalPoints int) error {
	return UpdatePlayerScore(s.db, playerID, additionalPoints)
}

func (s *SQLiteStore) UpdatePlayerStatus(playerID, status string) error {
	return UpdatePlayerStatus(s.db, playerID, status)
}

func (s *SQLiteStore) UpdatePlayerName(playerID, name string) error {
	_, err := s.db.Exec(`UPDATE players SET name = ? WHERE id = ?`, name, playerID)
	return err
}

func (s *SQLiteStore) SetPlayerScore(playerID string, score int) error {
	return SetPlayerScore(s.db, playerID, score)
}

func (s *SQLiteStore) CountPlayersByMatch(matchID string) (int, error) {
	return CountPlayersByMatch(s.db, matchID)
}

func (s *SQLiteStore) GetRanking(matchID string) ([]models.Player, error) {
	return GetRanking(s.db, matchID)
}

func (s *SQLiteStore) CreateRound(id, matchID string, roundNumber int) error {
	return CreateRound(s.db, id, matchID, roundNumber)
}

func (s *SQLiteStore) GetCurrentRound(matchID string) (*models.Round, error) {
	return GetCurrentRound(s.db, matchID)
}

func (s *SQLiteStore) GetLastFinishedRound(matchID string) (*models.Round, error) {
	return GetLastFinishedRound(s.db, matchID)
}

func (s *SQLiteStore) GetRound(roundID string) (*models.Round, error) {
	return GetRound(s.db, roundID)
}

func (s *SQLiteStore) SetRoundStarter(roundID, playerID string) error {
	return SetRoundStarter(s.db, roundID, playerID)
}

func (s *SQLiteStore) CountRounds(matchID string) (int, error) {
	return CountRounds(s.db, matchID)
}

func (s *SQLiteStore) SetRoundWinner(roundID, playerID string) error {
	return SetRoundWinner(s.db, roundID, playerID)
}

func (s *SQLiteStore) FinishRound(roundID string) error {
	return FinishRound(s.db, roundID)
}

func (s *SQLiteStore) SetTableImage(roundID, imagePath, tilesJSON string) error {
	return SetTableImage(s.db, roundID, imagePath, tilesJSON)
}

func (s *SQLiteStore) GetRoundTableTiles(roundID string) (string, string, error) {
	return GetRoundTableTiles(s.db, roundID)
}

func (s *SQLiteStore) CreateHandImage(id, roundID, playerID, imagePath string) error {
	return CreateHandImage(s.db, id, roundID, playerID, imagePath)
}

func (s *SQLiteStore) GetHandImage(imageID string) (*models.HandImage, error) {
	return GetHandImage(s.db, imageID)
}

func (s *SQLiteStore) GetHandImageByRoundAndPlayer(roundID, playerID string) (*models.HandImage, error) {
	return GetHandImageByRoundAndPlayer(s.db, roundID, playerID)
}

func (s *SQLiteStore) UpdateHandImagePoints(imageID string, points int, confirmed bool, tilesJSON string) error {
	return UpdateHandImagePoints(s.db, imageID, points, confirmed, tilesJSON)
}

func (s *SQLiteStore) GetHandImages(roundID string) ([]models.HandImage, error) {
	return GetHandImages(s.db, roundID)
}

func (s *SQLiteStore) CreateTournament(id, name, baseURL string) error {
	return CreateTournament(s.db, id, name, baseURL)
}

func (s *SQLiteStore) GetTournament(id string) (*models.Tournament, error) {
	return GetTournament(s.db, id)
}

func (s *SQLiteStore) UpdateTournamentStatus(id, status string) error {
	return UpdateTournamentStatus(s.db, id, status)
}

func (s *SQLiteStore) CreateTournamentPlayer(id, tournamentID, name, uniqueID string) error {
	return CreateTournamentPlayer(s.db, id, tournamentID, name, uniqueID)
}

func (s *SQLiteStore) GetTournamentPlayers(tournamentID string) ([]models.TournamentPlayer, error) {
	return GetTournamentPlayers(s.db, tournamentID)
}

func (s *SQLiteStore) GetTournamentPlayerByUniqueID(tournamentID, uniqueID string) (*models.TournamentPlayer, error) {
	return GetTournamentPlayerByUniqueID(s.db, tournamentID, uniqueID)
}

func (s *SQLiteStore) CountTournamentPlayers(tournamentID string) (int, error) {
	return CountTournamentPlayers(s.db, tournamentID)
}

func (s *SQLiteStore) AssignTournamentPlayer(playerID string, tableNum int, matchID string) error {
	return AssignTournamentPlayer(s.db, playerID, tableNum, matchID)
}

func (s *SQLiteStore) CreateTournamentMatch(tournamentID, matchID string, tableNum int) error {
	return CreateTournamentMatch(s.db, tournamentID, matchID, tableNum)
}

func (s *SQLiteStore) GetTournamentMatches(tournamentID string) ([]models.TournamentMatch, error) {
	return GetTournamentMatches(s.db, tournamentID)
}

func (s *SQLiteStore) GetTournamentRanking(tournamentID string) ([]models.TournamentRankEntry, error) {
	return GetTournamentRanking(s.db, tournamentID)
}

func (s *SQLiteStore) GetGlobalStats() ([]models.GlobalStat, error) {
	return GetGlobalStats(s.db)
}

func (s *SQLiteStore) GetMostRoundsLost() ([]models.ZoeiraStat, error) {
	return GetMostRoundsLost(s.db)
}

func (s *SQLiteStore) GetPintoKings() ([]models.ZoeiraStat, error) {
	return GetPintoKings(s.db)
}

func (s *SQLiteStore) GetBrancoKings() ([]models.ZoeiraStat, error) {
	return GetBrancoKings(s.db)
}

func (s *SQLiteStore) GetCloseCallKings() ([]models.ZoeiraStat, error) {
	return GetCloseCallKings(s.db)
}

func (s *SQLiteStore) CreateNomination(id, nominatedUID, nominatedName, matchID, nickname, proposerUID string) error {
	return CreateNomination(s.db, id, nominatedUID, nominatedName, matchID, nickname, proposerUID)
}

func (s *SQLiteStore) VoteForNomination(nominationID, voterUID string) (bool, error) {
	return VoteForNomination(s.db, nominationID, voterUID)
}

func (s *SQLiteStore) GetNominationsForMatch(matchID string) ([]models.NicknameNomination, error) {
	return GetNominationsForMatch(s.db, matchID)
}

func (s *SQLiteStore) GetNominationsForPlayer(matchID, nominatedUID string) ([]models.NicknameNomination, error) {
	return GetNominationsForPlayer(s.db, matchID, nominatedUID)
}

func (s *SQLiteStore) GetTopNicknameForPlayer(uniqueID string) string {
	return GetTopNicknameForPlayer(s.db, uniqueID)
}

func (s *SQLiteStore) GetAllTimeNicknames() ([]models.NicknameNomination, error) {
	return GetAllTimeNicknames(s.db)
}
