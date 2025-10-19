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
type SurveyDetails struct {
	Survey      Survey                 `json:"survey"`
	Invitations []EnrollmentInvitation `json:"invitations"`
	Statistics  SurveyStatistics       `json:"statistics"`
}

type SurveyStatisticsCounts struct {
	TotalEnrollments         int
	ResponsesStarted         int
	ResponsesSubmitted       int
	ResponsesInProgress      int
	AverageCompletionSeconds *float64
}

type SurveyStatistics struct {
	TotalEnrollments          int      `json:"total_enrollments"`
	ResponsesStarted          int      `json:"responses_started"`
	ResponsesSubmitted        int      `json:"responses_submitted"`
	ResponsesInProgress       int      `json:"responses_in_progress"`
	CompletionRate            float64  `json:"completion_rate"`
	OverallProgress           float64  `json:"overall_progress"`
	AverageCompletionSeconds  *float64 `json:"average_completion_seconds,omitempty"`
	AverageCompletionDuration *string  `json:"average_completion_duration,omitempty"`
}

type SurveyResultsSummary struct {
	Survey     Survey           `json:"survey"`
	Results    []SurveyResult   `json:"results"`
	Statistics SurveyStatistics `json:"statistics"`
}

type SurveySummary struct {
	Survey     Survey           `json:"survey"`
	Statistics SurveyStatistics `json:"statistics"`
}

func (c SurveyStatisticsCounts) ToSurveyStatistics() SurveyStatistics {
	stats := SurveyStatistics{
		TotalEnrollments:         c.TotalEnrollments,
		ResponsesStarted:         c.ResponsesStarted,
		ResponsesSubmitted:       c.ResponsesSubmitted,
		ResponsesInProgress:      c.ResponsesInProgress,
		AverageCompletionSeconds: c.AverageCompletionSeconds,
	}

	if stats.TotalEnrollments > 0 {
		stats.CompletionRate = float64(stats.ResponsesSubmitted) / float64(stats.TotalEnrollments)
		stats.OverallProgress = float64(stats.ResponsesStarted) / float64(stats.TotalEnrollments)
	}

	if c.AverageCompletionSeconds != nil {
		duration := time.Duration(*c.AverageCompletionSeconds * float64(time.Second))
		formatted := duration.String()
		stats.AverageCompletionDuration = &formatted
	}

	return stats
}
