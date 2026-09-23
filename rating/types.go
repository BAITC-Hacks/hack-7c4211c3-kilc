package rating

// Field is a text value supplied for one part of a task card.
type Field struct {
	Text      string `json:"text"`
	Confirmed bool   `json:"confirmed"`
}

// Card contains the fields considered by the rating scorer.
type Card struct {
	Context           Field `json:"context"`
	Need              Field `json:"need"`
	Users             Field `json:"users"`
	Data              Field `json:"data"`
	Constraints       Field `json:"constraints"`
	ExpectedResult    Field `json:"expected_result"`
	SuccessCriteria   Field `json:"success_criteria"`
	Contact           Field `json:"contact"`
	InteractionFormat Field `json:"interaction_format"`
}

const (
	FieldContext           = "context"
	FieldNeed              = "need"
	FieldUsers             = "users"
	FieldData              = "data"
	FieldConstraints       = "constraints"
	FieldExpectedResult    = "expected_result"
	FieldSuccessCriteria   = "success_criteria"
	FieldContact           = "contact"
	FieldInteractionFormat = "interaction_format"
)

const (
	LevelDraft    = "draft"
	LevelWorking  = "working"
	LevelReady    = "ready"
	LevelPriority = "priority"
)

const (
	ReasonEmpty       = "empty"
	ReasonShort       = "short"
	ReasonNoNumber    = "no_number"
	ReasonNoContact   = "no_contact"
	ReasonUnconfirmed = "unconfirmed"
)

// Component describes one weighted part of the rating.
type Component struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Max   int    `json:"max"`
	Score int    `json:"score"`
}

// Hint explains how a field can improve its confirmed score.
type Hint struct {
	Field   string `json:"field"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
	Gain    int    `json:"gain"`
}

// Result is the complete rating breakdown for a card.
type Result struct {
	Total      int         `json:"total"`
	Potential  int         `json:"potential"`
	Level      string      `json:"level"`
	LevelLabel string      `json:"level_label"`
	Components []Component `json:"components"`
	Hints      []Hint      `json:"hints"`
}
