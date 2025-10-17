package domains

import (
	"encoding/json"
	"time"
)

type SurveySubmission struct {
	Token   string                   `json:"token"`
	Channel string                   `json:"channel,omitempty"`
	Answers []SurveySubmissionAnswer `json:"answers"`
}

type SurveySubmissionAnswer struct {
	QuestionCode  string          `json:"question_code"`
	SectionCode   *string         `json:"section_code,omitempty"`
	RepeatPath    string          `json:"repeat_path,omitempty"`
	ValueText     *string         `json:"value_text,omitempty"`
	ValueNumber   *float64        `json:"value_number,omitempty"`
	ValueBool     *bool           `json:"value_bool,omitempty"`
	ValueDate     *time.Time      `json:"value_date,omitempty"`
	ValueDateTime *time.Time      `json:"value_datetime,omitempty"`
	ValueJSON     json.RawMessage `json:"value_json,omitempty"`
}

type SurveyAnswerToSave struct {
	QuestionCode  string
	SectionCode   *string
	RepeatPath    string
	ValueText     *string
	ValueNumber   *float64
	ValueBool     *bool
	ValueDate     *time.Time
	ValueDateTime *time.Time
	ValueJSON     json.RawMessage
}

type SurveyResponseToSave struct {
	SurveyID       int64
	EnrollmentID   int64
	Channel        string
	State          string
	SubmittedAt    time.Time
	Answers        []SurveyAnswerToSave
	IncrementUsage bool
}

type SurveyResponse struct {
	ID           int64      `json:"id"`
	SurveyID     int64      `json:"survey_id"`
	EnrollmentID int64      `json:"enrollment_id"`
	State        string     `json:"state"`
	Channel      *string    `json:"channel,omitempty"`
	StartedAt    time.Time  `json:"started_at"`
	SubmittedAt  *time.Time `json:"submitted_at,omitempty"`
}

type SurveyAnswer struct {
	QuestionCode  string          `json:"question_code"`
	SectionCode   *string         `json:"section_code,omitempty"`
	RepeatPath    string          `json:"repeat_path"`
	ValueText     *string         `json:"value_text,omitempty"`
	ValueNumber   *float64        `json:"value_number,omitempty"`
	ValueBool     *bool           `json:"value_bool,omitempty"`
	ValueDate     *time.Time      `json:"value_date,omitempty"`
	ValueDateTime *time.Time      `json:"value_datetime,omitempty"`
	ValueJSON     json.RawMessage `json:"value_json,omitempty"`
}

type SurveyResponseResult struct {
	Response SurveyResponse `json:"response"`
	Answers  []SurveyAnswer `json:"answers"`
}

type SurveyResult struct {
	Survey     Survey         `json:"survey"`
	Enrollment Enrollment     `json:"enrollment"`
	Response   SurveyResponse `json:"response"`
	Answers    []SurveyAnswer `json:"answers"`
}
