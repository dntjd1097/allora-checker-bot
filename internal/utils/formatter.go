package utils

import (
	"fmt"
	"sort"
	"strings"

	"github.com/dntjd1097/allora-checker-bot/internal/models"
)

type Formatter struct{}

func NewFormatter() *Formatter {
	return &Formatter{}
}

// FormatUserInfo formats user information including competitions and rank changes
func (f *Formatter) FormatUserInfo(user *models.AlloraUser, prevHistory *models.UserHistory) string {
	var sb strings.Builder

	// Format header
	f.writeHeader(&sb, "👤 User Information")

	// Format basic info with changes
	rankChange := ""
	pointsChange := ""
	if prevHistory != nil {
		rankDiff := prevHistory.Ranking - user.Ranking
		pointsDiff := user.TotalPoints - prevHistory.TotalPoints

		rankChange = f.formatChange(float64(rankDiff), "")
		pointsChange = f.formatChange(pointsDiff, "%.2f")
	}

	// Write user details
	f.writeUserDetails(&sb, user, rankChange, pointsChange)

	return sb.String()
}

// FormatCompetitionInfo formats competition information with rank changes
func (f *Formatter) FormatCompetitionInfo(comp models.Competition, prevComp *models.CompHistory) string {
	var sb strings.Builder

	// Calculate changes
	changes := f.calculateCompChanges(comp, prevComp)

	// Write competition details
	f.writeCompetitionDetails(&sb, comp, changes)

	return sb.String()
}

// FormatRankChangeMessage formats rank change alerts for multiple users
func (f *Formatter) FormatRankChangeMessage(changes map[string]models.RankChangeInfo, users []models.UserRankInfo) string {
	var sb strings.Builder

	// 전체 순위
	sb.WriteString("📊 Overall Rankings\n")
	sb.WriteString("─────────────\n")
	for i, user := range users {
		if change, ok := changes[user.Address]; ok {
			rankChange := f.formatChange(float64(change.OverallRankDiff), "")
			pointsChange := f.formatChange(change.PointsDiff, "%.2f")

			sb.WriteString(fmt.Sprintf("%d. %s (@%s)\n", i+1, user.Name, user.Username))
			sb.WriteString(fmt.Sprintf("└ #%-3d%-8s | %-6.2f%-8s | 🏅 %s\n",
				user.Ranking, rankChange,
				user.Points, pointsChange,
				user.BadgeName))
		}
	}

	// Competition ID 순서대로 정렬을 위한 ID 슬라이스 생성
	var compIDs []int
	compMap := f.buildCompetitionMap(users)
	for id := range compMap {
		compIDs = append(compIDs, id)
	}
	sort.Ints(compIDs)

	// 정렬된 ID 순서대로 경쟁 부문별 순위 작성
	for _, compID := range compIDs {
		name := compMap[compID]
		sb.WriteString(fmt.Sprintf("\n🎯 [%d] %s\n", compID, name))
		sb.WriteString("─────────────\n")
		for i, user := range users {
			if change, ok := changes[user.Address]; ok {
				for _, comp := range user.Competitions {
					if comp.ID == compID {
						if compChange, ok := change.CompChanges[comp.ID]; ok {
							rankChange := f.formatChange(float64(compChange.RankDiff), "")
							pointsChange := f.formatChange(compChange.PointsDiff, "%.2f")

							sb.WriteString(fmt.Sprintf("%d. %s (@%s)\n", i+1, user.Name, user.Username))
							sb.WriteString(fmt.Sprintf("     #%-3d%-8s | %-6.2f%-8s | #%d/%d %.5f\n",
								comp.Ranking, rankChange,
								comp.Points, pointsChange,
								comp.WeightRank, comp.TotalWeightParticipants,
								comp.Weight))
						}
						break
					}
				}
			}
		}
	}

	return sb.String()
}

// Helper methods
func (f *Formatter) writeHeader(sb *strings.Builder, title string) {
	sb.WriteString(title + "\n")
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")
}

func (f *Formatter) writeUserDetails(sb *strings.Builder, user *models.AlloraUser, rankChange, pointsChange string) {
	sb.WriteString(fmt.Sprintf("Name: %s %s (@%s)\n", user.FirstName, user.LastName, user.Username))
	sb.WriteString(fmt.Sprintf("Rank: #%-3d%s | Points: %-6.2f%s\n",
		user.Ranking, rankChange, user.TotalPoints, pointsChange))
	sb.WriteString(fmt.Sprintf("Badge: %s (%.2f%%)\n\n", user.BadgeName, user.BadgePercentile))
}

type CompetitionChanges struct {
	RankChange   string
	PointsChange string
}

func (f *Formatter) calculateCompChanges(comp models.Competition, prevComp *models.CompHistory) CompetitionChanges {
	changes := CompetitionChanges{}
	if prevComp != nil {
		rankDiff := prevComp.Ranking - comp.Ranking
		pointsDiff := comp.Points - prevComp.Points

		changes.RankChange = f.formatChange(float64(rankDiff), "")
		changes.PointsChange = f.formatChange(pointsDiff, "%.2f")
	}
	return changes
}

func (f *Formatter) writeCompetitionDetails(sb *strings.Builder, comp models.Competition, changes CompetitionChanges) {
	sb.WriteString(fmt.Sprintf("%d. %s\n", comp.ID, comp.Name))
	sb.WriteString(fmt.Sprintf("   Rank: #%-3d%s | Points: %-6.2f%s | Weight: #%d/%d (%.8f)\n",
		comp.Ranking, changes.RankChange,
		comp.Points, changes.PointsChange,
		comp.WeightRank, comp.TotalWeightParticipants,
		comp.Weight))
}

func (f *Formatter) formatChange(diff float64, format string) string {
	if diff == 0 {
		return "   " // 변화 없을 때 공백으로 처리
	}

	var changeStr string
	if format == "" {
		changeStr = fmt.Sprintf("%d", int(diff))
	} else {
		changeStr = fmt.Sprintf(format, diff)
	}

	if diff > 0 {
		return fmt.Sprintf("⬆%s", changeStr)
	}
	return fmt.Sprintf("⬇%s", changeStr)
}

func (f *Formatter) determineRankTrend(changes map[string]models.RankChangeInfo, users []models.UserRankInfo) (bool, bool) {
	hasIncrease := false
	hasDecrease := false

	for _, change := range changes {
		if change.OverallRankDiff > 0 {
			hasIncrease = true
		} else if change.OverallRankDiff < 0 {
			hasDecrease = true
		}
	}

	return hasIncrease, hasDecrease
}

func (f *Formatter) formatTrendHeader(hasIncrease, hasDecrease bool) string {
	var header string
	if hasIncrease && !hasDecrease {
		header = "🎉 *Rank Increase Alert!*"
	} else if hasDecrease && !hasIncrease {
		header = "⚠️ *Rank Decrease Alert!*"
	}
	return header
}

func (f *Formatter) formatOverallRankings(sb *strings.Builder, users []models.UserRankInfo, changes map[string]models.RankChangeInfo) {
	sb.WriteString("📊 Overall Rankings:\n")
	for i, user := range users {
		if change, ok := changes[user.Address]; ok {
			rankChange := f.formatChange(float64(change.OverallRankDiff), "")
			pointsChange := f.formatChange(change.PointsDiff, "%.2f")

			sb.WriteString(fmt.Sprintf("%d. %s (@%s)\n", i+1, user.Name, user.Username))
			sb.WriteString(fmt.Sprintf("   Rank: #%-3d%s | Points: %-6.2f%s | 🏅 %s\n\n",
				user.Ranking, rankChange, user.Points, pointsChange, user.BadgeName))
		}
	}
}

func (f *Formatter) formatCompetitionRankings(sb *strings.Builder, users []models.UserRankInfo, changes map[string]models.RankChangeInfo) {
	compMap := f.buildCompetitionMap(users)

	for compID, name := range compMap {
		sb.WriteString(fmt.Sprintf("\n📌 %s (ID: %d):\n", name, compID))
		f.writeCompetitionUserRankings(sb, users, changes, compID)
	}
}

func (f *Formatter) buildCompetitionMap(users []models.UserRankInfo) map[int]string {
	compMap := make(map[int]string)
	for _, user := range users {
		for _, comp := range user.Competitions {
			compMap[comp.ID] = comp.Name
		}
	}
	return compMap
}

func (f *Formatter) writeCompetitionUserRankings(sb *strings.Builder, users []models.UserRankInfo, changes map[string]models.RankChangeInfo, compID int) {
	for _, user := range users {
		if change, ok := changes[user.Address]; ok {
			for _, comp := range user.Competitions {
				if comp.ID == compID {
					if compChange, ok := change.CompChanges[comp.ID]; ok {
						rankChange := f.formatChange(float64(compChange.RankDiff), "")
						pointsChange := f.formatChange(compChange.PointsDiff, "%.2f")

						// 사용자 정보
						sb.WriteString(fmt.Sprintf("*%s* (@%s)\n", user.Name, user.Username))

						// 상세 정보를 한 줄로 표시
						sb.WriteString(fmt.Sprintf("Rank: #%-3d%-8s | Points: %-6.2f%-8s | Weight: #%d/%d (%.8f)\n\n",
							comp.Ranking, rankChange,
							comp.Points, pointsChange,
							comp.WeightRank, comp.TotalWeightParticipants,
							comp.Weight))
					}
					break
				}
			}
		}
	}
}
