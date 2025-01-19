package service

import (
	"fmt"
	"log"
	"sort"
	"strconv"
	"time"

	"github.com/dntjd1097/allora-checker-bot/internal/config"
	"github.com/dntjd1097/allora-checker-bot/internal/models"
	"github.com/dntjd1097/allora-checker-bot/internal/utils"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type TelegramService struct {
	bot            *tgbotapi.BotAPI
	config         *config.Config
	alloraService  *AlloraService
	historyService *HistoryService
	formatter      *utils.Formatter
}

func NewTelegramService(bot *tgbotapi.BotAPI, config *config.Config, alloraService *AlloraService, historyService *HistoryService) *TelegramService {
	return &TelegramService{
		bot:            bot,
		config:         config,
		alloraService:  alloraService,
		historyService: historyService,
		formatter:      utils.NewFormatter(),
	}
}

// InitBot initializes the Telegram bot with retry mechanism
func InitBot(token string, maxRetries int) (*tgbotapi.BotAPI, error) {
	var bot *tgbotapi.BotAPI
	var err error

	for i := 0; i < maxRetries; i++ {
		bot, err = tgbotapi.NewBotAPI(token)
		if err == nil {
			return bot, nil
		}
		log.Printf("Failed to initialize bot (attempt %d/%d): %v", i+1, maxRetries, err)
		time.Sleep(time.Second * 5)
	}

	return nil, fmt.Errorf("failed to initialize bot after %d attempts", maxRetries)
}

// HandleUpdates processes incoming updates and periodic checks
func (s *TelegramService) HandleUpdates(updates tgbotapi.UpdatesChannel, ticker *time.Ticker) {
	for {
		select {
		case update := <-updates:
			if update.Message != nil {
				s.handleMessage(update.Message)
			}
		case <-ticker.C:
			s.CheckRankChanges()
		}
	}
}

// handleMessage processes incoming messages
func (s *TelegramService) handleMessage(message *tgbotapi.Message) {
	if message.Command() == "rank" {
		s.handleRankCommand(message)
	}
}

// handleRankCommand processes the /rank command
func (s *TelegramService) handleRankCommand(message *tgbotapi.Message) {
	changes := make(map[string]models.RankChangeInfo)
	var users []models.UserRankInfo
	var userData = make(map[string]*models.AlloraUser) // 임시 저장용 맵 추가

	// 먼저 데이터만 가져오기
	for _, address := range s.config.Allora.Address {
		log.Printf("Checking address: %s", address)
		user, err := s.alloraService.FetchUserData(address)
		if err != nil {
			log.Printf("Error fetching user data for %s: %v", address, err)
			continue
		}
		userData[address] = user // 임시 저장

		// 이전 기록 로드
		prevHistory, err := s.historyService.LoadHistory(address)
		if err != nil {
			log.Printf("Error loading history for %s: %v", address, err)
		}

		// 변경사항 계산
		if prevHistory != nil {
			changes[address] = s.calculateChanges(user, prevHistory)
		}

		// UserRankInfo 생성
		userInfo := models.UserRankInfo{
			Name:         fmt.Sprintf("%s %s", user.FirstName, user.LastName),
			Username:     user.Username,
			Ranking:      user.Ranking,
			Points:       user.TotalPoints,
			BadgeName:    user.BadgeName,
			Address:      address,
			Competitions: user.Competitions,
		}
		users = append(users, userInfo)
	}

	// Sort users by ranking
	sort.Slice(users, func(i, j int) bool {
		return users[i].Ranking < users[j].Ranking
	})

	// Format message
	messageText := s.formatter.FormatRankChangeMessage(changes, users)

	// Create and send message
	msg := tgbotapi.NewMessage(message.Chat.ID, messageText)
	msg.ParseMode = "HTML"
	if s.config.Telegram.MessageThread != 0 {
		msg.ReplyToMessageID = s.config.Telegram.MessageThread
	}

	if _, err := s.bot.Send(msg); err != nil {
		log.Printf("Error sending message: %v", err)
	}

	// Save history after sending the message
	for _, user := range users {
		if err := s.historyService.SaveHistory(user.Address, userData[user.Address]); err != nil {
			log.Printf("Error saving history for %s: %v", user.Address, err)
		}
	}
}

// SendRankChangeNotification sends a notification about rank changes
func (s *TelegramService) SendRankChangeNotification(changes map[string]models.RankChangeInfo, users []models.UserRankInfo) {
	// Format message
	messageText := s.formatter.FormatRankChangeMessage(changes, users)

	// Convert chat ID from string to int64
	chatID, err := strconv.ParseInt(s.config.Telegram.ChatID, 10, 64)
	if err != nil {
		log.Printf("Error parsing chat ID: %v", err)
		return
	}

	// Create and send message
	msg := tgbotapi.NewMessage(chatID, messageText)
	msg.ParseMode = "HTML"
	if s.config.Telegram.MessageThread != 0 {
		msg.ReplyToMessageID = s.config.Telegram.MessageThread
	}

	if _, err := s.bot.Send(msg); err != nil {
		log.Printf("Error sending message: %v", err)
	}
}

// CheckRankChanges checks for rank changes and sends notifications
func (s *TelegramService) CheckRankChanges() {
	log.Println("Starting rank change check...")
	changes := make(map[string]models.RankChangeInfo)
	var users []models.UserRankInfo
	userData := make(map[string]*models.AlloraUser)

	// Fetch data and check for changes
	for _, address := range s.config.Allora.Address {
		// Fetch current user data
		userData, err := s.alloraService.GetUserInfo(address)
		if err != nil {
			log.Printf("Error fetching user data for %s: %v", address, err)
			continue
		}

		// Update competition weights for the user
		err = s.alloraService.UpdateCompetitionWeights(userData, address)
		if err != nil {
			log.Printf("Error updating competition weights for %s: %v", address, err)
			continue
		}

		prevHistory, err := s.historyService.LoadHistory(address)
		if err != nil {
			log.Printf("Error loading history for %s: %v", address, err)
			continue
		}

		if prevHistory != nil {
			change := models.RankChangeInfo{
				OverallRankChanged: prevHistory.Ranking != userData.Ranking,
				OverallRankDiff:    prevHistory.Ranking - userData.Ranking,
				PointsDiff:         userData.TotalPoints - prevHistory.TotalPoints,
				CompChanges:        make(map[int]models.CompChangeInfo),
			}

			for _, comp := range userData.Competitions {
				for _, prevComp := range prevHistory.Competitions {
					if comp.ID == prevComp.ID {
						change.CompChanges[comp.ID] = models.CompChangeInfo{
							RankChanged:    prevComp.Ranking != comp.Ranking,
							RankDiff:       prevComp.Ranking - comp.Ranking,
							PointsDiff:     comp.Points - prevComp.Points,
							WeightDiff:     comp.Weight - prevComp.Weight,
							WeightRankDiff: prevComp.WeightRank - comp.WeightRank,
						}
						break
					}
				}
			}
			changes[address] = change
		}

		users = append(users, models.UserRankInfo{
			Name:         fmt.Sprintf("%s %s", userData.FirstName, userData.LastName),
			Username:     userData.Username,
			Ranking:      userData.Ranking,
			Points:       userData.TotalPoints,
			BadgeName:    userData.BadgeName,
			Address:      address,
			Competitions: userData.Competitions,
		})
	}

	// Check if there are any rank changes
	hasRankChanges := false
	for _, change := range changes {
		if change.OverallRankChanged {
			hasRankChanges = true
			break
		}
		for _, compChange := range change.CompChanges {
			if compChange.RankChanged {
				hasRankChanges = true
				break
			}
		}
		if hasRankChanges {
			break
		}
	}

	// Send notification and save history only if there are rank changes
	if hasRankChanges {
		// Send notification
		s.SendRankChangeNotification(changes, users)

		// Save history after sending the notification
		for address, user := range userData {
			if err := s.historyService.SaveHistory(address, user); err != nil {
				log.Printf("Error saving history for %s: %v", address, err)
			}
		}
	}
}

// calculateChanges calculates the differences between current and previous data
func (s *TelegramService) calculateChanges(current *models.AlloraUser, prev *models.UserHistory) models.RankChangeInfo {
	changes := models.RankChangeInfo{
		OverallRankChanged: current.Ranking != prev.Ranking,
		OverallRankDiff:    prev.Ranking - current.Ranking,
		PointsDiff:         current.TotalPoints - prev.TotalPoints,
		CompChanges:        make(map[int]models.CompChangeInfo),
	}

	for _, currentComp := range current.Competitions {
		for _, prevComp := range prev.Competitions {
			if currentComp.ID == prevComp.ID {
				changes.CompChanges[currentComp.ID] = models.CompChangeInfo{
					RankChanged:    currentComp.Ranking != prevComp.Ranking,
					RankDiff:       prevComp.Ranking - currentComp.Ranking,
					PointsDiff:     currentComp.Points - prevComp.Points,
					WeightDiff:     currentComp.Weight - prevComp.Weight,
					WeightRankDiff: prevComp.WeightRank - currentComp.WeightRank,
				}
				break
			}
		}
	}

	return changes
}

// UpdateConfig holds configuration for handling updates
type UpdateConfig struct {
	Bot    *tgbotapi.BotAPI
	Config *config.Config
}

// NewUpdateConfig creates a new UpdateConfig instance
func NewUpdateConfig(bot *tgbotapi.BotAPI, config *config.Config) *UpdateConfig {
	return &UpdateConfig{
		Bot:    bot,
		Config: config,
	}
}

// HandleUpdates processes incoming updates from Telegram
func HandleUpdates(updateConfig *UpdateConfig) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := updateConfig.Bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		// Handle commands
		if update.Message.IsCommand() {
			switch update.Message.Command() {
			case "rank":
				sendRankCommand(updateConfig.Bot, updateConfig.Config, update.Message.Chat.ID)
			case "help":
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, `Available commands:
/rank - Show current rankings
/help - Show this help message`)
				if updateConfig.Config.Telegram.MessageThread != 0 {
					msg.ReplyToMessageID = updateConfig.Config.Telegram.MessageThread
				}
				updateConfig.Bot.Send(msg)
			}
		}
	}
}

// sendRankCommand handles the /rank command
func sendRankCommand(bot *tgbotapi.BotAPI, cfg *config.Config, chatID int64) {
	// Create required services
	alloraService := NewAlloraService(cfg.Allora.API)
	historyService := NewHistoryService("history")
	formatter := utils.NewFormatter()

	var users []models.UserRankInfo
	changes := make(map[string]models.RankChangeInfo)

	// Fetch and process data for each address
	for _, address := range cfg.Allora.Address {
		userData, err := alloraService.FetchUserData(address)
		if err != nil {
			log.Printf("Error fetching user data for %s: %v", address, err)
			continue
		}

		// Get weight information for each competition
		for i := range userData.Competitions {
			comp := &userData.Competitions[i]
			topicID := strconv.Itoa(comp.TopicID)

			// Fetch network inferences for weight information
			networkInferences, err := alloraService.FetchNetworkInferences(topicID)
			if err != nil {
				log.Printf("Error fetching network inferences for topic %s: %v", topicID, err)
				continue
			}

			// Calculate weight rankings
			var weights []models.WeightRank
			for _, w := range networkInferences.InfererWeights {
				weight, _ := strconv.ParseFloat(w.Weight, 64)
				weights = append(weights, models.WeightRank{
					Worker: w.Worker,
					Weight: weight,
				})
			}

			// Sort weights and assign ranks
			sort.Slice(weights, func(i, j int) bool {
				return weights[i].Weight > weights[j].Weight
			})
			for i := range weights {
				weights[i].Rank = i + 1
			}

			// Find user's weight rank
			for _, w := range weights {
				if w.Worker == address {
					comp.Weight = w.Weight
					comp.WeightRank = w.Rank
					comp.TotalWeightParticipants = len(weights)
					break
				}
			}
		}

		// Load previous history
		prevHistory, err := historyService.LoadHistory(address)
		if err != nil {
			log.Printf("Error loading history for %s: %v", address, err)
		}

		// Save current data as history
		if err := historyService.SaveHistory(address, userData); err != nil {
			log.Printf("Error saving history for %s: %v", address, err)
		}

		// Create user rank info
		userInfo := models.UserRankInfo{
			Name:         fmt.Sprintf("%s %s", userData.FirstName, userData.LastName),
			Username:     userData.Username,
			Ranking:      userData.Ranking,
			Points:       userData.TotalPoints,
			BadgeName:    userData.BadgeName,
			Address:      address,
			Competitions: userData.Competitions,
		}
		users = append(users, userInfo)

		// Calculate changes if previous history exists
		if prevHistory != nil {
			change := models.RankChangeInfo{
				OverallRankChanged: prevHistory.Ranking != userData.Ranking,
				OverallRankDiff:    prevHistory.Ranking - userData.Ranking,
				PointsDiff:         userData.TotalPoints - prevHistory.TotalPoints,
				CompChanges:        make(map[int]models.CompChangeInfo),
			}

			// Calculate competition changes
			for _, comp := range userData.Competitions {
				for _, prevComp := range prevHistory.Competitions {
					if comp.ID == prevComp.ID {
						change.CompChanges[comp.ID] = models.CompChangeInfo{
							RankChanged:    prevComp.Ranking != comp.Ranking,
							RankDiff:       prevComp.Ranking - comp.Ranking,
							PointsDiff:     comp.Points - prevComp.Points,
							WeightDiff:     comp.Weight - prevComp.Weight,
							WeightRankDiff: prevComp.WeightRank - comp.WeightRank,
						}
						break
					}
				}
			}
			changes[address] = change
		}
	}

	// Sort users by ranking
	sort.Slice(users, func(i, j int) bool {
		return users[i].Ranking < users[j].Ranking
	})

	// Format and send message
	messageText := formatter.FormatRankChangeMessage(changes, users)
	msg := tgbotapi.NewMessage(chatID, messageText)
	if cfg.Telegram.MessageThread != 0 {
		msg.ReplyToMessageID = cfg.Telegram.MessageThread
	}

	if _, err := bot.Send(msg); err != nil {
		log.Printf("Error sending message: %v", err)
	}
}
