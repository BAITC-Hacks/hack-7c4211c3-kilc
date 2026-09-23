package rating

import (
	"strings"
	"testing"
)

func TestRewardBonus(t *testing.T) {
	tests := []struct {
		name       string
		rewardType string
		reward     Field
		wantBonus  int
		wantHint   string
	}{
		{name: "no reward", wantHint: "Укажите вознаграждение"},
		{name: "money", rewardType: "money", reward: Field{"Грант для команды", true}, wantBonus: 10},
		{name: "internship", rewardType: "internship", reward: Field{"Оплачиваемая стажировка", true}, wantBonus: 10},
		{name: "nonmonetary", rewardType: "nonmonetary", reward: Field{"Сертификат для команды", true}, wantBonus: 5, wantHint: "дали бы +10"},
		{name: "unconfirmed", rewardType: "money", reward: Field{"Грант для команды", false}, wantHint: "Подтвердите"},
		{name: "empty text", rewardType: "money", reward: Field{"   ", true}, wantHint: "Укажите вознаграждение"},
		{name: "unknown", rewardType: "vip", reward: Field{"Грант для команды", true}, wantHint: "Укажите вознаграждение"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			card := Card{RewardType: tc.rewardType, Reward: tc.reward}
			result := Score(card)
			if result.Bonus != tc.wantBonus {
				t.Errorf("Bonus = %d, want %d", result.Bonus, tc.wantBonus)
			}
			if result.Position != result.Total+tc.wantBonus {
				t.Errorf("Position = %d, want Total + bonus = %d", result.Position, result.Total+tc.wantBonus)
			}
			if tc.wantHint != "" && !strings.Contains(result.BonusHint, tc.wantHint) {
				t.Errorf("BonusHint = %q, want it to contain %q", result.BonusHint, tc.wantHint)
			}
			if tc.wantBonus > 0 {
				rewardType, _ := rewardTypeFor(tc.rewardType)
				if result.BonusLabel != rewardType.Label {
					t.Errorf("BonusLabel = %q, want %q", result.BonusLabel, rewardType.Label)
				}
			}
		})
	}
}

func TestValidRewardType(t *testing.T) {
	for _, tc := range []struct {
		code string
		want bool
	}{{"", true}, {"money", true}, {"vip", false}} {
		if got := ValidRewardType(tc.code); got != tc.want {
			t.Errorf("ValidRewardType(%q) = %t, want %t", tc.code, got, tc.want)
		}
	}
	if got := RewardBonus("vip"); got != 0 {
		t.Errorf("RewardBonus(vip) = %d, want 0", got)
	}
}

func TestRewardDoesNotChangeReadiness(t *testing.T) {
	base := fullCard(true)
	withoutReward := Score(base)
	base.RewardType = "money"
	base.Reward = Field{"Денежный грант", true}
	withReward := Score(base)
	if withoutReward.Total != 100 || withReward.Total != 100 || withoutReward.Level != LevelPriority || withReward.Level != LevelPriority {
		t.Fatalf("readiness changed: without reward total=%d level=%q; with reward total=%d level=%q", withoutReward.Total, withoutReward.Level, withReward.Total, withReward.Level)
	}
	if withReward.Position != 110 {
		t.Errorf("reward position = %d, want 110", withReward.Position)
	}

	card := Card{
		Context: base.Context, Need: base.Need, Data: base.Data,
		ExpectedResult: base.ExpectedResult, SuccessCriteria: base.SuccessCriteria,
		Contact:    Field{"Позвоните менеджеру", true},
		RewardType: "money", Reward: Field{"Грант для команды", true},
	}
	result := Score(card)
	if result.Total != 72 || result.Level != LevelReady || result.Position != 82 {
		t.Errorf("got total=%d level=%q position=%d, want 72 ready 82", result.Total, result.Level, result.Position)
	}
}
