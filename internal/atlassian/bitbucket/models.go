// Package bitbucket provides a read-only SDK for the Bitbucket Cloud REST API v2.0.
// It mirrors the structure of the other atlassian service packages (interface,
// concrete impl, domain models) but targets api.bitbucket.org with its own
// base URL and credentials (base64 username:apiToken).
package bitbucket

// paginatedResponse is the generic wire envelope for Bitbucket list endpoints.
type paginatedResponse[T any] struct {
	Values  []T    `json:"values"`
	Next    string `json:"next,omitempty"`
	Size    int    `json:"size,omitempty"`
	Page    int    `json:"page,omitempty"`
	PageLen int    `json:"pagelen,omitempty"`
}

// Repository is a Bitbucket repository.
type Repository struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Language    string `json:"language,omitempty"`
	FullName    string `json:"full_name"`
	UpdatedOn   string `json:"updated_on"`
	IsPrivate   bool   `json:"is_private"`
}

// Account is a Bitbucket user account (author, reviewer, etc.).
type Account struct {
	DisplayName string `json:"display_name,omitempty"`
	Nickname    string `json:"nickname,omitempty"`
	UUID        string `json:"uuid,omitempty"`
}

// PullRequestParticipant is a reviewer/participant entry on a PR.
type PullRequestParticipant struct {
	User           *Account `json:"user,omitempty"`
	Role           string   `json:"role,omitempty"`
	Approved       bool     `json:"approved,omitempty"`
	State          string   `json:"state,omitempty"`
	ParticipatedOn string   `json:"participated_on,omitempty"`
}

// PullRequest is a Bitbucket pull request.
type PullRequest struct {
	ID           int                      `json:"id"`
	Title        string                   `json:"title"`
	Description  string                   `json:"description,omitempty"`
	State        string                   `json:"state"`
	Source       PullRequestEndpoint      `json:"source"`
	Destination  PullRequestEndpoint      `json:"destination"`
	Author       Account                  `json:"author"`
	CreatedOn    string                   `json:"created_on"`
	UpdatedOn    string                   `json:"updated_on"`
	ClosedOn     string                   `json:"closed_on,omitempty"`
	MergeCommit  *CommitRef               `json:"merge_commit,omitempty"`
	CommentCount int                      `json:"comment_count,omitempty"`
	TaskCount    int                      `json:"task_count,omitempty"`
	Draft        bool                     `json:"draft,omitempty"`
	Reviewers    []Account                `json:"reviewers,omitempty"`
	Participants []PullRequestParticipant `json:"participants,omitempty"`
	Links        PullRequestLinks         `json:"links"`
}

// PullRequestEndpoint is the source/destination branch of a PR.
type PullRequestEndpoint struct {
	Branch     PullRequestBranch     `json:"branch"`
	Repository PullRequestRepository `json:"repository,omitempty"`
}

// PullRequestBranch is a branch reference on a PR endpoint.
type PullRequestBranch struct {
	Name string `json:"name"`
}

// PullRequestRepository is the repo reference on a PR endpoint.
type PullRequestRepository struct {
	FullName string `json:"full_name"`
}

// PullRequestLinks holds the HTML link of a PR.
type PullRequestLinks struct {
	HTML Link `json:"html"`
}

// Link is a Bitbucket hyperlink reference.
type Link struct {
	Href string `json:"href"`
}

// CommitRef is a minimal commit reference (hash only).
type CommitRef struct {
	Hash string `json:"hash"`
}

// Branch is a Bitbucket branch (ref).
type Branch struct {
	Name   string       `json:"name"`
	Target BranchTarget `json:"target"`
}

// BranchTarget is the tip commit a branch points to.
type BranchTarget struct {
	Hash   string             `json:"hash"`
	Date   string             `json:"date"`
	Author BranchTargetAuthor `json:"author"`
}

// BranchTargetAuthor wraps the commit author user of a branch tip.
type BranchTargetAuthor struct {
	User BranchTargetUser `json:"user"`
}

// BranchTargetUser is the display name of a branch tip author.
type BranchTargetUser struct {
	DisplayName string `json:"display_name"`
}

// Pipeline is a Bitbucket Pipelines run.
type Pipeline struct {
	UUID              string         `json:"uuid"`
	BuildNumber       int            `json:"build_number"`
	State             PipelineState  `json:"state"`
	Target            PipelineTarget `json:"target"`
	CreatedOn         string         `json:"created_on"`
	DurationInSeconds *int           `json:"duration_in_seconds,omitempty"`
	Links             PipelineLinks  `json:"links"`
}

// PipelineState is the state of a pipeline (pending/in progress/completed).
type PipelineState struct {
	Name   string               `json:"name"`
	Result *PipelineStateResult `json:"result,omitempty"`
	Stage  *PipelineStateStage  `json:"stage,omitempty"`
}

// PipelineStateResult is the terminal result of a pipeline (successful/failed).
type PipelineStateResult struct {
	Name string `json:"name"`
}

// PipelineStateStage is the current stage of an in-progress pipeline.
type PipelineStateStage struct {
	Name string `json:"name"`
}

// PipelineTarget is the ref a pipeline ran against.
type PipelineTarget struct {
	RefName string `json:"ref_name,omitempty"`
	RefType string `json:"ref_type,omitempty"`
}

// PipelineLinks holds the self link of a pipeline.
type PipelineLinks struct {
	Self Link `json:"self"`
}

// Commit is a Bitbucket commit.
type Commit struct {
	Hash    string        `json:"hash"`
	Message string        `json:"message,omitempty"`
	Date    string        `json:"date,omitempty"`
	Author  *CommitAuthor `json:"author,omitempty"`
	Links   *CommitLinks  `json:"links,omitempty"`
}

// CommitAuthor is the author of a commit.
type CommitAuthor struct {
	Raw  string   `json:"raw,omitempty"`
	User *Account `json:"user,omitempty"`
}

// CommitLinks holds the HTML link of a commit.
type CommitLinks struct {
	HTML *Link `json:"html,omitempty"`
}

// PullRequestComment is a comment on a PR.
type PullRequestComment struct {
	ID        int                        `json:"id"`
	CreatedOn string                     `json:"created_on"`
	UpdatedOn string                     `json:"updated_on,omitempty"`
	User      *PullRequestCommentUser    `json:"user,omitempty"`
	Content   *PullRequestCommentContent `json:"content,omitempty"`
	Deleted   bool                       `json:"deleted,omitempty"`
}

// PullRequestCommentUser is the author of a PR comment.
type PullRequestCommentUser struct {
	DisplayName string `json:"display_name,omitempty"`
}

// PullRequestCommentContent is the body of a PR comment.
type PullRequestCommentContent struct {
	Raw    string `json:"raw,omitempty"`
	Markup string `json:"markup,omitempty"`
	HTML   string `json:"html,omitempty"`
}

// CommitStatus is a build/pipeline status check on a PR.
type CommitStatus struct {
	Key         string `json:"key"`
	Name        string `json:"name,omitempty"`
	State       string `json:"state,omitempty"`
	URL         string `json:"url,omitempty"`
	Description string `json:"description,omitempty"`
	UpdatedOn   string `json:"updated_on,omitempty"`
	CreatedOn   string `json:"created_on,omitempty"`
}

// CommitFile is a file reference inside a diffstat entry.
type CommitFile struct {
	Path        string `json:"path"`
	EscapedPath string `json:"escaped_path,omitempty"`
	Type        string `json:"type,omitempty"`
}

// PullRequestDiffstat is a single changed-file entry (diffstat) for a PR.
type PullRequestDiffstat struct {
	Type         string      `json:"type"`
	Status       string      `json:"status,omitempty"`
	LinesAdded   int         `json:"lines_added,omitempty"`
	LinesRemoved int         `json:"lines_removed,omitempty"`
	Old          *CommitFile `json:"old,omitempty"`
	New          *CommitFile `json:"new,omitempty"`
}

// PullRequestTask is a task on a pull request.
type PullRequestTask struct {
	ID         int                     `json:"id"`
	State      string                  `json:"state,omitempty"`
	Content    *PullRequestTaskContent `json:"content,omitempty"`
	Creator    *Account                `json:"creator,omitempty"`
	Assignee   *Account                `json:"assignee,omitempty"`
	CreatedOn  string                  `json:"created_on,omitempty"`
	UpdatedOn  string                  `json:"updated_on,omitempty"`
	ResolvedOn string                  `json:"resolved_on,omitempty"`
}

// PullRequestTaskContent is the body of a PR task.
type PullRequestTaskContent struct {
	Raw  string `json:"raw,omitempty"`
	HTML string `json:"html,omitempty"`
}

// --- write request types (public) ---

// CreatePRRequest is the public request to create a pull request.
// Reviewers accept nicknames or UUIDs wrapped in braces (e.g. "{uuid}").
type CreatePRRequest struct {
	Title             string
	SourceBranch      string
	DestinationBranch string
	Description       string   // optional
	Reviewers         []string // optional; nickname or {uuid}
	CloseSourceBranch bool     // optional
}

// UpdatePRRequest is the public partial-update request for a pull request.
// Empty string fields are omitted; a non-nil Reviewers slice replaces reviewers.
type UpdatePRRequest struct {
	Title             string   // optional
	Description       string   // optional
	DestinationBranch string   // optional
	Reviewers         []string // nil = leave unchanged; non-nil = replace
}
