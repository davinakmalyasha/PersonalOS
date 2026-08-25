package store

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/davinakmalyasha/PersonalOS/services/api/internal/domain/planner"
)

// ---- Tasks ----

type Task struct {
	ID             string   `json:"id"`
	Title          string   `json:"title"`
	Notes          string   `json:"notes"`
	Status         string   `json:"status"`
	Priority       string   `json:"priority"`
	DueDate        *string  `json:"due_date"`
	Project        *string  `json:"project"`
	RecurrenceRule *string  `json:"recurrence_rule"`
	ParentID       *string  `json:"parent_id"`
	BlockedBy      *string  `json:"blocked_by"`
	EstimateMin    *int     `json:"estimate_minutes"`
	Tags           []string `json:"tags"`
	CreatedAt      string   `json:"created_at"`
	UpdatedAt      string   `json:"updated_at"`
	CompletedAt    *string  `json:"completed_at"`

	tagsRaw string
}

type TaskFilter struct {
	Status    string // todo|doing|done|"" ; "open" = todo+doing
	Priority  string
	Due       string // exact date
	DueBefore string // due on/before date (overdue scans)
	Project   string
	ParentID  string // subtask filter; "root" = top-level only
	Tag       string
	Q         string
	Page      int
	PageSize  int
}

func validStatus(s string) bool   { return s == "todo" || s == "doing" || s == "done" }
func validPriority(p string) bool { return p == "low" || p == "med" || p == "high" }

func joinTags(tags []string) string {
	if len(tags) == 0 {
		return "[]"
	}
	quoted := make([]string, len(tags))
	for i, t := range tags {
		quoted[i] = `"` + strings.ReplaceAll(t, `"`, `\"`) + `"`
	}
	return "[" + strings.Join(quoted, ",") + "]"
}

func splitTags(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "[]" {
		return []string{}
	}
	raw = strings.TrimPrefix(raw, "[")
	raw = strings.TrimSuffix(raw, "]")
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.Trim(strings.TrimSpace(p), `"`); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func (p *Planner) CreateTask(title, notes, status, priority string, dueDate, project *string, tags []string, recurrenceRule *string, parentID, blockedBy *string, estimateMin *int) (Task, error) {
	if status == "" {
		status = "todo"
	}
	if priority == "" {
		priority = "med"
	}
	if !validStatus(status) || !validPriority(priority) {
		return Task{}, ErrInvalid
	}
	if recurrenceRule != nil && strings.TrimSpace(*recurrenceRule) != "" {
		if _, err := planner.ParseRecurrence(*recurrenceRule); err != nil {
			return Task{}, ErrInvalid
		}
	} else {
		recurrenceRule = nil
	}
	// Subtask rules: parent must exist and be a top-level task (no grandchildren).
	if parentID != nil && *parentID != "" {
		parent, err := p.GetTask(*parentID)
		if err != nil {
			return Task{}, err
		}
		if parent.ParentID != nil {
			return Task{}, ErrInvalid
		}
	} else {
		parentID = nil
	}
	if blockedBy != nil && *blockedBy != "" {
		if _, err := p.GetTask(*blockedBy); err != nil {
			return Task{}, err
		}
	} else {
		blockedBy = nil
	}
	if estimateMin != nil && *estimateMin < 0 {
		return Task{}, ErrInvalid
	}
	now := NowRFC3339()
	var due, proj, rec, parent, blocked, est interface{}
	if dueDate != nil && *dueDate != "" {
		due = *dueDate
	}
	if project != nil && *project != "" {
		proj = *project
	}
	if recurrenceRule != nil {
		rule := strings.TrimSpace(*recurrenceRule)
		rec = rule
	}
	if parentID != nil {
		parent = *parentID
	}
	if blockedBy != nil {
		blocked = *blockedBy
	}
	if estimateMin != nil {
		est = *estimateMin
	}
	var completed interface{}
	if status == "done" {
		completed = now
	}
	t := Task{
		ID: NewID(), Title: title, Notes: notes, Status: status, Priority: priority,
		Tags: normalizeTagList(tags), CreatedAt: now, UpdatedAt: now,
	}
	if recurrenceRule != nil {
		rule := strings.TrimSpace(*recurrenceRule)
		t.RecurrenceRule = &rule
	}
	if parentID != nil {
		v := *parentID
		t.ParentID = &v
	}
	if blockedBy != nil {
		v := *blockedBy
		t.BlockedBy = &v
	}
	if estimateMin != nil {
		v := *estimateMin
		t.EstimateMin = &v
	}
	if due != nil {
		v := due.(string)
		t.DueDate = &v
	}
	if proj != nil {
		v := proj.(string)
		t.Project = &v
	}
	if status == "done" {
		t.CompletedAt = &now
	}
	_, err := p.DB.Exec(`
		INSERT INTO tasks (id,title,notes,status,priority,due_date,project,recurrence_rule,parent_id,blocked_by,estimate_minutes,tags,created_at,updated_at,completed_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		t.ID, t.Title, t.Notes, t.Status, t.Priority, due, proj, rec, parent, blocked, est, joinTags(t.Tags), now, now, completed)
	if err != nil {
		return Task{}, err
	}
	logChange(p.DB, "task", t.ID, "create", t.Title)
	return p.GetTask(t.ID)
}

const taskCols = `id,title,notes,status,priority,due_date,project,recurrence_rule,parent_id,blocked_by,estimate_minutes,tags,created_at,updated_at,completed_at`

func taskScan(t *Task, tagsRaw *string) []interface{} {
	return []interface{}{&t.ID, &t.Title, &t.Notes, &t.Status, &t.Priority,
		&t.DueDate, &t.Project, &t.RecurrenceRule, &t.ParentID, &t.BlockedBy, &t.EstimateMin,
		tagsRaw, &t.CreatedAt, &t.UpdatedAt, &t.CompletedAt}
}

func (p *Planner) getTaskBy(q string, args ...interface{}) (Task, error) {
	var t Task
	err := p.DB.QueryRow(`SELECT `+taskCols+` FROM tasks WHERE `+q, args...).
		Scan(taskScan(&t, &t.tagsRaw)...)
	if errors.Is(err, sql.ErrNoRows) {
		return Task{}, ErrNotFound
	}
	if err != nil {
		return Task{}, err
	}
	t.Tags = splitTags(t.tagsRaw)
	return t, nil
}

func (p *Planner) GetTask(id string) (Task, error) { return p.getTaskBy(`id=?`, id) }

func (p *Planner) buildTaskWhere(f TaskFilter) (string, []interface{}) {
	where := []string{"1=1"}
	args := []interface{}{}
	switch f.Status {
	case "":
	case "open":
		where = append(where, "status IN ('todo','doing')")
	default:
		where = append(where, "status=?")
		args = append(args, f.Status)
	}
	if f.Priority != "" {
		where = append(where, "priority=?")
		args = append(args, f.Priority)
	}
	if f.Due != "" {
		where = append(where, "due_date=?")
		args = append(args, f.Due)
	}
	if f.DueBefore != "" {
		where = append(where, "due_date IS NOT NULL AND due_date<=?")
		args = append(args, f.DueBefore)
	}
	if f.Project != "" {
		where = append(where, "project=?")
		args = append(args, f.Project)
	}
	switch f.ParentID {
	case "":
	case "root":
		where = append(where, "parent_id IS NULL")
	default:
		where = append(where, "parent_id=?")
		args = append(args, f.ParentID)
	}
	if f.Tag != "" {
		where = append(where, "tags LIKE ?")
		args = append(args, `%"`+f.Tag+`"%`)
	}
	if f.Q != "" {
		where = append(where, "(LOWER(title) LIKE ? OR LOWER(notes) LIKE ?)")
		pat := "%" + strings.ToLower(f.Q) + "%"
		args = append(args, pat, pat)
	}
	return strings.Join(where, " AND "), args
}

func (p *Planner) ListTasks(f TaskFilter) ([]Task, int, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 || f.PageSize > 100 {
		f.PageSize = 100
	}
	whereSQL, args := p.buildTaskWhere(f)

	var total int
	if err := p.DB.QueryRow(`SELECT COUNT(*) FROM tasks WHERE `+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	order := `ORDER BY COALESCE(due_date,'9999-12-31'), CASE priority WHEN 'high' THEN 0 WHEN 'med' THEN 1 ELSE 2 END, created_at DESC`
	q := `SELECT ` + taskCols + ` FROM tasks WHERE ` + whereSQL + ` ` + order + ` LIMIT ? OFFSET ?`
	pageArgs := append(append([]interface{}{}, args...), f.PageSize, (f.Page-1)*f.PageSize)
	rows, err := p.DB.Query(q, pageArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []Task{}
	for rows.Next() {
		var t Task
		if err := rows.Scan(taskScan(&t, &t.tagsRaw)...); err != nil {
			return nil, 0, err
		}
		t.Tags = splitTags(t.tagsRaw)
		out = append(out, t)
	}
	return out, total, rows.Err()
}

type TaskUpdate struct {
	Title          *string
	Notes          *string
	Status         *string
	Priority       *string
	DueDate        *string // empty string clears
	Project        *string // empty string clears
	RecurrenceRule *string // empty string clears; validated
	ParentID       *string // set once at create; PATCH only validates/clears
	BlockedBy      *string // empty string clears; validated
	EstimateMin    **int   // ptr-to-nil clears
	Tags           *[]string
}

func (p *Planner) UpdateTask(id string, u TaskUpdate) (Task, error) {
	cur, err := p.GetTask(id)
	if err != nil {
		return Task{}, err
	}
	if u.Title != nil && *u.Title != "" {
		cur.Title = *u.Title
	}
	if u.Notes != nil {
		cur.Notes = *u.Notes
	}
	if u.Priority != nil {
		if !validPriority(*u.Priority) {
			return Task{}, ErrInvalid
		}
		cur.Priority = *u.Priority
	}
	statusChanged := false
	if u.Status != nil {
		if !validStatus(*u.Status) {
			return Task{}, ErrInvalid
		}
		statusChanged = cur.Status != *u.Status
		cur.Status = *u.Status
	}
	if u.Tags != nil {
		cur.Tags = normalizeTagList(*u.Tags)
	}
	clearDue := u.DueDate != nil && *u.DueDate == ""
	if u.DueDate != nil && !clearDue {
		cur.DueDate = u.DueDate
	} else if clearDue {
		cur.DueDate = nil
	}
	clearProject := u.Project != nil && *u.Project == ""
	if u.Project != nil && !clearProject {
		cur.Project = u.Project
	} else if clearProject {
		cur.Project = nil
	}
	if u.RecurrenceRule != nil {
		if strings.TrimSpace(*u.RecurrenceRule) == "" {
			cur.RecurrenceRule = nil
		} else {
			rule := strings.TrimSpace(*u.RecurrenceRule)
			if _, verr := planner.ParseRecurrence(rule); verr != nil {
				return Task{}, ErrInvalid
			}
			cur.RecurrenceRule = &rule
		}
	}
	if u.ParentID != nil {
		if strings.TrimSpace(*u.ParentID) == "" {
			cur.ParentID = nil
		} else {
			parent, perr := p.GetTask(*u.ParentID)
			if perr != nil {
				return Task{}, perr
			}
			if parent.ParentID != nil {
				return Task{}, ErrInvalid // no grandchildren
			}
			cur.ParentID = u.ParentID
		}
	}
	if u.BlockedBy != nil {
		if strings.TrimSpace(*u.BlockedBy) == "" || *u.BlockedBy == id {
			cur.BlockedBy = nil
		} else {
			if _, berr := p.GetTask(*u.BlockedBy); berr != nil {
				return Task{}, berr
			}
			cur.BlockedBy = u.BlockedBy
		}
	}
	if u.EstimateMin != nil {
		if *u.EstimateMin == nil || **u.EstimateMin >= 0 {
			cur.EstimateMin = *u.EstimateMin
		} else {
			return Task{}, ErrInvalid
		}
	}

	now := NowRFC3339()
	cur.UpdatedAt = now
	spawnNext := false
	if statusChanged {
		if cur.Status == "done" {
			cur.CompletedAt = &now
			if cur.RecurrenceRule != nil {
				spawnNext = true
			}
		} else {
			cur.CompletedAt = nil
		}
	}

	var due, proj, rec, parent, blocked, est, completed interface{}
	if cur.DueDate != nil {
		due = *cur.DueDate
	}
	if cur.Project != nil {
		proj = *cur.Project
	}
	if cur.RecurrenceRule != nil {
		rec = *cur.RecurrenceRule
	}
	if cur.ParentID != nil {
		parent = *cur.ParentID
	}
	if cur.BlockedBy != nil {
		blocked = *cur.BlockedBy
	}
	if cur.EstimateMin != nil {
		est = *cur.EstimateMin
	}
	if cur.CompletedAt != nil {
		completed = *cur.CompletedAt
	}
	_, err = p.DB.Exec(`
		UPDATE tasks SET title=?, notes=?, status=?, priority=?, due_date=?, project=?, recurrence_rule=?, parent_id=?, blocked_by=?, estimate_minutes=?, tags=?, updated_at=?, completed_at=?
		WHERE id=?`,
		cur.Title, cur.Notes, cur.Status, cur.Priority, due, proj, rec, parent, blocked, est, joinTags(cur.Tags), now, completed, id)
	if err != nil {
		return Task{}, err
	}
	if spawnNext {
		if _, err := p.spawnRecurringInstance(cur); err != nil {
			return Task{}, err
		}
	}
	action := "update"
	if statusChanged && cur.Status == "done" {
		action = "complete"
	}
	logChange(p.DB, "task", id, action, cur.Title)
	return p.GetTask(id)
}

// spawnRecurringInstance creates the next open instance of a completed
// recurring task: same fields, due date advanced one interval from the
// completed task's due date (or completion day when undated).
func (p *Planner) spawnRecurringInstance(done Task) (Task, error) {
	rule, err := planner.ParseRecurrence(*done.RecurrenceRule)
	if err != nil {
		return Task{}, ErrInvalid
	}
	// Respect series end: no spawn past UNTIL.
	if rule.Until != "" {
		if u, perr := time.Parse("20060102", rule.Until); perr == nil {
			base := time.Now().UTC()
			if done.DueDate != nil {
				if t, derr := time.Parse("2006-01-02", *done.DueDate); derr == nil {
					base = t
				}
			}
			if u.Before(base.AddDate(0, 0, 1)) {
				return done, nil // series exhausted â€” keep the completed task as-is
			}
		}
	}
	base := time.Now().UTC()
	if done.DueDate != nil {
		if t, perr := time.Parse("2006-01-02", *done.DueDate); perr == nil {
			base = t
		}
	}
	next := nextOccurrenceAfter(rule, base)
	nextDue := next.Format("2006-01-02")
	dueP := &nextDue
	return p.CreateTask(done.Title, done.Notes, "todo", done.Priority, dueP, done.Project, done.Tags, done.RecurrenceRule, nil, nil, nil)
}

// nextOccurrenceAfter returns the first occurrence date strictly after base,
// reusing the domain Expand (COUNT/UNTIL respected; unbounded rules bounded by
// the 1-year scan window).
func nextOccurrenceAfter(r planner.Recurrence, base time.Time) time.Time {
	from := base.AddDate(0, 0, 1)
	days := r.Expand(base, from.Format("2006-01-02"), base.AddDate(1, 0, 0).Format("2006-01-02"))
	if len(days) == 0 {
		return base.AddDate(0, 0, 1)
	}
	t, err := time.Parse("2006-01-02", days[0])
	if err != nil {
		return base.AddDate(0, 0, 1)
	}
	return t
}

func (p *Planner) DeleteTask(id string) error {
	cur, err := p.GetTask(id)
	if err != nil {
		return err
	}
	res, err := p.DB.Exec(`DELETE FROM tasks WHERE id=?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	logChange(p.DB, "task", id, "delete", cur.Title)
	return nil
}

func normalizeTagList(tags []string) []string {
	out := make([]string, 0, len(tags))
	seen := map[string]bool{}
	for _, t := range tags {
		t = strings.TrimSpace(t)
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	return out
}
