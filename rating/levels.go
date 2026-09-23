package rating

// Level describes one rating level for filters and labels in the UI.
type Level struct {
	Code  string `json:"code"`
	Label string `json:"label"`
	Min   int    `json:"min"`
	Max   int    `json:"max"`
}

var levelOrder = [...]string{LevelDraft, LevelWorking, LevelReady, LevelPriority}

// Levels returns all rating levels from lowest to highest score.
func Levels() []Level {
	levels := make([]Level, 0, len(levelOrder))
	for _, code := range levelOrder {
		min, max, _ := LevelRange(code)
		levels = append(levels, Level{Code: code, Label: levelLabel(code), Min: min, Max: max})
	}
	return levels
}

// LevelLabel returns the display label for a level code, or "" if unknown.
func LevelLabel(code string) string {
	return levelLabel(code)
}
