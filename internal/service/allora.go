package service

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/dntjd1097/allora-checker-bot/internal/models"
)

const (
	version = "v8"
)

type AlloraService struct {
	client *http.Client
	api    string
}

// NewAlloraService creates a new instance of AlloraService with configured HTTP client
func NewAlloraService(api string) *AlloraService {
	return &AlloraService{
		client: &http.Client{
			Timeout: time.Second * 60,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 100,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		api: api,
	}
}

// FetchUserData retrieves user data from the Allora API
func (s *AlloraService) FetchUserData(address string) (*models.AlloraUser, error) {
	url := fmt.Sprintf("https://forge.allora.network/api/upshot-api-proxy/allora/forge/user/%s", address)
	resp, err := s.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user data: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var response models.AlloraResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &response.Data, nil
}

// FetchScore retrieves the score for a specific topic and address
func (s *AlloraService) FetchScore(topicID, address string) (*models.ScoreData, error) {
	url := fmt.Sprintf("%s/emissions/%s/inferer_score_ema/%s/%s", s.api, version, topicID, address)
	resp, err := s.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch score: %w", err)
	}
	defer resp.Body.Close()

	var scoreResp models.ScoreResponse
	if err := json.NewDecoder(resp.Body).Decode(&scoreResp); err != nil {
		return nil, fmt.Errorf("failed to decode score response: %w", err)
	}

	return &scoreResp.Score, nil
}

// FetchLowestScore retrieves the lowest score for a specific topic
func (s *AlloraService) FetchLowestScore(topicID string) (*models.ScoreData, error) {
	url := fmt.Sprintf("%s/emissions/%s/current_lowest_inferer_score/%s", s.api, version, topicID)
	resp, err := s.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch lowest score: %w", err)
	}
	defer resp.Body.Close()

	var scoreResp models.ScoreResponse
	if err := json.NewDecoder(resp.Body).Decode(&scoreResp); err != nil {
		return nil, fmt.Errorf("failed to decode lowest score response: %w", err)
	}

	return &scoreResp.Score, nil
}

// FetchNetworkInferences retrieves network inferences for a specific topic
func (s *AlloraService) FetchNetworkInferences(topicID string) (*models.NetworkInferencesResponse, error) {
	url := fmt.Sprintf("%s/emissions/%s/latest_network_inferences/%s", s.api, version, topicID)
	resp, err := s.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch network inferences: %w", err)
	}
	defer resp.Body.Close()

	var result models.NetworkInferencesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode network inferences response: %w", err)
	}

	return &result, nil
}

// UpdateCompetitionWeights updates the weights for competitions
func (s *AlloraService) UpdateCompetitionWeights(userData *models.AlloraUser, address string) error {
	for i := range userData.Competitions {
		topicID := strconv.Itoa(userData.Competitions[i].TopicID)
		networkInferences, err := s.FetchNetworkInferences(topicID)
		if err != nil {
			return fmt.Errorf("failed to fetch network inferences for topic %s: %w", topicID, err)
		}

		weights := s.processWeights(networkInferences.InfererWeights)
		s.updateCompetitionWeight(&userData.Competitions[i], weights, address)
	}
	return nil
}

// processWeights converts and sorts weight information
func (s *AlloraService) processWeights(weights []models.InfererWeight) []models.WeightRank {
	var result []models.WeightRank
	for _, w := range weights {
		weight, _ := strconv.ParseFloat(w.Weight, 64)
		result = append(result, models.WeightRank{
			Worker: w.Worker,
			Weight: weight,
		})
	}

	// Sort weights in descending order
	for i := range result {
		maxIdx := i
		for j := i + 1; j < len(result); j++ {
			if result[j].Weight > result[maxIdx].Weight {
				maxIdx = j
			}
		}
		if maxIdx != i {
			result[i], result[maxIdx] = result[maxIdx], result[i]
		}
	}

	// Assign ranks
	for i := range result {
		result[i].Rank = i + 1
	}

	return result
}

// updateCompetitionWeight updates weight information for a specific competition
func (s *AlloraService) updateCompetitionWeight(comp *models.Competition, weights []models.WeightRank, address string) {
	for _, w := range weights {
		if w.Worker == address {
			comp.Weight = w.Weight
			comp.WeightRank = w.Rank
			comp.TotalWeightParticipants = len(weights)
			break
		}
	}
}

// IsActive checks if a user is active in a specific competition
func (s *AlloraService) IsActive(topicID, address string) (bool, float64, error) {
	userScore, err := s.FetchScore(topicID, address)
	if err != nil {
		return false, 0, err
	}

	lowestScore, err := s.FetchLowestScore(topicID)
	if err != nil {
		return false, 0, err
	}

	userScoreFloat, _ := strconv.ParseFloat(userScore.Score, 64)
	lowestScoreFloat, _ := strconv.ParseFloat(lowestScore.Score, 64)

	return userScoreFloat > lowestScoreFloat, userScoreFloat - lowestScoreFloat, nil
}

// GetUserInfo is an alias for FetchUserData
func (s *AlloraService) GetUserInfo(address string) (*models.AlloraUser, error) {
	return s.FetchUserData(address)
}

// GetNetworkInferences is an alias for FetchNetworkInferences
func (s *AlloraService) GetNetworkInferences(topicID string) (*models.NetworkInferencesResponse, error) {
	return s.FetchNetworkInferences(topicID)
}
