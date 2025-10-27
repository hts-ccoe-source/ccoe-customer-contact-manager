package typeform

// SurveyType represents the type of survey to create
type SurveyType string

const (
	SurveyTypeChange      SurveyType = "change"
	SurveyTypeCIC         SurveyType = "cic"
	SurveyTypeInnerSource SurveyType = "innersource"
	SurveyTypeFinOps      SurveyType = "finops"
	SurveyTypeGeneral     SurveyType = "general"
)

// Field represents a Typeform field
type Field struct {
	Type       string                 `json:"type"`
	Title      string                 `json:"title"`
	Properties map[string]interface{} `json:"properties,omitempty"`
}

// SurveyTemplate defines the structure for each survey type
type SurveyTemplate struct {
	Fields []Field
}

// GetSurveyTemplate returns the template for a given survey type
func GetSurveyTemplate(surveyType SurveyType) SurveyTemplate {
	switch surveyType {
	case SurveyTypeChange:
		return getChangeTemplate()
	case SurveyTypeCIC:
		return getCICTemplate()
	case SurveyTypeInnerSource:
		return getInnerSourceTemplate()
	case SurveyTypeFinOps:
		return getFinOpsTemplate()
	case SurveyTypeGeneral:
		return getGeneralTemplate()
	default:
		return getGeneralTemplate()
	}
}

// getChangeTemplate returns the survey template for changes
func getChangeTemplate() SurveyTemplate {
	return SurveyTemplate{
		Fields: []Field{
			{
				Type:  "yes_no",
				Title: "Was this change excellent?",
			},
			{
				Type:  "opinion_scale",
				Title: "How likely are you to recommend Hearst CCOE to a colleague?",
				Properties: map[string]interface{}{
					"start_at_one": false,
					"steps":        11,
					"labels": map[string]string{
						"left":  "Not at all likely",
						"right": "Extremely likely",
					},
				},
			},
			{
				Type:  "long_text",
				Title: "What could we improve about this change?",
				Properties: map[string]interface{}{
					"description": "Any suggestions/comments/criticisms are welcome",
				},
			},
		},
	}
}

// getCICTemplate returns the survey template for CIC announcements
func getCICTemplate() SurveyTemplate {
	return SurveyTemplate{
		Fields: []Field{
			{
				Type:  "yes_no",
				Title: "Was this CIC announcement excellent?",
			},
			{
				Type:  "opinion_scale",
				Title: "How likely are you to recommend Hearst CCOE to a colleague?",
				Properties: map[string]interface{}{
					"start_at_one": false,
					"steps":        11,
					"labels": map[string]string{
						"left":  "Not at all likely",
						"right": "Extremely likely",
					},
				},
			},
			{
				Type:  "long_text",
				Title: "What could we improve about CIC announcements?",
				Properties: map[string]interface{}{
					"description": "Any suggestions/comments/criticisms are welcome",
				},
			},
		},
	}
}

// getInnerSourceTemplate returns the survey template for InnerSource announcements
func getInnerSourceTemplate() SurveyTemplate {
	return SurveyTemplate{
		Fields: []Field{
			{
				Type:  "yes_no",
				Title: "Was this InnerSource announcement excellent?",
			},
			{
				Type:  "opinion_scale",
				Title: "How likely are you to recommend Hearst CCOE to a colleague?",
				Properties: map[string]interface{}{
					"start_at_one": false,
					"steps":        11,
					"labels": map[string]string{
						"left":  "Not at all likely",
						"right": "Extremely likely",
					},
				},
			},
			{
				Type:  "long_text",
				Title: "What could we improve about InnerSource announcements?",
				Properties: map[string]interface{}{
					"description": "Any suggestions/comments/criticisms are welcome",
				},
			},
		},
	}
}

// getFinOpsTemplate returns the survey template for FinOps announcements
func getFinOpsTemplate() SurveyTemplate {
	return SurveyTemplate{
		Fields: []Field{
			{
				Type:  "yes_no",
				Title: "Was this FinOps announcement excellent?",
			},
			{
				Type:  "opinion_scale",
				Title: "How likely are you to recommend Hearst CCOE to a colleague?",
				Properties: map[string]interface{}{
					"start_at_one": false,
					"steps":        11,
					"labels": map[string]string{
						"left":  "Not at all likely",
						"right": "Extremely likely",
					},
				},
			},
			{
				Type:  "long_text",
				Title: "What could we improve about FinOps announcements?",
				Properties: map[string]interface{}{
					"description": "Any suggestions/comments/criticisms are welcome",
				},
			},
		},
	}
}

// getGeneralTemplate returns the survey template for general announcements
func getGeneralTemplate() SurveyTemplate {
	return SurveyTemplate{
		Fields: []Field{
			{
				Type:  "yes_no",
				Title: "Was this announcement excellent?",
			},
			{
				Type:  "opinion_scale",
				Title: "How likely are you to recommend Hearst CCOE to a colleague?",
				Properties: map[string]interface{}{
					"start_at_one": false,
					"steps":        11,
					"labels": map[string]string{
						"left":  "Not at all likely",
						"right": "Extremely likely",
					},
				},
			},
			{
				Type:  "long_text",
				Title: "What could we improve about this announcement?",
				Properties: map[string]interface{}{
					"description": "Any suggestions/comments/criticisms are welcome",
				},
			},
		},
	}
}
