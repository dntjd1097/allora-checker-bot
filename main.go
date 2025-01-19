package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"gopkg.in/yaml.v2"
)

// Config structures
type Config struct {
	Telegram struct {
		Token         string `yaml:"token"`
		ChatID        string `yaml:"chat_id"`
		MessageThread int    `yaml:"message_thread"`
	} `yaml:"telegram"`
	Allora struct {
		RPC     string   `yaml:"rpc"`
		API     string   `yaml:"api"`
		Address []string `yaml:"address"`
	} `yaml:"allora"`
}

// Allora API response structures
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
	ID      int     `json:"id"`
	Name    string  `json:"name"`
	TopicID int     `json:"topic_id"`
	Ranking int     `json:"ranking"`
	Points  float64 `json:"points"`
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
	ID      int     `json:"id"`
	Points  float64 `json:"points"`
	Ranking int     `json:"ranking"`
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

func loadConfig() (*Config, error) {
	file, err := os.ReadFile("config.yaml")
	if err != nil {
		return nil, err
	}

	var config Config
	err = yaml.Unmarshal(file, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

func fetchUserData(address string) (*AlloraUser, error) {
	url := fmt.Sprintf("https://forge.allora.network/api/upshot-api-proxy/allora/forge/user/%s", address)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var response AlloraResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response.Data, nil
}

func fetchScore(api string, topicID, address string) (*ScoreData, error) {
	url := fmt.Sprintf("%s/emissions/v7/inferer_score_ema/%s/%s", api, topicID, address)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var scoreResp ScoreResponse
	if err := json.NewDecoder(resp.Body).Decode(&scoreResp); err != nil {
		return nil, err
	}
	return &scoreResp.Score, nil
}

func fetchLowestScore(api string, topicID string) (*ScoreData, error) {
	url := fmt.Sprintf("%s/emissions/v7/current_lowest_inferer_score/%s", api, topicID)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var scoreResp ScoreResponse
	if err := json.NewDecoder(resp.Body).Decode(&scoreResp); err != nil {
		return nil, err
	}
	return &scoreResp.Score, nil
}

func loadHistory(address string) (*UserHistory, error) {
	filename := fmt.Sprintf("history/%s.json", address)
	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var history UserHistory
	if err := json.Unmarshal(data, &history); err != nil {
		return nil, err
	}
	return &history, nil
}

func saveHistory(address string, history *UserHistory) error {
	if err := os.MkdirAll("history", 0755); err != nil {
		return fmt.Errorf("failed to create history directory: %v", err)
	}

	filename := fmt.Sprintf("history/%s.json", address)
	data, err := json.Marshal(history)
	if err != nil {
		return err
	}
	return os.WriteFile(filename, data, 0644)
}

func formatUserInfo(user *AlloraUser, address string, config *Config) string {
	if user == nil {
		return fmt.Sprintf("❌ No data available for address: %s\n", address)
	}

	// Load previous history
	prevHistory, _ := loadHistory(address)

	var sb strings.Builder

	// Header with user info
	sb.WriteString(fmt.Sprintf("📊 %s %s | %s\n", user.FirstName, user.LastName, user.Username))
	sb.WriteString(fmt.Sprintf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n"))

	// Compare with previous data if available
	if prevHistory != nil {
		rankDiff := prevHistory.Ranking - user.Ranking
		pointsDiff := user.TotalPoints - prevHistory.TotalPoints

		rankChange := ""
		if rankDiff > 0 {
			rankChange = fmt.Sprintf(" (⬆️ +%d)", rankDiff)
		} else if rankDiff < 0 {
			rankChange = fmt.Sprintf(" (⬇️ %d)", rankDiff)
		}

		pointsChange := ""
		if pointsDiff > 0 {
			pointsChange = fmt.Sprintf(" (⬆️ +%.2f)", pointsDiff)
		} else if pointsDiff < 0 {
			pointsChange = fmt.Sprintf(" (⬇️ %.2f)", pointsDiff)
		}

		sb.WriteString(fmt.Sprintf("🏆 Rank #%-3d%s | ⭐ Points: %-6.2f%s | 🏅 %s\n",
			user.Ranking, rankChange, user.TotalPoints, pointsChange, user.BadgeName))
	} else {
		sb.WriteString(fmt.Sprintf("🏆 Rank #%-3d | ⭐ Points: %-6.2f | 🏅 %s\n",
			user.Ranking, user.TotalPoints, user.BadgeName))
	}

	sb.WriteString(fmt.Sprintf("📝 %s\n\n", user.BadgeDescription))

	// Competition details with active status
	if len(user.Competitions) > 0 {
		sort.Slice(user.Competitions, func(i, j int) bool {
			return user.Competitions[i].ID < user.Competitions[j].ID
		})

		sb.WriteString("🎯 Active Competitions:\n")
		for _, comp := range user.Competitions {
			// Find previous competition data
			var prevComp *CompHistory
			if prevHistory != nil {
				for _, pc := range prevHistory.Competitions {
					if pc.ID == comp.ID {
						prevComp = &pc
						break
					}
				}
			}

			// Fetch current scores
			userScore, err := fetchScore(config.Allora.API, strconv.Itoa(comp.TopicID), address)
			if err != nil {
				continue
			}

			lowestScore, err := fetchLowestScore(config.Allora.API, strconv.Itoa(comp.TopicID))
			if err != nil {
				continue
			}

			userScoreFloat, _ := strconv.ParseFloat(userScore.Score, 64)
			lowestScoreFloat, _ := strconv.ParseFloat(lowestScore.Score, 64)

			// Format competition info with changes
			sb.WriteString(fmt.Sprintf("%d. %s\n", comp.ID, comp.Name))

			if prevComp != nil {
				rankDiff := prevComp.Ranking - comp.Ranking
				pointsDiff := comp.Points - prevComp.Points

				rankChange := ""
				if rankDiff > 0 {
					rankChange = fmt.Sprintf(" (⬆️ +%d)", rankDiff)
				} else if rankDiff < 0 {
					rankChange = fmt.Sprintf(" (⬇️ %d)", rankDiff)
				}

				pointsChange := ""
				if pointsDiff > 0 {
					pointsChange = fmt.Sprintf(" (⬆️ +%.2f)", pointsDiff)
				} else if pointsDiff < 0 {
					pointsChange = fmt.Sprintf(" (⬇️ %.2f)", pointsDiff)
				}

				sb.WriteString(fmt.Sprintf("  ├ Rank: #%-3d%s | Points: %-6.2f%s\n",
					comp.Ranking, rankChange, comp.Points, pointsChange))
			} else {
				sb.WriteString(fmt.Sprintf("  ├ Rank: #%-3d | Points: %-6.2f\n",
					comp.Ranking, comp.Points))
			}

			if userScoreFloat > lowestScoreFloat {
				scoreDiff := userScoreFloat - lowestScoreFloat
				sb.WriteString(fmt.Sprintf("  └ ✅ Active | Diff: %.6f\n", scoreDiff))
			} else {
				sb.WriteString("  └ ❌ Inactive\n")
			}
		}
	}

	// Save current data as history
	newHistory := UserHistory{
		Timestamp:    time.Now(),
		TotalPoints:  user.TotalPoints,
		Ranking:      user.Ranking,
		Competitions: make([]CompHistory, len(user.Competitions)),
	}

	for i, comp := range user.Competitions {
		newHistory.Competitions[i] = CompHistory{
			ID:      comp.ID,
			Points:  comp.Points,
			Ranking: comp.Ranking,
		}
	}

	_ = saveHistory(address, &newHistory)

	return sb.String()
}

func handleRankCommand(bot *tgbotapi.BotAPI, config *Config) {
	log.Println("Starting handleRankCommand...")

	// 고루틴으로 데이터 수집 병렬화
	var wg sync.WaitGroup
	userChan := make(chan UserRankInfo, len(config.Allora.Address))
	errorChan := make(chan error, len(config.Allora.Address))

	// 병렬로 사용자 데이터 수집
	for _, address := range config.Allora.Address {
		wg.Add(1)
		go func(addr string) {
			defer wg.Done()

			userData, err := fetchUserData(addr)
			if err != nil {
				errorChan <- fmt.Errorf("error fetching data for %s: %v", addr, err)
				return
			}

			userChan <- UserRankInfo{
				Name:         fmt.Sprintf("%s %s", userData.FirstName, userData.LastName),
				Username:     userData.Username,
				Ranking:      userData.Ranking,
				Points:       userData.TotalPoints,
				BadgeName:    userData.BadgeName,
				Address:      addr,
				Competitions: userData.Competitions,
			}
		}(address)
	}

	// 모든 고루틴이 완료될 때까지 대기
	go func() {
		wg.Wait()
		close(userChan)
		close(errorChan)
	}()

	// 결과 수집
	var users []UserRankInfo
	for user := range userChan {
		users = append(users, user)
	}

	// 에러 로깅
	for err := range errorChan {
		log.Printf("Error: %v", err)
	}

	// 결과가 없으면 종료
	if len(users) == 0 {
		log.Println("No user data collected")
		return
	}

	// Sort users by overall ranking
	sort.Slice(users, func(i, j int) bool {
		return users[i].Ranking < users[j].Ranking
	})

	var sb strings.Builder
	sb.WriteString("🤖 Allora Network Status Report\n")
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

	// Overall Rankings
	sb.WriteString("📊 Overall Rankings:\n")
	for i, user := range users {
		prevHistory, _ := loadHistory(user.Address)

		rankChange := ""
		pointsChange := ""
		if prevHistory != nil {
			rankDiff := prevHistory.Ranking - user.Ranking
			pointsDiff := user.Points - prevHistory.TotalPoints

			if rankDiff > 0 {
				rankChange = fmt.Sprintf(" (⬆️ +%d)", rankDiff)
			} else if rankDiff < 0 {
				rankChange = fmt.Sprintf(" (⬇️ %d)", rankDiff)
			}

			if pointsDiff > 0 {
				pointsChange = fmt.Sprintf(" (⬆️ +%.2f)", pointsDiff)
			} else if pointsDiff < 0 {
				pointsChange = fmt.Sprintf(" (⬇️ %.2f)", pointsDiff)
			}
		}

		sb.WriteString(fmt.Sprintf("%d. %s (@%s)\n", i+1, user.Name, user.Username))
		sb.WriteString(fmt.Sprintf("   Rank: #%-3d%s | Points: %-6.2f%s | 🏅 %s\n\n",
			user.Ranking, rankChange, user.Points, pointsChange, user.BadgeName))
	}

	// Competition Rankings (병렬 처리)
	var compWg sync.WaitGroup
	compChan := make(chan struct {
		ID    int
		Name  string
		Users []struct {
			Name     string
			Username string
			Ranking  int
			Points   float64
			Address  string
		}
	}, len(users)*len(users[0].Competitions))

	// Get unique competition IDs
	compIDs := make(map[int]string)
	for _, user := range users {
		for _, comp := range user.Competitions {
			compIDs[comp.ID] = comp.Name
		}
	}

	// Sort competition IDs
	var sortedCompIDs []int
	for id := range compIDs {
		sortedCompIDs = append(sortedCompIDs, id)
	}
	sort.Ints(sortedCompIDs)

	// 각 competition 데이터 병렬 처리
	for _, compID := range sortedCompIDs {
		compWg.Add(1)
		go func(id int, name string) {
			defer compWg.Done()

			var compUsers []struct {
				Name     string
				Username string
				Ranking  int
				Points   float64
				Address  string
			}

			for _, user := range users {
				for _, comp := range user.Competitions {
					if comp.ID == id {
						compUsers = append(compUsers, struct {
							Name     string
							Username string
							Ranking  int
							Points   float64
							Address  string
						}{
							Name:     user.Name,
							Username: user.Username,
							Ranking:  comp.Ranking,
							Points:   comp.Points,
							Address:  user.Address,
						})
						break
					}
				}
			}

			sort.Slice(compUsers, func(i, j int) bool {
				return compUsers[i].Ranking < compUsers[j].Ranking
			})

			compChan <- struct {
				ID    int
				Name  string
				Users []struct {
					Name     string
					Username string
					Ranking  int
					Points   float64
					Address  string
				}
			}{
				ID:    id,
				Name:  name,
				Users: compUsers,
			}
		}(compID, compIDs[compID])
	}

	// 모든 competition 처리 완료 대기
	go func() {
		compWg.Wait()
		close(compChan)
	}()

	// Competition 결과 수집 및 출력
	sb.WriteString("\n🎯 Competition Rankings:\n")
	for comp := range compChan {
		sb.WriteString(fmt.Sprintf("\n📌 %s (ID: %d):\n", comp.Name, comp.ID))
		for i, cu := range comp.Users {
			prevHistory, _ := loadHistory(cu.Address)

			rankChange := ""
			pointsChange := ""
			if prevHistory != nil {
				for _, pc := range prevHistory.Competitions {
					if pc.ID == comp.ID {
						rankDiff := pc.Ranking - cu.Ranking
						pointsDiff := cu.Points - pc.Points

						if rankDiff > 0 {
							rankChange = fmt.Sprintf(" (⬆️ +%d)", rankDiff)
						} else if rankDiff < 0 {
							rankChange = fmt.Sprintf(" (⬇️ %d)", rankDiff)
						}

						if pointsDiff > 0 {
							pointsChange = fmt.Sprintf(" (⬆️ +%.2f)", pointsDiff)
						} else if pointsDiff < 0 {
							pointsChange = fmt.Sprintf(" (⬇️ %.2f)", pointsDiff)
						}
						break
					}
				}
			}

			sb.WriteString(fmt.Sprintf("%d. %s (@%s)\n", i+1, cu.Name, cu.Username))
			sb.WriteString(fmt.Sprintf("   Rank: #%-3d%s | Points: %-6.2f%s\n",
				cu.Ranking, rankChange, cu.Points, pointsChange))
		}
	}

	// Send message to Telegram
	chatID, err := strconv.ParseInt(config.Telegram.ChatID, 10, 64)
	if err != nil {
		log.Printf("Error parsing chat ID: %v", err)
		return
	}
	log.Printf("Preparing to send message to chat ID: %d", chatID)

	msg := tgbotapi.NewMessage(chatID, sb.String())
	if config.Telegram.MessageThread != 0 {
		msg.ReplyToMessageID = config.Telegram.MessageThread
		log.Printf("Setting message thread ID: %d", config.Telegram.MessageThread)
	}

	log.Println("Attempting to send message...")
	result, err := bot.Send(msg)
	if err != nil {
		log.Printf("Error sending message: %v", err)
		return
	}
	log.Printf("Message sent successfully, message ID: %d", result.MessageID)
}

func initBot(token string, maxRetries int) (*tgbotapi.BotAPI, error) {
	log.Printf("Initializing bot with %d retries...", maxRetries)

	client := &http.Client{
		Timeout: time.Second * 30,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   100,
		},
	}

	var bot *tgbotapi.BotAPI
	var err error

	for i := 0; i < maxRetries; i++ {
		log.Printf("Attempt %d/%d to initialize bot", i+1, maxRetries)
		bot, err = tgbotapi.NewBotAPIWithClient(token, tgbotapi.APIEndpoint, client)
		if err == nil {
			log.Printf("Bot initialized successfully")
			// Test the bot connection
			if me, err := bot.GetMe(); err == nil {
				log.Printf("Bot connected as: %s (@%s)", me.FirstName, me.UserName)
				return bot, nil
			}
		}

		log.Printf("Attempt %d/%d failed: %v", i+1, maxRetries, err)
		time.Sleep(time.Second * 5)
	}

	return nil, fmt.Errorf("failed to initialize bot after %d attempts: %v", maxRetries, err)
}

func main() {
	// Load configuration
	config, err := loadConfig()
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	// Initialize Telegram bot with retry
	bot, err := initBot(config.Telegram.Token, 3)
	if err != nil {
		log.Fatalf("Error initializing bot: %v", err)
	}

	// Set up updates configuration
	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 30

	updates := bot.GetUpdatesChan(updateConfig)

	log.Println("Bot started successfully")

	// Create ticker for periodic rank checking
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	// Handle updates and periodic checks
	go func() {
		for {
			select {
			case update := <-updates:
				if update.Message == nil {
					continue
				}

				// Check if message is a reply and matches the thread
				if update.Message.ReplyToMessage == nil || update.Message.ReplyToMessage.MessageID != config.Telegram.MessageThread {
					continue
				}

				// Check if it's the /rank command
				if update.Message.Command() == "rank" {
					log.Println("Rank command received")
					handleRankCommand(bot, config)
				}

			case <-ticker.C:
				checkRankChanges(bot, config)
			}
		}
	}()

	// Keep the main goroutine running
	select {}
}

func checkRankChanges(bot *tgbotapi.BotAPI, config *Config) {
	var rankChanged bool
	var rankIncreased bool
	var users []UserRankInfo

	// Check each user's ranking
	for _, address := range config.Allora.Address {
		prevHistory, err := loadHistory(address)
		if err != nil {
			log.Printf("Error loading history for %s: %v", address, err)
			continue
		}

		// Skip if no previous history
		if prevHistory == nil {
			continue
		}

		userData, err := fetchUserData(address)
		if err != nil {
			log.Printf("Error fetching data for %s: %v", address, err)
			continue
		}

		// Check if overall ranking changed
		if userData.Ranking != prevHistory.Ranking {
			rankChanged = true
			if userData.Ranking < prevHistory.Ranking {
				rankIncreased = true
			}
		}

		// Check if any competition ranking changed
		for _, comp := range userData.Competitions {
			for _, prevComp := range prevHistory.Competitions {
				if comp.ID == prevComp.ID && comp.Ranking != prevComp.Ranking {
					rankChanged = true
					if comp.Ranking < prevComp.Ranking {
						rankIncreased = true
					}
					break
				}
			}
			if rankChanged {
				break
			}
		}

		users = append(users, UserRankInfo{
			Name:         fmt.Sprintf("%s %s", userData.FirstName, userData.LastName),
			Username:     userData.Username,
			Ranking:      userData.Ranking,
			Points:       userData.TotalPoints,
			BadgeName:    userData.BadgeName,
			Address:      address,
			Competitions: userData.Competitions,
		})

		// Save new history
		newHistory := UserHistory{
			Timestamp:    time.Now(),
			TotalPoints:  userData.TotalPoints,
			Ranking:      userData.Ranking,
			Competitions: make([]CompHistory, len(userData.Competitions)),
		}

		for i, comp := range userData.Competitions {
			newHistory.Competitions[i] = CompHistory{
				ID:      comp.ID,
				Points:  comp.Points,
				Ranking: comp.Ranking,
			}
		}

		if err := saveHistory(address, &newHistory); err != nil {
			log.Printf("Error saving history for %s: %v", address, err)
		}
	}

	// Send message if any ranking changed
	if rankChanged {
		log.Printf("Rank change detected (Increased: %v), sending notification", rankIncreased)

		// Sort users by overall ranking
		sort.Slice(users, func(i, j int) bool {
			return users[i].Ranking < users[j].Ranking
		})

		var sb strings.Builder
		// Change header based on rank change direction
		if rankIncreased {
			sb.WriteString("🎉 Rank Increase Alert!\n")
		} else {
			sb.WriteString("⚠️ Rank Decrease Alert!\n")
		}
		sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

		// Overall Rankings
		sb.WriteString("📊 Overall Rankings:\n")
		for i, user := range users {
			prevHistory, _ := loadHistory(user.Address)

			rankChange := ""
			pointsChange := ""
			if prevHistory != nil {
				rankDiff := prevHistory.Ranking - user.Ranking
				pointsDiff := user.Points - prevHistory.TotalPoints

				if rankDiff > 0 {
					rankChange = fmt.Sprintf(" (⬆️ +%d)", rankDiff)
				} else if rankDiff < 0 {
					rankChange = fmt.Sprintf(" (⬇️ %d)", rankDiff)
				}

				if pointsDiff > 0 {
					pointsChange = fmt.Sprintf(" (⬆️ +%.2f)", pointsDiff)
				} else if pointsDiff < 0 {
					pointsChange = fmt.Sprintf(" (⬇️ %.2f)", pointsDiff)
				}
			}

			sb.WriteString(fmt.Sprintf("%d. %s (@%s)\n", i+1, user.Name, user.Username))
			sb.WriteString(fmt.Sprintf("   Rank: #%-3d%s | Points: %-6.2f%s | 🏅 %s\n\n",
				user.Ranking, rankChange, user.Points, pointsChange, user.BadgeName))
		}

		// Competition Rankings
		sb.WriteString("\n🎯 Competition Rankings:\n")

		// Get unique competition IDs
		compIDs := make(map[int]string)
		for _, user := range users {
			for _, comp := range user.Competitions {
				compIDs[comp.ID] = comp.Name
			}
		}

		// Sort competition IDs
		var sortedCompIDs []int
		for id := range compIDs {
			sortedCompIDs = append(sortedCompIDs, id)
		}
		sort.Ints(sortedCompIDs)

		// Show rankings for each competition
		for _, compID := range sortedCompIDs {
			sb.WriteString(fmt.Sprintf("\n📌 %s (ID: %d):\n", compIDs[compID], compID))

			var compUsers []struct {
				Name     string
				Username string
				Ranking  int
				Points   float64
				Address  string
			}

			for _, user := range users {
				for _, comp := range user.Competitions {
					if comp.ID == compID {
						compUsers = append(compUsers, struct {
							Name     string
							Username string
							Ranking  int
							Points   float64
							Address  string
						}{
							Name:     user.Name,
							Username: user.Username,
							Ranking:  comp.Ranking,
							Points:   comp.Points,
							Address:  user.Address,
						})
						break
					}
				}
			}

			sort.Slice(compUsers, func(i, j int) bool {
				return compUsers[i].Ranking < compUsers[j].Ranking
			})

			for i, cu := range compUsers {
				prevHistory, _ := loadHistory(cu.Address)

				rankChange := ""
				pointsChange := ""
				if prevHistory != nil {
					for _, pc := range prevHistory.Competitions {
						if pc.ID == compID {
							rankDiff := pc.Ranking - cu.Ranking
							pointsDiff := cu.Points - pc.Points

							if rankDiff > 0 {
								rankChange = fmt.Sprintf(" (⬆️ +%d)", rankDiff)
							} else if rankDiff < 0 {
								rankChange = fmt.Sprintf(" (⬇️ %d)", rankDiff)
							}

							if pointsDiff > 0 {
								pointsChange = fmt.Sprintf(" (⬆️ +%.2f)", pointsDiff)
							} else if pointsDiff < 0 {
								pointsChange = fmt.Sprintf(" (⬇️ %.2f)", pointsDiff)
							}
							break
						}
					}
				}

				sb.WriteString(fmt.Sprintf("%d. %s (@%s)\n", i+1, cu.Name, cu.Username))
				sb.WriteString(fmt.Sprintf("   Rank: #%-3d%s | Points: %-6.2f%s\n",
					cu.Ranking, rankChange, cu.Points, pointsChange))
			}
		}

		// Send message to Telegram
		chatID, err := strconv.ParseInt(config.Telegram.ChatID, 10, 64)
		if err != nil {
			log.Printf("Error parsing chat ID: %v", err)
			return
		}

		msg := tgbotapi.NewMessage(chatID, sb.String())
		if config.Telegram.MessageThread != 0 {
			msg.ReplyToMessageID = config.Telegram.MessageThread
		}

		if _, err := bot.Send(msg); err != nil {
			log.Printf("Error sending message: %v", err)
			return
		}
		log.Println("Alert message sent successfully")
	}
}
