// Package releases provides types and a service client for the Jira Versions REST API v3.
// It mirrors the agile package structure: interface, concrete impl, and domain models.
package releases

// Release is the domain model for a Jira project version (release).
type Release struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Archived    bool   `json:"archived"`
	Released    bool   `json:"released"`
	StartDate   string `json:"start_date,omitempty"`
	ReleaseDate string `json:"release_date,omitempty"`
	ProjectID   string `json:"project_id"`
}

// ReleaseIssueCounts holds issue counts for a release by fix-version and affects-version.
type ReleaseIssueCounts struct {
	FixVersion     int `json:"fix_version"`
	AffectsVersion int `json:"affects_version"`
}

// CreateReleaseRequest is the public request to create a new Jira release.
// ProjectID is passed as an integer in the wire JSON; pass the string representation.
// StartDate and ReleaseDate are optional; pass "" to omit.
type CreateReleaseRequest struct {
	ProjectID   string // sent as int in wire JSON via strconv.Atoi
	Name        string
	Description string // optional
	StartDate   string // YYYY-MM-DD, optional
	ReleaseDate string // YYYY-MM-DD, optional
}

// UpdateReleaseRequest is the public partial-update request; nil fields are omitted.
type UpdateReleaseRequest struct {
	Name        *string
	Description *string
	Released    *bool
	Archived    *bool
	ReleaseDate *string
}

// --- unexported wire types for the Jira REST API v3 ---

// releasesAPIItem is the JSON shape for a single version from the Jira REST API.
// ProjectID is returned as an integer; we convert to string in the domain mapping.
type releasesAPIItem struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Archived    bool   `json:"archived"`
	Released    bool   `json:"released"`
	StartDate   string `json:"startDate"`
	ReleaseDate string `json:"releaseDate"`
	ProjectID   int    `json:"projectId"` // Jira returns integer; converted to string in domain
}

// releaseIssueCountsAPIResponse is the JSON shape from GET .../relatedIssueCounts.
type releaseIssueCountsAPIResponse struct {
	FixVersion     int `json:"fixVersion"`
	AffectsVersion int `json:"affectsVersion"`
}

// createReleaseAPIRequest is the JSON body for POST /rest/api/3/version.
type createReleaseAPIRequest struct {
	ProjectID   int    `json:"projectId"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	StartDate   string `json:"startDate,omitempty"`
	ReleaseDate string `json:"releaseDate,omitempty"`
}

// updateReleaseAPIRequest is the JSON body for PUT /rest/api/3/version/{id}.
// omitempty on strings; Released and Archived are explicit pointer bools.
type updateReleaseAPIRequest struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Released    *bool  `json:"released,omitempty"`
	Archived    *bool  `json:"archived,omitempty"`
	ReleaseDate string `json:"releaseDate,omitempty"`
}
