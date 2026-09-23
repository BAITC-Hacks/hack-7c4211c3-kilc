package rating

import (
	"sort"
	"strconv"
	"strings"
	"unicode"
)

type fieldDefinition struct {
	key       string
	label     string
	max       int
	component int
	value     func(Card) Field
}

var componentDefinitions = [...]Component{
	{Key: "context_and_need", Label: "Контекст и потребность", Max: 20},
	{Key: "data_and_materials", Label: "Данные и материалы", Max: 20},
	{Key: "expected_result", Label: "Ожидаемый результат", Max: 15},
	{Key: "success_criteria", Label: "Критерии успеха", Max: 15},
	{Key: "constraints", Label: "Ограничения", Max: 10},
	{Key: "users", Label: "Пользователи", Max: 10},
	{Key: "business_contact", Label: "Связь с бизнесом", Max: 10},
}

var fieldDefinitions = [...]fieldDefinition{
	{key: FieldContext, label: "Контекст", max: 10, component: 0, value: func(c Card) Field { return c.Context }},
	{key: FieldNeed, label: "Потребность", max: 10, component: 0, value: func(c Card) Field { return c.Need }},
	{key: FieldUsers, label: "Пользователи", max: 10, component: 5, value: func(c Card) Field { return c.Users }},
	{key: FieldData, label: "Данные и материалы", max: 20, component: 1, value: func(c Card) Field { return c.Data }},
	{key: FieldConstraints, label: "Ограничения", max: 10, component: 4, value: func(c Card) Field { return c.Constraints }},
	{key: FieldExpectedResult, label: "Ожидаемый результат", max: 15, component: 2, value: func(c Card) Field { return c.ExpectedResult }},
	{key: FieldSuccessCriteria, label: "Критерии успеха", max: 15, component: 3, value: func(c Card) Field { return c.SuccessCriteria }},
	{key: FieldContact, label: "Контакт", max: 5, component: 6, value: func(c Card) Field { return c.Contact }},
	{key: FieldInteractionFormat, label: "Формат взаимодействия", max: 5, component: 6, value: func(c Card) Field { return c.InteractionFormat }},
}

// Score calculates confirmed and potential scores and returns their breakdown.
func Score(c Card) Result {
	components := make([]Component, len(componentDefinitions))
	copy(components, componentDefinitions[:])
	hints := make([]Hint, 0, len(fieldDefinitions))
	result := Result{Components: components, Hints: hints}

	for _, definition := range fieldDefinitions {
		field := definition.value(c)
		quality, reason := fieldQuality(definition, field.Text)
		result.Potential += quality
		confirmedScore := 0
		if field.Confirmed {
			confirmedScore = quality
			result.Total += quality
		}
		result.Components[definition.component].Score += confirmedScore

		if confirmedScore < definition.max {
			if reason == "" {
				reason = ReasonUnconfirmed
			}
			gain := definition.max - confirmedScore
			hints = append(hints, Hint{
				Field:   definition.key,
				Reason:  reason,
				Message: hintMessage(reason, definition.label, gain),
				Gain:    gain,
			})
		}
	}

	sort.SliceStable(hints, func(i, j int) bool { return hints[i].Gain > hints[j].Gain })
	result.Hints = hints
	result.Level = LevelFor(result.Total)
	result.LevelLabel = levelLabel(result.Level)
	trimmedReward := strings.TrimSpace(c.Reward.Text)
	rewardType, knownRewardType := rewardTypeFor(c.RewardType)
	if ValidRewardType(c.RewardType) && c.RewardType != "" && trimmedReward != "" && c.Reward.Confirmed {
		result.Bonus = RewardBonus(c.RewardType)
		result.BonusLabel = rewardType.Label
	}
	result.Position = result.Total + result.Bonus
	switch {
	case c.RewardType == "" || !knownRewardType || trimmedReward == "":
		result.BonusHint = "Укажите вознаграждение для команды — до +10 к позиции в каталоге"
	case !c.Reward.Confirmed:
		result.BonusHint = "Подтвердите вознаграждение (+" + strconv.Itoa(rewardType.Bonus) + " к позиции в каталоге)"
	case c.RewardType == "nonmonetary":
		result.BonusHint = "Денежное вознаграждение или оплачиваемая стажировка дали бы +10 к позиции"
	}
	return result
}

func fieldQuality(definition fieldDefinition, text string) (int, string) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return 0, ReasonEmpty
	}
	if definition.key == FieldContact {
		if strings.Contains(trimmed, "@") || countDigits(trimmed) >= 7 {
			return definition.max, ""
		}
		return definition.max / 2, ReasonNoContact
	}
	wordCount := len(strings.Fields(trimmed))
	if wordCount < 5 {
		return definition.max / 2, ReasonShort
	}
	if definition.key == FieldSuccessCriteria {
		for _, r := range trimmed {
			if unicode.IsDigit(r) {
				return definition.max, ""
			}
		}
		return definition.max / 2, ReasonNoNumber
	}
	return definition.max, ""
}

func countDigits(text string) int {
	count := 0
	for _, r := range text {
		if unicode.IsDigit(r) {
			count++
		}
	}
	return count
}

func hintMessage(reason, label string, gain int) string {
	switch reason {
	case ReasonEmpty:
		return "Заполните «" + label + "» (+" + strconv.Itoa(gain) + ")"
	case ReasonShort:
		return "Раскройте «" + label + "» подробнее, хотя бы 5 слов (+" + strconv.Itoa(gain) + ")"
	case ReasonNoNumber:
		return "Добавьте в «" + label + "» измеримый показатель: число, процент или срок (+" + strconv.Itoa(gain) + ")"
	case ReasonNoContact:
		return "Укажите в «" + label + "» email или телефон (+" + strconv.Itoa(gain) + ")"
	default:
		return "Подтвердите поле «" + label + "» (+" + strconv.Itoa(gain) + ")"
	}
}

// LevelFor returns the level code for a score, clamping scores to 0–100.
func LevelFor(score int) string {
	switch {
	case score < 40:
		return LevelDraft
	case score < 70:
		return LevelWorking
	case score < 90:
		return LevelReady
	default:
		return LevelPriority
	}
}

// LevelRange returns the inclusive score range for a known level code.
func LevelRange(level string) (min, max int, ok bool) {
	switch level {
	case LevelDraft:
		return 0, 39, true
	case LevelWorking:
		return 40, 69, true
	case LevelReady:
		return 70, 89, true
	case LevelPriority:
		return 90, 100, true
	default:
		return 0, 0, false
	}
}

func levelLabel(level string) string {
	switch level {
	case LevelDraft:
		return "Черновик"
	case LevelWorking:
		return "Рабочая"
	case LevelReady:
		return "Готовая"
	case LevelPriority:
		return "Приоритетная"
	default:
		return ""
	}
}
