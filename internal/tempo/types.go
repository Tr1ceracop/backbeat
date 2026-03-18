package tempo

type CreateWorklogRequest struct {
	IssueID          int    `json:"issueId"`
	AuthorAccountID  string `json:"authorAccountId"`
	TimeSpentSeconds int    `json:"timeSpentSeconds"`
	StartDate        string `json:"startDate"`
	StartTime        string `json:"startTime"`
	Description      string `json:"description,omitempty"`
}

type WorklogResponse struct {
	TempoWorklogID   int    `json:"tempoWorklogId"`
	IssueID          int    `json:"issueId"`
	TimeSpentSeconds int    `json:"timeSpentSeconds"`
	StartDate        string `json:"startDate"`
	StartTime        string `json:"startTime"`
	Description      string `json:"description"`
}

type WorklogSearchResult struct {
	Results  []WorklogResponse `json:"results"`
	Metadata struct {
		Count  int `json:"count"`
		Offset int `json:"offset"`
		Limit  int `json:"limit"`
	} `json:"metadata"`
}
