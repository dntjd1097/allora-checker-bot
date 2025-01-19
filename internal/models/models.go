package models

import "time"

// API Response Structures
type AlloraResponse struct {
	RequestID string     `json:"request_id"`
	Status    bool       `json:"status"`
	Data      AlloraUser `json:"data"`
}

type AlloraUser struct {
	FirstName        string        `json:"first_name"`
	LastName         string        `json:"last_name"`
	Username         string        `json:"username"`
	CosmosAddress    string        `json:"cosmos_address"`
	TotalPoints      float64       `json:"total_points"`
	Ranking          int           `json:"ranking"`
	BadgePercentile  float64       `json:"badge_percentile"`
	BadgeName        string        `json:"badge_name"`
	BadgeDescription string        `json:"badge_description"`
	Competitions     []Competition `json:"competitions"`
}

type Competition struct {
	ID                      int     `json:"id"`
	Name                    string  `json:"name"`
	TopicID                 int     `json:"topic_id"`
	Points                  float64 `json:"points"`
	Ranking                 int     `json:"ranking"`
	Weight                  float64 `json:"-"`
	WeightRank              int     `json:"-"`
	TotalWeightParticipants int     `json:"-"`
}

// Add new structures for API responses
type ScoreResponse struct {
	Score ScoreData `json:"score"`
}

type ScoreData struct {
	TopicID     string `json:"topic_id"`
	BlockHeight string `json:"block_height"`
	Address     string `json:"address"`
	Score       string `json:"score"`
}

// Add new structure for historical data
type UserHistory struct {
	Timestamp    time.Time     `json:"timestamp"`
	TotalPoints  float64       `json:"total_points"`
	Ranking      int           `json:"ranking"`
	Competitions []CompHistory `json:"competitions"`
}

type CompHistory struct {
	ID                      int     `json:"id"`
	Points                  float64 `json:"points"`
	Ranking                 int     `json:"ranking"`
	Weight                  float64 `json:"weight"`
	WeightRank              int     `json:"weight_rank"`
	TotalWeightParticipants int     `json:"total_weight_participants"`
}

// Add new structure for ranking display
type UserRankInfo struct {
	Name         string
	Username     string
	Ranking      int
	Points       float64
	BadgeName    string
	Address      string
	Competitions []Competition
}

// Add new structures for network inferences
type NetworkInferencesResponse struct {
	NetworkInferences struct {
		TopicID       string         `json:"topic_id"`
		InfererValues []InfererValue `json:"inferer_values"`
	} `json:"network_inferences"`
	InfererWeights []InfererWeight `json:"inferer_weights"`
}

type InfererValue struct {
	Worker string `json:"worker"`
	Value  string `json:"value"`
}

type InfererWeight struct {
	Worker string `json:"worker"`
	Weight string `json:"weight"`
}

type WeightRank struct {
	Worker string
	Weight float64
	Rank   int
}

// Add new structures for rank changes
type RankChangeInfo struct {
	OverallRankChanged bool                   `json:"overall_rank_changed"`
	OverallRankDiff    int                    `json:"overall_rank_diff"`
	PointsDiff         float64                `json:"points_diff"`
	CompChanges        map[int]CompChangeInfo `json:"comp_changes"`
}

type CompChangeInfo struct {
	RankChanged    bool    `json:"rank_changed"`
	RankDiff       int     `json:"rank_diff"`
	PointsDiff     float64 `json:"points_diff"`
	WeightDiff     float64 `json:"weight_diff"`
	WeightRankDiff int     `json:"weight_rank_diff"`
}
