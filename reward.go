package rating

// RewardType describes a reward offered to a student team.
type RewardType struct {
	Code  string `json:"code"`
	Label string `json:"label"`
	Bonus int    `json:"bonus"`
}

var RewardTypes = []RewardType{
	{Code: "nonmonetary", Label: "Сертификат, рекомендация или мерч", Bonus: 5},
	{Code: "internship", Label: "Оплачиваемая стажировка или оффер", Bonus: 10},
	{Code: "money", Label: "Денежное вознаграждение или грант", Bonus: 10},
}

// ValidRewardType reports whether code is empty or names a supported reward.
func ValidRewardType(code string) bool {
	if code == "" {
		return true
	}
	for _, rewardType := range RewardTypes {
		if rewardType.Code == code {
			return true
		}
	}
	return false
}

// RewardBonus returns the catalog bonus for a reward type.
func RewardBonus(code string) int {
	for _, rewardType := range RewardTypes {
		if rewardType.Code == code {
			return rewardType.Bonus
		}
	}
	return 0
}

func rewardTypeFor(code string) (RewardType, bool) {
	for _, rewardType := range RewardTypes {
		if rewardType.Code == code {
			return rewardType, true
		}
	}
	return RewardType{}, false
}
