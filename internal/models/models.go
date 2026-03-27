package models

import "time"

type Match struct {
	ID             string    `db:"id"`
	Status         string    `db:"status"`
	BaseURL        string    `db:"base_url"`
	WinnerPlayerID string    `db:"winner_player_id"`
	CreatedAt      time.Time `db:"created_at"`
}

type Player struct {
	ID               string    `db:"id"`
	MatchID          string    `db:"match_id"`
	Name             string    `db:"name"`
	UniqueIdentifier string    `db:"unique_identifier"`
	TotalScore       int       `db:"total_score"`
	Status           string    `db:"status"`
	CreatedAt        time.Time `db:"created_at"`
}

type Round struct {
	ID              string    `db:"id"`
	MatchID         string    `db:"match_id"`
	RoundNumber     int       `db:"round_number"`
	WinnerPlayerID  string    `db:"winner_player_id"`
	StarterPlayerID string    `db:"starter_player_id"`
	Status          string    `db:"status"`
	CreatedAt       time.Time `db:"created_at"`
}

type HandImage struct {
	ID                string    `db:"id"`
	RoundID           string    `db:"round_id"`
	PlayerID          string    `db:"player_id"`
	ImagePath         string    `db:"image_path"`
	PointsCalculated  int       `db:"points_calculated"`
	Confirmed         int       `db:"confirmed"`
	DetectedTilesJSON string    `db:"detected_tiles"` // JSON array, ex: ["3-4","0-2"]
	CreatedAt         time.Time `db:"created_at"`
}

// TileStats resume a cobertura de pedras em uma rodada.
type TileStats struct {
	HandTiles      []string // pedras nas mãos dos jogadores
	TableTiles     []string // pedras detectadas na foto da mesa
	SeenTiles      []string // união de hand + table (sem duplicatas)
	TotalInSet     int      // total de pedras no jogo (28 ou 55)
	SeenCount      int
	RemainingCount int  // pedras provavelmente no dorme
	HasTablePhoto  bool // se a foto da mesa foi tirada
}

// GlobalStat agrega estatísticas de um jogador (pelo unique_identifier) em todas as partidas.
type GlobalStat struct {
	UniqueIdentifier string
	Name             string
	MatchesPlayed    int
	MatchesWon       int
	BustCount        int
	MaxBustScore     int
	TotalRoundWins   int
}

// ZoeiraStat é usado para rankings de zoeiras/conquistas no Hall da Fama.
type ZoeiraStat struct {
	UniqueIdentifier string
	Name             string
	Count            int
}

// NicknameNomination é um apelido proposto para um jogador.
type NicknameNomination struct {
	ID                  string
	NominatedUniqueID   string
	NominatedName       string
	MatchID             string
	Nickname            string
	VoteCount           int
	ProposerUniqueID    string
	CreatedAt           time.Time
}

// NicknameVote registra que um unique_id votou em uma nomination.
type NicknameVote struct {
	NominationID   string
	VoterUniqueID  string
}

type Tournament struct {
	ID        string
	Name      string
	Status    string // registration, active, finished
	BaseURL   string
	CreatedAt time.Time
}

type TournamentPlayer struct {
	ID               string
	TournamentID     string
	Name             string
	UniqueIdentifier string
	TableNumber      int
	MatchID          string
	CreatedAt        time.Time
}

type TournamentMatch struct {
	TournamentID string
	MatchID      string
	TableNumber  int
}

type TournamentRankEntry struct {
	UniqueIdentifier string
	Name             string
	TableNumber      int
	MatchID          string
	Score            int
	Status           string // active, estourou
}
