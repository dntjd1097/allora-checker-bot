package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

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

func formatUserInfo(user *AlloraUser, address string, config *Config) string {
	if user == nil {
		return fmt.Sprintf("❌ No data available for address: %s\n", address)
	}

	var sb strings.Builder

	// Header with user info
	sb.WriteString(fmt.Sprintf("📊 %s %s | %s\n", user.FirstName, user.LastName, user.Username))
	sb.WriteString(fmt.Sprintf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n"))
	sb.WriteString(fmt.Sprintf("🏆 Rank #%-3d | ⭐ Points: %-6.2f | 🏅 %s\n",
		user.Ranking, user.TotalPoints, user.BadgeName))
	sb.WriteString(fmt.Sprintf("📝 %s\n\n", user.BadgeDescription))

	// Competition details with active status
	if len(user.Competitions) > 0 {
		sb.WriteString("🎯 Active Competitions:\n")
		for _, comp := range user.Competitions {
			// Fetch current scores
			userScore, err := fetchScore(config.Allora.API, strconv.Itoa(comp.TopicID), address)
			if err != nil {
				continue
			}

			lowestScore, err := fetchLowestScore(config.Allora.API, strconv.Itoa(comp.TopicID))
			if err != nil {
				continue
			}

			// Convert scores to float64 for comparison
			userScoreFloat, _ := strconv.ParseFloat(userScore.Score, 64)
			lowestScoreFloat, _ := strconv.ParseFloat(lowestScore.Score, 64)

			// Format competition name to be more compact
			sb.WriteString(fmt.Sprintf("• %s\n", comp.Name))
			sb.WriteString(fmt.Sprintf("  ├ Rank: #%-3d | Points: %-6.2f\n",
				comp.Ranking, comp.Points))

			if userScoreFloat > lowestScoreFloat {
				scoreDiff := userScoreFloat - lowestScoreFloat
				sb.WriteString(fmt.Sprintf("  └ ✅ Active | Diff: %.6f\n", scoreDiff))
			} else {
				sb.WriteString("  └ ❌ Inactive\n")
			}
		}
	}

	return sb.String()
}

func handleRankCommand(bot *tgbotapi.BotAPI, config *Config) {
	message := "🤖 Allora Network Status Report\n" +
		"━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n"

	for _, address := range config.Allora.Address {
		userData, err := fetchUserData(address)
		if err != nil {
			log.Printf("Error fetching data for %s: %v", address, err)
			message += fmt.Sprintf("Error fetching data for %s\n", address)
			continue
		}

		message += formatUserInfo(userData, address, config)
		message += "\n"
	}

	// Convert string chat ID to int64
	chatID, err := strconv.ParseInt(config.Telegram.ChatID, 10, 64)
	if err != nil {
		log.Printf("Error parsing chat ID: %v", err)
		return
	}

	msg := tgbotapi.NewMessage(chatID, message)
	// Thread ID를 설정하는 올바른 방법
	if config.Telegram.MessageThread != 0 {
		msg.ReplyToMessageID = config.Telegram.MessageThread
	}

	_, err = bot.Send(msg)
	if err != nil {
		log.Printf("Error sending message: %v", err)
	}
}

func main() {
	// Load configuration
	config, err := loadConfig()
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	// Initialize Telegram bot
	bot, err := tgbotapi.NewBotAPI(config.Telegram.Token)
	if err != nil {
		log.Fatalf("Error initializing bot: %v", err)
	}

	// Set up updates configuration
	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 30

	updates := bot.GetUpdatesChan(updateConfig)

	log.Println("Bot started successfully")

	// Handle updates
	for update := range updates {
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
			go handleRankCommand(bot, config)
		}
	}
}
