package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/dntjd1097/allora-checker-bot/internal/models"
)

// AlloraClient handles communication with the Allora API
type AlloraClient struct {
	httpClient *http.Client
	baseURL    string
	apiURL     string
}

// NewAlloraClient creates a new instance of AlloraClient
func NewAlloraClient(baseURL, apiURL string) *AlloraClient {
	return &AlloraClient{
		httpClient: &http.Client{
			Timeout: time.Second * 60,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 100,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		baseURL: baseURL,
		apiURL:  apiURL,
	}
}

// GetUserData fetches user data from the Allora API
func (c *AlloraClient) GetUserData(address string) (*models.AlloraUser, error) {
	url := fmt.Sprintf("%s/api/upshot-api-proxy/allora/forge/user/%s", c.baseURL, address)
	resp, err := c.httpClient.Get(url)
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

// GetScore fetches score for a specific topic and address
func (c *AlloraClient) GetScore(topicID, address string) (*models.ScoreData, error) {
	url := fmt.Sprintf("%s/emissions/v7/inferer_score_ema/%s/%s", c.apiURL, topicID, address)
	resp, err := c.httpClient.Get(url)
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

// GetLowestScore fetches the lowest score for a specific topic
func (c *AlloraClient) GetLowestScore(topicID string) (*models.ScoreData, error) {
	url := fmt.Sprintf("%s/emissions/v7/current_lowest_inferer_score/%s", c.apiURL, topicID)
	resp, err := c.httpClient.Get(url)
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

// GetNetworkInferences fetches network inferences for a specific topic
func (c *AlloraClient) GetNetworkInferences(topicID string) (*models.NetworkInferencesResponse, error) {
	url := fmt.Sprintf("%s/emissions/v7/latest_network_inferences/%s", c.apiURL, topicID)
	resp, err := c.httpClient.Get(url)
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

// IsActive checks if a user is active in a specific competition
func (c *AlloraClient) IsActive(topicID, address string) (bool, float64, error) {
	userScore, err := c.GetScore(topicID, address)
	if err != nil {
		return false, 0, err
	}

	lowestScore, err := c.GetLowestScore(topicID)
	if err != nil {
		return false, 0, err
	}

	userScoreFloat, _ := strconv.ParseFloat(userScore.Score, 64)
	lowestScoreFloat, _ := strconv.ParseFloat(lowestScore.Score, 64)

	return userScoreFloat > lowestScoreFloat, userScoreFloat - lowestScoreFloat, nil
}
