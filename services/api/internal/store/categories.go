package store

import (
	"database/sql"
	"errors"
)

type Category struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	ParentID  *string `json:"parent_id"`
	Color     *string `json:"color"`
	CreatedAt string  `json:"created_at"`
}

func (f *Finance) CreateCategory(name string, parentID, color *string) (Category, error) {
	if parentID != nil {
		if _, err := f.GetCategory(*parentID); err != nil {
			return Category{}, err
		}
	}
	c := Category{ID: NewID(), Name: name, ParentID: parentID, Color: color, CreatedAt: NowRFC3339()}
	_, err := f.DB.Exec(`INSERT INTO categories (id,name,parent_id,color,created_at) VALUES (?,?,?,?,?)`,
		c.ID, c.Name, c.ParentID, c.Color, c.CreatedAt)
	if isUniqueErr(err) {
		return Category{}, ErrConflict // duplicate name under same parent
	}
	return c, err
}

func (f *Finance) GetCategory(id string) (Category, error) {
	var c Category
	err := f.DB.QueryRow(`SELECT id,name,parent_id,color,created_at FROM categories WHERE id=?`, id).
		Scan(&c.ID, &c.Name, &c.ParentID, &c.Color, &c.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Category{}, ErrNotFound
	}
	return c, err
}

func (f *Finance) ListCategories() ([]Category, error) {
	rows, err := f.DB.Query(`SELECT id,name,parent_id,color,created_at FROM categories ORDER BY name, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Category
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.Name, &c.ParentID, &c.Color, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// UpdateCategory changes name/color and optionally re-parents. Re-parenting
// into itself or one of its descendants is rejected to avoid cycles.
func (f *Finance) UpdateCategory(id string, name *string, parentID, color **string) (Category, error) {
	cur, err := f.GetCategory(id)
	if err != nil {
		return Category{}, err
	}
	newParent := cur.ParentID
	if parentID != nil {
		newParent = *parentID
		if newParent != nil {
			if *newParent == id {
				return Category{}, ErrInvalid
			}
			if f.isDescendant(id, *newParent) {
				return Category{}, ErrInvalid
			}
			if _, err := f.GetCategory(*newParent); err != nil {
				return Category{}, err
			}
		}
	}
	newName := cur.Name
	if name != nil {
		newName = *name
	}
	var newColor interface{}
	if color != nil {
		if *color == nil {
			newColor = nil
		} else {
			newColor = **color
		}
	} else if cur.Color != nil {
		newColor = *cur.Color
	}
	_, err = f.DB.Exec(`UPDATE categories SET name=?, parent_id=?, color=? WHERE id=?`,
		newName, newParent, newColor, id)
	if isUniqueErr(err) {
		return Category{}, ErrConflict
	}
	if err != nil {
		return Category{}, err
	}
	return f.GetCategory(id)
}

func (f *Finance) isDescendant(ancestorID, candidateID string) bool {
	parent := candidateID
	for depth := 0; depth < 32 && parent != ""; depth++ {
		if parent == ancestorID {
			return true
		}
		var p sql.NullString
		if err := f.DB.QueryRow(`SELECT parent_id FROM categories WHERE id=?`, parent).Scan(&p); err != nil {
			return false
		}
		if !p.Valid {
			return false
		}
		parent = p.String
	}
	return false
}

// DeleteCategory removes a category. Transactions referencing it are either
// blocked or reassigned; child categories always block.
func (f *Finance) DeleteCategory(id string, reassignTo *string) error {
	var children int
	if err := f.DB.QueryRow(`SELECT COUNT(*) FROM categories WHERE parent_id=?`, id).Scan(&children); err != nil {
		return err
	}
	if children > 0 {
		return ErrConflict
	}
	var used int
	if err := f.DB.QueryRow(`SELECT COUNT(*) FROM transactions WHERE category_id=?`, id).Scan(&used); err != nil {
		return err
	}

	tx, err := f.DB.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if used > 0 {
		if reassignTo == nil {
			return ErrConflict
		}
		if _, err := tx.Exec(`UPDATE transactions SET category_id=? WHERE category_id=?`, *reassignTo, id); err != nil {
			return err
		}
	}
	res, err := tx.Exec(`DELETE FROM categorization_rules WHERE category_id=?`, id)
	if err != nil {
		return err
	}
	_, _ = res.RowsAffected()
	res2, err := tx.Exec(`DELETE FROM budgets WHERE category_id=?`, id)
	if err != nil {
		return err
	}
	_, _ = res2.RowsAffected()
	res3, err := tx.Exec(`DELETE FROM categories WHERE id=?`, id)
	if err != nil {
		return err
	}
	if n, _ := res3.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return tx.Commit()
}
