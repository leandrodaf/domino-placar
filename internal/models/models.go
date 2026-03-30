package models

import "time"

// GameTypeDefault is the default game type for new matches.
const GameTypeDefault = "pontinho"

// DefaultMaxPoints returns the default bust limit for a given game type.
func DefaultMaxPoints(gameType string) int {
	switch gameType {
	case "cem":
		return 100
	case "cento_cinquenta":
		return 150
	case "duzentos":
		return 200
	default:
		return 51 // pontinho, personalizado or unknown
	}
}

type Match struct {
	ID             string    `db:"id"`
	Status         string    `db:"status"`
	BaseURL        string    `db:"base_url"`
	WinnerPlayerID string    `db:"winner_player_id"`
	TurmaID        string    `db:"turma_id"`
	GameType       string    `db:"game_type"`
	MaxPoints      int       `db:"max_points"`
	CreatedAt      time.Time `db:"created_at"`
	PlayerCount    int       `db:"-"` // populated by dashboard queries, not stored
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
	GameType  string
	MaxPoints int
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
	MaxPoints        int
	Status           string // active, estourou
}

// Turma represents a persistent group of players (family, friends, league).
type Turma struct {
	ID                string    `db:"id"`
	Name              string    `db:"name"`
	Description       string    `db:"description"`
	InviteCode        string    `db:"invite_code"`
	IsPrivate         bool      `db:"is_private"`
	CreatedByUniqueID string    `db:"created_by_unique_id"`
	BaseURL           string    `db:"base_url"`
	CreatedAt         time.Time `db:"created_at"`
}

// TurmaMember represents a member of a Turma.
type TurmaMember struct {
	ID               string    `db:"id"`
	TurmaID          string    `db:"turma_id"`
	UniqueIdentifier string    `db:"unique_identifier"`
	Name             string    `db:"name"`
	Role             string    `db:"role"` // admin, member
	JoinedAt         time.Time `db:"joined_at"`
}

// TurmaRankEntry aggregates a player's stats within a Turma.
type TurmaRankEntry struct {
	UniqueIdentifier string
	Name             string
	MatchesPlayed    int
	MatchesWon       int
	TotalScore       int
	BustCount        int
	TotalRoundWins   int
}
