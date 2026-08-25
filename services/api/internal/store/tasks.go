package store

import (
	"database/sql"
	"errors"
	"strings"
)

// ---- Tasks ----

type Task struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Notes       string   `json:"notes"`
	Status      string   `json:"status"`
	Priority    string   `json:"priority"`
	DueDate     *string  `json:"due_date"`
	Project     *string  `json:"project"`
	Tags        []string `json:"tags"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
	CompletedAt *string  `json:"completed_at"`

	tagsRaw string
}

type TaskFilter struct {
	Status    string // todo|doing|done|"" ; "open" = todo+doing
	Priority  string
	Due       string // exact date
	DueBefore string // due on/before date (overdue scans)
	Project   string
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

func (p *Planner) CreateTask(title, notes, status, priority string, dueDate, project *string, tags []string) (Task, error) {
	if status == "" {
		status = "todo"
	}
	if priority == "" {
		priority = "med"
	}
	if !validStatus(status) || !validPriority(priority) {
		return Task{}, ErrInvalid
	}
	now := NowRFC3339()
	var due, proj interface{}
	if dueDate != nil && *dueDate != "" {
		due = *dueDate
	}
	if project != nil && *project != "" {
		proj = *project
	}
	var completed interface{}
	if status == "done" {
		completed = now
	}
	t := Task{
		ID: NewID(), Title: title, Notes: notes, Status: status, Priority: priority,
		Tags: normalizeTagList(tags), CreatedAt: now, UpdatedAt: now,
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
		INSERT INTO tasks (id,title,notes,status,priority,due_date,project,tags,created_at,updated_at,completed_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		t.ID, t.Title, t.Notes, t.Status, t.Priority, due, proj, joinTags(t.Tags), now, now, completed)
	if err != nil {
		return Task{}, err
	}
	return p.GetTask(t.ID)
}

const taskCols = `id,title,notes,status,priority,due_date,project,tags,created_at,updated_at,completed_at`

func taskScan(t *Task, tagsRaw *string) []interface{} {
	return []interface{}{&t.ID, &t.Title, &t.Notes, &t.Status, &t.Priority,
		&t.DueDate, &t.Project, tagsRaw, &t.CreatedAt, &t.UpdatedAt, &t.CompletedAt}
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
	Title    *string
	Notes    *string
	Status   *string
	Priority *string
	DueDate  *string // empty string clears
	Project  *string // empty string clears
	Tags     *[]string
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

	now := NowRFC3339()
	cur.UpdatedAt = now
	if statusChanged {
		if cur.Status == "done" {
			cur.CompletedAt = &now
		} else {
			cur.CompletedAt = nil
		}
	}

	var due, proj, completed interface{}
	if cur.DueDate != nil {
		due = *cur.DueDate
	}
	if cur.Project != nil {
		proj = *cur.Project
	}
	if cur.CompletedAt != nil {
		completed = *cur.CompletedAt
	}
	_, err = p.DB.Exec(`
		UPDATE tasks SET title=?, notes=?, status=?, priority=?, due_date=?, project=?, tags=?, updated_at=?, completed_at=?
		WHERE id=?`,
		cur.Title, cur.Notes, cur.Status, cur.Priority, due, proj, joinTags(cur.Tags), now, completed, id)
	if err != nil {
		return Task{}, err
	}
	return p.GetTask(id)
}

func (p *Planner) DeleteTask(id string) error {
	res, err := p.DB.Exec(`DELETE FROM tasks WHERE id=?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
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
