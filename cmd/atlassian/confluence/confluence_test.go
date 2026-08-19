package confluence_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	confluencecli "github.com/jinkp/atlassian-go-mcp/cmd/atlassian/confluence"
	confluencesvc "github.com/jinkp/atlassian-go-mcp/internal/atlassian/confluence"
	"github.com/jinkp/atlassian-go-mcp/internal/audit"
)

// --- Mock service ---

type mockConfluenceService struct {
	getPageFunc             func(ctx context.Context, pageID, bodyFormat string) (*confluencesvc.Page, error)
	getPagesInSpaceFunc     func(ctx context.Context, spaceID string, limit int, cursor string) (*confluencesvc.PageList, error)
	getSpacesFunc           func(ctx context.Context, limit int, cursor string, keys []string, spaceType string) (*confluencesvc.SpaceList, error)
	getPageDescendantsFunc  func(ctx context.Context, pageID string, limit int, cursor string) (*confluencesvc.PageRefList, error)
	getFooterCommentsFunc   func(ctx context.Context, pageID string, limit int, cursor string) (*confluencesvc.CommentList, error)
	getInlineCommentsFunc   func(ctx context.Context, pageID string, limit int, cursor string) (*confluencesvc.CommentList, error)
	getCommentChildrenFunc  func(ctx context.Context, commentID string, limit int, cursor string) (*confluencesvc.CommentList, error)
	createPageFunc          func(ctx context.Context, req confluencesvc.CreatePageRequest) (*confluencesvc.Page, error)
	updatePageFunc          func(ctx context.Context, req confluencesvc.UpdatePageRequest) (*confluencesvc.Page, error)
	createFooterCommentFunc func(ctx context.Context, req confluencesvc.CreateCommentRequest) (*confluencesvc.Comment, error)
	createInlineCommentFunc func(ctx context.Context, req confluencesvc.CreateInlineCommentRequest) (*confluencesvc.Comment, error)
	searchContentFunc       func(ctx context.Context, cql string, limit int) ([]confluencesvc.SearchResult, error)
}

func (m *mockConfluenceService) GetPage(ctx context.Context, pageID, bodyFormat string) (*confluencesvc.Page, error) {
	if m.getPageFunc != nil {
		return m.getPageFunc(ctx, pageID, bodyFormat)
	}
	return nil, errors.New("not implemented")
}

func (m *mockConfluenceService) GetPagesInSpace(ctx context.Context, spaceID string, limit int, cursor string) (*confluencesvc.PageList, error) {
	if m.getPagesInSpaceFunc != nil {
		return m.getPagesInSpaceFunc(ctx, spaceID, limit, cursor)
	}
	return nil, errors.New("not implemented")
}

func (m *mockConfluenceService) GetSpaces(ctx context.Context, limit int, cursor string, keys []string, spaceType string) (*confluencesvc.SpaceList, error) {
	if m.getSpacesFunc != nil {
		return m.getSpacesFunc(ctx, limit, cursor, keys, spaceType)
	}
	return nil, errors.New("not implemented")
}

func (m *mockConfluenceService) GetPageDescendants(ctx context.Context, pageID string, limit int, cursor string) (*confluencesvc.PageRefList, error) {
	if m.getPageDescendantsFunc != nil {
		return m.getPageDescendantsFunc(ctx, pageID, limit, cursor)
	}
	return nil, errors.New("not implemented")
}

func (m *mockConfluenceService) GetFooterComments(ctx context.Context, pageID string, limit int, cursor string) (*confluencesvc.CommentList, error) {
	if m.getFooterCommentsFunc != nil {
		return m.getFooterCommentsFunc(ctx, pageID, limit, cursor)
	}
	return nil, errors.New("not implemented")
}

func (m *mockConfluenceService) GetInlineComments(ctx context.Context, pageID string, limit int, cursor string) (*confluencesvc.CommentList, error) {
	if m.getInlineCommentsFunc != nil {
		return m.getInlineCommentsFunc(ctx, pageID, limit, cursor)
	}
	return nil, errors.New("not implemented")
}

func (m *mockConfluenceService) GetCommentChildren(ctx context.Context, commentID string, limit int, cursor string) (*confluencesvc.CommentList, error) {
	if m.getCommentChildrenFunc != nil {
		return m.getCommentChildrenFunc(ctx, commentID, limit, cursor)
	}
	return nil, errors.New("not implemented")
}

func (m *mockConfluenceService) CreatePage(ctx context.Context, req confluencesvc.CreatePageRequest) (*confluencesvc.Page, error) {
	if m.createPageFunc != nil {
		return m.createPageFunc(ctx, req)
	}
	return nil, errors.New("not implemented")
}

func (m *mockConfluenceService) UpdatePage(ctx context.Context, req confluencesvc.UpdatePageRequest) (*confluencesvc.Page, error) {
	if m.updatePageFunc != nil {
		return m.updatePageFunc(ctx, req)
	}
	return nil, errors.New("not implemented")
}

func (m *mockConfluenceService) CreateFooterComment(ctx context.Context, req confluencesvc.CreateCommentRequest) (*confluencesvc.Comment, error) {
	if m.createFooterCommentFunc != nil {
		return m.createFooterCommentFunc(ctx, req)
	}
	return nil, errors.New("not implemented")
}

func (m *mockConfluenceService) CreateInlineComment(ctx context.Context, req confluencesvc.CreateInlineCommentRequest) (*confluencesvc.Comment, error) {
	if m.createInlineCommentFunc != nil {
		return m.createInlineCommentFunc(ctx, req)
	}
	return nil, errors.New("not implemented")
}

func (m *mockConfluenceService) SearchContent(ctx context.Context, cql string, limit int) ([]confluencesvc.SearchResult, error) {
	if m.searchContentFunc != nil {
		return m.searchContentFunc(ctx, cql, limit)
	}
	return nil, errors.New("not implemented")
}

// --- page create: dry-run ---

func TestPageCreate_DryRun(t *testing.T) {
	svc := &mockConfluenceService{
		createPageFunc: func(_ context.Context, _ confluencesvc.CreatePageRequest) (*confluencesvc.Page, error) {
			t.Error("service should NOT be called in dry-run mode")
			return nil, nil
		},
	}

	cmd := confluencecli.NewPageCmd(svc, audit.NewNoopLogger(), true)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"create", "--space-id", "S1", "--title", "My Page", "--body", "<p>hello</p>"})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "[DRY RUN]") {
		t.Errorf("expected [DRY RUN] in output, got: %q", out)
	}
}

// --- page create: calls service ---

func TestPageCreate_CallsService(t *testing.T) {
	var called bool
	svc := &mockConfluenceService{
		createPageFunc: func(_ context.Context, req confluencesvc.CreatePageRequest) (*confluencesvc.Page, error) {
			called = true
			if req.SpaceID != "S1" {
				t.Errorf("expected space-id 'S1', got %q", req.SpaceID)
			}
			if req.Title != "My Page" {
				t.Errorf("expected title 'My Page', got %q", req.Title)
			}
			return &confluencesvc.Page{ID: "12345", Title: "My Page"}, nil
		},
	}

	cmd := confluencecli.NewPageCmd(svc, audit.NewNoopLogger(), false)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"create", "--space-id", "S1", "--title", "My Page", "--body", "<p>hello</p>"})
	_ = cmd.Execute()

	if !called {
		t.Error("expected service.CreatePage to be called")
	}
	if !strings.Contains(buf.String(), "12345") {
		t.Errorf("expected page ID '12345' in output, got: %q", buf.String())
	}
}

// --- page update: dry-run ---

func TestPageUpdate_DryRun(t *testing.T) {
	svc := &mockConfluenceService{
		updatePageFunc: func(_ context.Context, _ confluencesvc.UpdatePageRequest) (*confluencesvc.Page, error) {
			t.Error("service should NOT be called in dry-run mode")
			return nil, nil
		},
	}

	cmd := confluencecli.NewPageCmd(svc, audit.NewNoopLogger(), true)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"update", "PAGE123", "--title", "New Title", "--body", "<p>new</p>"})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "[DRY RUN]") {
		t.Errorf("expected [DRY RUN] in output, got: %q", out)
	}
}

// --- page update: calls service ---

func TestPageUpdate_CallsService(t *testing.T) {
	var called bool
	svc := &mockConfluenceService{
		updatePageFunc: func(_ context.Context, req confluencesvc.UpdatePageRequest) (*confluencesvc.Page, error) {
			called = true
			if req.PageID != "PAGE123" {
				t.Errorf("expected page-id 'PAGE123', got %q", req.PageID)
			}
			return &confluencesvc.Page{ID: "PAGE123", Title: "New Title"}, nil
		},
	}

	cmd := confluencecli.NewPageCmd(svc, audit.NewNoopLogger(), false)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"update", "PAGE123", "--title", "New Title", "--body", "<p>new</p>"})
	_ = cmd.Execute()

	if !called {
		t.Error("expected service.UpdatePage to be called")
	}
}

// --- comment add-footer: dry-run ---

func TestCommentAddFooter_DryRun(t *testing.T) {
	svc := &mockConfluenceService{
		createFooterCommentFunc: func(_ context.Context, _ confluencesvc.CreateCommentRequest) (*confluencesvc.Comment, error) {
			t.Error("service should NOT be called in dry-run mode")
			return nil, nil
		},
	}

	cmd := confluencecli.NewCommentCmd(svc, audit.NewNoopLogger(), true)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"add-footer", "PAGE1", "--body", "<p>comment</p>"})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "[DRY RUN]") {
		t.Errorf("expected [DRY RUN] in output, got: %q", out)
	}
}

// --- comment add-footer: calls service ---

func TestCommentAddFooter_CallsService(t *testing.T) {
	var called bool
	svc := &mockConfluenceService{
		createFooterCommentFunc: func(_ context.Context, req confluencesvc.CreateCommentRequest) (*confluencesvc.Comment, error) {
			called = true
			if req.PageID != "PAGE1" {
				t.Errorf("expected page-id 'PAGE1', got %q", req.PageID)
			}
			return &confluencesvc.Comment{ID: "CMT1"}, nil
		},
	}

	cmd := confluencecli.NewCommentCmd(svc, audit.NewNoopLogger(), false)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"add-footer", "PAGE1", "--body", "<p>comment</p>"})
	_ = cmd.Execute()

	if !called {
		t.Error("expected service.CreateFooterComment to be called")
	}
	if !strings.Contains(buf.String(), "CMT1") {
		t.Errorf("expected comment ID 'CMT1' in output, got: %q", buf.String())
	}
}

// --- comment add-inline: dry-run ---

func TestCommentAddInline_DryRun(t *testing.T) {
	svc := &mockConfluenceService{
		createInlineCommentFunc: func(_ context.Context, _ confluencesvc.CreateInlineCommentRequest) (*confluencesvc.Comment, error) {
			t.Error("service should NOT be called in dry-run mode")
			return nil, nil
		},
	}

	cmd := confluencecli.NewCommentCmd(svc, audit.NewNoopLogger(), true)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"add-inline", "PAGE1", "--body", "<p>inline</p>", "--text-selection", "some text"})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "[DRY RUN]") {
		t.Errorf("expected [DRY RUN] in output, got: %q", out)
	}
}

// --- comment add-inline: calls service ---

func TestCommentAddInline_CallsService(t *testing.T) {
	var called bool
	svc := &mockConfluenceService{
		createInlineCommentFunc: func(_ context.Context, req confluencesvc.CreateInlineCommentRequest) (*confluencesvc.Comment, error) {
			called = true
			if req.TextSelection != "selected text" {
				t.Errorf("expected text-selection 'selected text', got %q", req.TextSelection)
			}
			return &confluencesvc.Comment{ID: "INLINE1"}, nil
		},
	}

	cmd := confluencecli.NewCommentCmd(svc, audit.NewNoopLogger(), false)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"add-inline", "PAGE1", "--body", "<p>inline</p>", "--text-selection", "selected text"})
	_ = cmd.Execute()

	if !called {
		t.Error("expected service.CreateInlineComment to be called")
	}
}

// --- comment add-inline: missing --text-selection is rejected ---

func TestCommentAddInline_MissingTextSelection_Rejected(t *testing.T) {
	svc := &mockConfluenceService{
		createInlineCommentFunc: func(_ context.Context, _ confluencesvc.CreateInlineCommentRequest) (*confluencesvc.Comment, error) {
			t.Error("service should NOT be called when text-selection is missing")
			return nil, nil
		},
	}

	cmd := confluencecli.NewCommentCmd(svc, audit.NewNoopLogger(), false)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	// Intentionally omit --text-selection
	cmd.SetArgs([]string{"add-inline", "PAGE1", "--body", "<p>inline</p>"})
	err := cmd.Execute()
	// cobra should return an error for missing required flag
	if err == nil {
		t.Error("expected error when --text-selection is missing, got nil")
	}
}

// --- page get: success ---

func TestPageGet_Success(t *testing.T) {
	svc := &mockConfluenceService{
		getPageFunc: func(_ context.Context, pageID, bodyFormat string) (*confluencesvc.Page, error) {
			if pageID != "PAGE42" {
				t.Errorf("expected pageID 'PAGE42', got %q", pageID)
			}
			return &confluencesvc.Page{
				ID:    "PAGE42",
				Title: "Architecture Overview",
			}, nil
		},
	}

	cmd := confluencecli.NewPageCmd(svc, audit.NewNoopLogger(), false)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"get", "PAGE42"})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "PAGE42") {
		t.Errorf("expected page ID in output, got: %q", out)
	}
	if !strings.Contains(out, "Architecture Overview") {
		t.Errorf("expected page title in output, got: %q", out)
	}
}

// --- page list: success ---

func TestPageList_Success(t *testing.T) {
	svc := &mockConfluenceService{
		getPagesInSpaceFunc: func(_ context.Context, spaceID string, _ int, _ string) (*confluencesvc.PageList, error) {
			if spaceID != "SPACE1" {
				t.Errorf("expected spaceID 'SPACE1', got %q", spaceID)
			}
			return &confluencesvc.PageList{
				Results: []confluencesvc.Page{
					{ID: "P1", Title: "Page One"},
				},
				NextCursor: "",
			}, nil
		},
	}

	cmd := confluencecli.NewPageCmd(svc, audit.NewNoopLogger(), false)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"list", "SPACE1"})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "P1") {
		t.Errorf("expected page ID 'P1' in output, got: %q", out)
	}
}

// --- spaces: success ---

func TestSpaces_Success(t *testing.T) {
	svc := &mockConfluenceService{
		getSpacesFunc: func(_ context.Context, _ int, _ string, _ []string, _ string) (*confluencesvc.SpaceList, error) {
			return &confluencesvc.SpaceList{
				Results: []confluencesvc.Space{
					{ID: "SP1", Key: "ENG", Name: "Engineering"},
				},
			}, nil
		},
	}

	cmd := confluencecli.NewSpacesCmd(svc)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "ENG") {
		t.Errorf("expected space key 'ENG' in output, got: %q", out)
	}
}

// --- search: success ---

func TestSearch_Success(t *testing.T) {
	svc := &mockConfluenceService{
		searchContentFunc: func(_ context.Context, cql string, _ int) ([]confluencesvc.SearchResult, error) {
			if cql != `type=page AND text~"golang"` {
				t.Errorf("unexpected CQL: %q", cql)
			}
			return []confluencesvc.SearchResult{
				{ContentID: "C1", Title: "Go Best Practices"},
			}, nil
		},
	}

	cmd := confluencecli.NewSearchCmd(svc)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{`type=page AND text~"golang"`})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "C1") {
		t.Errorf("expected content ID 'C1' in output, got: %q", out)
	}
}

// --- descendants: success ---

func TestPageDescendants_Success(t *testing.T) {
	svc := &mockConfluenceService{
		getPageDescendantsFunc: func(_ context.Context, pageID string, _ int, _ string) (*confluencesvc.PageRefList, error) {
			return &confluencesvc.PageRefList{
				Results: []confluencesvc.PageRef{
					{ID: "CHILD1", Title: "Child Page"},
				},
			}, nil
		},
	}

	cmd := confluencecli.NewPageCmd(svc, audit.NewNoopLogger(), false)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"descendants", "PARENT1"})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "CHILD1") {
		t.Errorf("expected child ID 'CHILD1' in output, got: %q", out)
	}
}

// --- comment footer: success ---

func TestCommentFooter_Success(t *testing.T) {
	svc := &mockConfluenceService{
		getFooterCommentsFunc: func(_ context.Context, pageID string, _ int, _ string) (*confluencesvc.CommentList, error) {
			return &confluencesvc.CommentList{
				Results: []confluencesvc.Comment{
					{ID: "FC1", Body: "Great page!"},
				},
			}, nil
		},
	}

	cmd := confluencecli.NewCommentCmd(svc, audit.NewNoopLogger(), false)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"footer", "PAGE1"})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "FC1") {
		t.Errorf("expected comment ID 'FC1' in output, got: %q", out)
	}
}

// --- comment inline: success ---

func TestCommentInline_Success(t *testing.T) {
	svc := &mockConfluenceService{
		getInlineCommentsFunc: func(_ context.Context, pageID string, _ int, _ string) (*confluencesvc.CommentList, error) {
			return &confluencesvc.CommentList{
				Results: []confluencesvc.Comment{
					{ID: "IC1", Body: "Needs clarification"},
				},
			}, nil
		},
	}

	cmd := confluencecli.NewCommentCmd(svc, audit.NewNoopLogger(), false)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"inline", "PAGE1"})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "IC1") {
		t.Errorf("expected comment ID 'IC1' in output, got: %q", out)
	}
}

// --- comment children: success ---

func TestCommentChildren_Success(t *testing.T) {
	svc := &mockConfluenceService{
		getCommentChildrenFunc: func(_ context.Context, commentID string, _ int, _ string) (*confluencesvc.CommentList, error) {
			return &confluencesvc.CommentList{
				Results: []confluencesvc.Comment{
					{ID: "REPLY1", Body: "Agreed!"},
				},
			}, nil
		},
	}

	cmd := confluencecli.NewCommentCmd(svc, audit.NewNoopLogger(), false)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"children", "CMT1"})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "REPLY1") {
		t.Errorf("expected reply ID 'REPLY1' in output, got: %q", out)
	}
}

// --- error mapping ---

func TestExitCodeForError_NotFound(t *testing.T) {
	_, err := (&mockConfluenceService{
		getPageFunc: func(_ context.Context, _, _ string) (*confluencesvc.Page, error) {
			return nil, confluencesvc.ErrNotFound
		},
	}).GetPage(context.Background(), "MISSING", "storage")

	if !errors.Is(err, confluencesvc.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

func TestExitCodeForError_Unauthorized(t *testing.T) {
	_, err := (&mockConfluenceService{
		getPageFunc: func(_ context.Context, _, _ string) (*confluencesvc.Page, error) {
			return nil, confluencesvc.ErrUnauthorized
		},
	}).GetPage(context.Background(), "PAGE1", "storage")

	if !errors.Is(err, confluencesvc.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got: %v", err)
	}
}
