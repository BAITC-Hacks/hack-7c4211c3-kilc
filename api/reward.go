package api

import (
	"net/http"

	"github.com/BAITC-Hacks/hack-7c4211c3-kilc/rating"
)

type rewardTypeView struct {
	Code  string `json:"code"`
	Label string `json:"label"`
	Bonus int    `json:"bonus"`
}

// RegisterRewardTypes exposes the available reward types for task forms.
func RegisterRewardTypes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/reward-types", func(w http.ResponseWriter, r *http.Request) {
		items := make([]rewardTypeView, 0, len(rating.RewardTypes)+1)
		items = append(items, rewardTypeView{Code: "", Label: "Без вознаграждения", Bonus: 0})
		for _, rewardType := range rating.RewardTypes {
			items = append(items, rewardTypeView{Code: rewardType.Code, Label: rewardType.Label, Bonus: rewardType.Bonus})
		}
		writeJSON(w, http.StatusOK, items)
	})
}
