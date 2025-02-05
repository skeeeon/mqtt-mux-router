//file: internal/rule/types.go
package rule

type Rule struct {
	Topic      string       `json:"topic"`
	Conditions *Conditions  `json:"conditions"`
	Action     *Action      `json:"action"`
}

// Conditions represents a group of conditions with a logical operator
type Conditions struct {
	Operator string      `json:"operator"` // "and" or "or"
	Items    []Condition `json:"items"`
	Groups   []Conditions `json:"groups,omitempty"` // For nested condition groups
}

type Condition struct {
	Field    string      `json:"field"`
	Operator string      `json:"operator"`
	Value    interface{} `json:"value"`
}

type Action struct {
	Topic   string `json:"topic"`
	Payload string `json:"payload"`
}
