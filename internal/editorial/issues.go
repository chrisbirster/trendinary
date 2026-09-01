package editorial

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

func (s *Store) Issues(ctx context.Context) ([]NewsletterIssue, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,title,status,issue_date,intro,question,created_at,updated_at FROM newsletter_issues ORDER BY created_at DESC`)
	if err != nil { return nil, err }
	defer rows.Close()
	out := []NewsletterIssue{}
	for rows.Next() {
		value, err := scanIssue(rows)
		if err != nil { return nil, err }
		out = append(out, value)
	}
	return out, rows.Err()
}

func (s *Store) Issue(ctx context.Context, id string) (NewsletterIssue, bool, error) {
	value, err := scanIssue(s.db.QueryRowContext(ctx, `SELECT id,title,status,issue_date,intro,question,created_at,updated_at FROM newsletter_issues WHERE id=?`, id))
	if errors.Is(err, sql.ErrNoRows) { return NewsletterIssue{}, false, nil }
	if err != nil { return NewsletterIssue{}, false, err }
	items, err := s.IssueItems(ctx, id)
	if err != nil { return NewsletterIssue{}, false, err }
	value.Items = items
	return value, true, nil
}

func (s *Store) PutIssue(ctx context.Context, issue NewsletterIssue) (NewsletterIssue, error) {
	if issue.ID == "" { issue.ID = NewID("issue") }
	if issue.Title == "" { issue.Title = "Untitled issue" }
	if issue.Status == "" { issue.Status = "draft" }
	if issue.Status != "draft" && issue.Status != "ready" && issue.Status != "published" && issue.Status != "archived" { return NewsletterIssue{}, fmt.Errorf("invalid issue status") }
	now := time.Now().UTC()
	if issue.CreatedAt.IsZero() { issue.CreatedAt = now }
	issue.UpdatedAt = now
	_, err := s.db.ExecContext(ctx, `INSERT INTO newsletter_issues (id,title,status,issue_date,intro,question,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET title=excluded.title,status=excluded.status,issue_date=excluded.issue_date,intro=excluded.intro,question=excluded.question,updated_at=excluded.updated_at`, issue.ID,issue.Title,issue.Status,issue.IssueDate,issue.Intro,issue.Question,issue.CreatedAt.Format(time.RFC3339Nano),issue.UpdatedAt.Format(time.RFC3339Nano))
	return issue, err
}

func (s *Store) AddIssueItem(ctx context.Context, item NewsletterIssueItem) (NewsletterIssueItem, error) {
	if item.ID == "" { item.ID = NewID("issueitem") }
	if item.Section == "" { item.Section = "misc" }
	if item.Section != "thinking" && item.Section != "worth_your_time" && item.Section != "question_source" && item.Section != "misc" { return NewsletterIssueItem{}, fmt.Errorf("invalid issue section") }
	_, err := s.db.ExecContext(ctx, `INSERT INTO newsletter_issue_items (id,issue_id,content_item_id,section,position,editor_note) VALUES (?,?,?,?,?,?) ON CONFLICT(issue_id,content_item_id,section) DO UPDATE SET position=excluded.position,editor_note=excluded.editor_note`, item.ID,item.IssueID,item.ContentItemID,item.Section,item.Position,item.EditorNote)
	return item, err
}

func (s *Store) RemoveIssueItem(ctx context.Context, issueID, itemID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM newsletter_issue_items WHERE issue_id=? AND id=?`, issueID,itemID)
	return err
}

func (s *Store) IssueItems(ctx context.Context, issueID string) ([]NewsletterIssueItem, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,issue_id,content_item_id,section,position,editor_note FROM newsletter_issue_items WHERE issue_id=? ORDER BY section,position,id`, issueID)
	if err != nil { return nil, err }
	defer rows.Close()
	out := []NewsletterIssueItem{}
	for rows.Next() {
		var item NewsletterIssueItem
		if err := rows.Scan(&item.ID,&item.IssueID,&item.ContentItemID,&item.Section,&item.Position,&item.EditorNote); err != nil { return nil, err }
		if content, ok, err := s.Content(ctx,item.ContentItemID); err == nil && ok { item.Content=&content }
		out=append(out,item)
	}
	return out,rows.Err()
}

func scanIssue(row rowScanner) (NewsletterIssue,error) {
	var value NewsletterIssue
	var created,updated string
	if err:=row.Scan(&value.ID,&value.Title,&value.Status,&value.IssueDate,&value.Intro,&value.Question,&created,&updated); err!=nil{return NewsletterIssue{},err}
	value.CreatedAt,_=time.Parse(time.RFC3339Nano,created); value.UpdatedAt,_=time.Parse(time.RFC3339Nano,updated)
	return value,nil
}
