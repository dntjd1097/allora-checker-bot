package service

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dntjd1097/allora-checker-bot/internal/models"
)

type HistoryService struct {
	baseDir string
}

func NewHistoryService(baseDir string) *HistoryService {
	return &HistoryService{
		baseDir: baseDir,
	}
}

// LoadHistory loads historical data for a specific address
func (s *HistoryService) LoadHistory(address string) (*models.UserHistory, error) {
	filename := filepath.Join(s.baseDir, fmt.Sprintf("history_%s.json", address))
	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read history file: %w", err)
	}

	var history models.UserHistory
	if err := json.Unmarshal(data, &history); err != nil {
		return nil, fmt.Errorf("failed to unmarshal history data: %w", err)
	}

	return &history, nil
}

// SaveHistory saves historical data for a specific address
func (s *HistoryService) SaveHistory(address string, userData *models.AlloraUser) error {
	history := models.UserHistory{
		Timestamp:    time.Now(),
		TotalPoints:  userData.TotalPoints,
		Ranking:      userData.Ranking,
		Competitions: make([]models.CompHistory, len(userData.Competitions)),
	}

	for i, comp := range userData.Competitions {
		history.Competitions[i] = models.CompHistory{
			ID:                      comp.ID,
			Points:                  comp.Points,
			Ranking:                 comp.Ranking,
			Weight:                  comp.Weight,
			WeightRank:              comp.WeightRank,
			TotalWeightParticipants: comp.TotalWeightParticipants,
		}
	}

	filename := filepath.Join(s.baseDir, fmt.Sprintf("history_%s.json", address))
	data, err := json.MarshalIndent(history, "", "    ")
	if err != nil {
		return fmt.Errorf("failed to marshal history data: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write history file: %w", err)
	}

	return nil
}
