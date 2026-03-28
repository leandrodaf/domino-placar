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

// TileStats summarizes the tile coverage in a round.
type TileStats struct {
	HandTiles      []string // tiles in players' hands
	TableTiles     []string // tiles detected on the table photo
	SeenTiles      []string // union of hand + table (no duplicates)
	TotalInSet     int      // total tiles in the game (28 or 55)
	SeenCount      int
	RemainingCount int  // tiles likely in the boneyard
	HasTablePhoto  bool // whether the table photo was taken
}

// GlobalStat aggregates a player's statistics (by unique_identifier) across all matches.
type GlobalStat struct {
	UniqueIdentifier string
	Name             string
	MatchesPlayed    int
	MatchesWon       int
	BustCount        int
	MaxBustScore     int
	TotalRoundWins   int
}

// ZoeiraStat is used for fun/achievement rankings in the Hall of Fame.
type ZoeiraStat struct {
	UniqueIdentifier string
	Name             string
	Count            int
}

// NicknameNomination is a nickname proposed for a player.
type NicknameNomination struct {
	ID                string
	NominatedUniqueID string
	NominatedName     string
	MatchID           string
	Nickname          string
	VoteCount         int
	ProposerUniqueID  string
	CreatedAt         time.Time
}

// NicknameVote registra que um unique_id votou em uma nomination.
type NicknameVote struct {
	NominationID  string
	VoterUniqueID string
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
