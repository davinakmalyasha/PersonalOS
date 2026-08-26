package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// ---- Recipes + grocery list ----

type Recipe struct {
	ID                 string          `json:"id"`
	Title              string          `json:"title"`
	Ingredients        json.RawMessage `json:"ingredients"` // JSON array of {name, qty, unit}
	Instructions       string          `json:"instructions"`
	Servings           *int64          `json:"servings"`
	CaloriesPerServing *int64          `json:"calories_per_serving"`
	Tags               []string        `json:"tags"`
	CreatedAt          string          `json:"created_at"`
	UpdatedAt          string          `json:"updated_at"`

	tagsRaw string
}

const recipeCols = `id,title,ingredients,instructions,servings,calories_per_serving,tags,created_at,updated_at`

func recipeScan(r *Recipe, tagsRaw *string) []interface{} {
	return []interface{}{&r.ID, &r.Title, &r.Ingredients, &r.Instructions,
		&r.Servings, &r.CaloriesPerServing, tagsRaw, &r.CreatedAt, &r.UpdatedAt}
}

func (h *Health) CreateRecipe(title, ingredientsJSON, instructions string, servings, caloriesPerServing *int64, tags []string) (Recipe, error) {
	if strings.TrimSpace(title) == "" {
		return Recipe{}, ErrInvalid
	}
	if !validJSONArray(ingredientsJSON) {
		return Recipe{}, ErrInvalid
	}
	if (servings != nil && *servings <= 0) || (caloriesPerServing != nil && *caloriesPerServing < 0) {
		return Recipe{}, ErrInvalid
	}
	now := NowRFC3339()
	r := Recipe{
		ID: NewID(), Title: title, Ingredients: rawJSONArray(ingredientsJSON),
		Instructions: instructions, Servings: servings, CaloriesPerServing: caloriesPerServing,
		Tags: normalizeTagList(tags), CreatedAt: now, UpdatedAt: now,
	}
	var serv, cal interface{}
	if servings != nil {
		serv = *servings
	}
	if caloriesPerServing != nil {
		cal = *caloriesPerServing
	}
	_, err := h.DB.Exec(
		`INSERT INTO recipes (`+recipeCols+`) VALUES (?,?,?,?,?,?,?,?,?)`,
		r.ID, r.Title, r.Ingredients, r.Instructions, serv, cal, joinTags(r.Tags), now, now)
	if err != nil {
		return Recipe{}, err
	}
	return h.GetRecipe(r.ID)
}

func (h *Health) GetRecipe(id string) (Recipe, error) {
	var r Recipe
	err := h.DB.QueryRow(`SELECT `+recipeCols+` FROM recipes WHERE id=?`, id).
		Scan(recipeScan(&r, &r.tagsRaw)...)
	if errors.Is(err, sql.ErrNoRows) {
		return Recipe{}, ErrNotFound
	}
	if err != nil {
		return Recipe{}, err
	}
	r.Tags = splitTags(r.tagsRaw)
	return r, nil
}

type RecipeFilter struct {
	Tag      string
	Q        string
	Page     int
	PageSize int
}

func (h *Health) ListRecipes(f RecipeFilter) ([]Recipe, int, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 || f.PageSize > 100 {
		f.PageSize = 50
	}
	where := []string{"1=1"}
	args := []interface{}{}
	if f.Tag != "" {
		where = append(where, "tags LIKE ?")
		args = append(args, `%"`+f.Tag+`"%`)
	}
	if f.Q != "" {
		where = append(where, "(LOWER(title) LIKE ? OR LOWER(instructions) LIKE ?)")
		pat := "%" + strings.ToLower(f.Q) + "%"
		args = append(args, pat, pat)
	}
	whereSQL := strings.Join(where, " AND ")
	var total int
	if err := h.DB.QueryRow(`SELECT COUNT(*) FROM recipes WHERE `+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	q := `SELECT ` + recipeCols + ` FROM recipes WHERE ` + whereSQL +
		` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	pageArgs := append(append([]interface{}{}, args...), f.PageSize, (f.Page-1)*f.PageSize)
	rows, err := h.DB.Query(q, pageArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []Recipe{}
	for rows.Next() {
		var r Recipe
		if err := rows.Scan(recipeScan(&r, &r.tagsRaw)...); err != nil {
			return nil, 0, err
		}
		r.Tags = splitTags(r.tagsRaw)
		out = append(out, r)
	}
	return out, total, rows.Err()
}

type RecipeUpdate struct {
	Title              *string
	Ingredients        *string
	Instructions       *string
	Servings           **int64
	CaloriesPerServing **int64
	Tags               *[]string
}

func (h *Health) UpdateRecipe(id string, u RecipeUpdate) (Recipe, error) {
	cur, err := h.GetRecipe(id)
	if err != nil {
		return Recipe{}, err
	}
	if u.Title != nil && strings.TrimSpace(*u.Title) != "" {
		cur.Title = *u.Title
	}
	if u.Ingredients != nil {
		if !validJSONArray(*u.Ingredients) {
			return Recipe{}, ErrInvalid
		}
		cur.Ingredients = rawJSONArray(*u.Ingredients)
	}
	if u.Instructions != nil {
		cur.Instructions = *u.Instructions
	}
	if u.Servings != nil {
		if *u.Servings == nil || **u.Servings > 0 {
			cur.Servings = *u.Servings
		} else {
			return Recipe{}, ErrInvalid
		}
	}
	if u.CaloriesPerServing != nil {
		if *u.CaloriesPerServing == nil || **u.CaloriesPerServing >= 0 {
			cur.CaloriesPerServing = *u.CaloriesPerServing
		} else {
			return Recipe{}, ErrInvalid
		}
	}
	if u.Tags != nil {
		cur.Tags = normalizeTagList(*u.Tags)
	}
	cur.UpdatedAt = NowRFC3339()

	var serv, cal interface{}
	if cur.Servings != nil {
		serv = *cur.Servings
	}
	if cur.CaloriesPerServing != nil {
		cal = *cur.CaloriesPerServing
	}
	_, err = h.DB.Exec(
		`UPDATE recipes SET title=?, ingredients=?, instructions=?, servings=?, calories_per_serving=?, tags=?, updated_at=? WHERE id=?`,
		cur.Title, cur.Ingredients, cur.Instructions, serv, cal, joinTags(cur.Tags), cur.UpdatedAt, id)
	if err != nil {
		return Recipe{}, err
	}
	return h.GetRecipe(id)
}

func (h *Health) DeleteRecipe(id string) error {
	res, err := h.DB.Exec(`DELETE FROM recipes WHERE id=?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// UseRecipeAsMeal copies a recipe into a logged meal: ingredients become the
// meal items, calories_per_serving carries over. eatenAt is RFC3339.
func (h *Health) UseRecipeAsMeal(id, eatenAt string, servings *int64) (Meal, error) {
	r, err := h.GetRecipe(id)
	if err != nil {
		return Meal{}, err
	}
	nServ := int64(1)
	if servings != nil && *servings > 0 {
		nServ = *servings
	} else if r.Servings != nil && *r.Servings > 0 {
		nServ = *r.Servings
	}
	var cal *int64
	if r.CaloriesPerServing != nil {
		c := *r.CaloriesPerServing * nServ
		cal = &c
	}
	title := fmt.Sprintf("%s (from recipe)", r.Title)
	return h.CreateMeal(eatenAt, title, "", string(r.Ingredients), cal, nil, nil, nil, r.Tags)
}

// ---- Grocery list ----

type GroceryItem struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Qty       string  `json:"qty"`
	Unit      *string `json:"unit"`
	Checked   bool    `json:"checked"`
	RecipeID  *string `json:"recipe_id"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

const groceryCols = `id,name,qty,unit,checked,recipe_id,created_at,updated_at`

func groceryScan(g *GroceryItem) []interface{} {
	return []interface{}{&g.ID, &g.Name, &g.Qty, &g.Unit, &g.Checked, &g.RecipeID, &g.CreatedAt, &g.UpdatedAt}
}

func (h *Health) CreateGroceryItem(name, qty string, unit *string, recipeID *string) (GroceryItem, error) {
	if strings.TrimSpace(name) == "" {
		return GroceryItem{}, ErrInvalid
	}
	g := GroceryItem{
		ID: NewID(), Name: name, Qty: qty,
		CreatedAt: NowRFC3339(), UpdatedAt: NowRFC3339(),
	}
	if unit != nil && *unit != "" {
		g.Unit = unit
	}
	if recipeID != nil && *recipeID != "" {
		if _, err := h.GetRecipe(*recipeID); err != nil {
			return GroceryItem{}, err
		}
		g.RecipeID = recipeID
	}
	_, err := h.DB.Exec(
		`INSERT INTO grocery_items (`+groceryCols+`) VALUES (?,?,?,?,0,?,?,?)`,
		g.ID, g.Name, g.Qty, g.Unit, g.RecipeID, g.CreatedAt, g.UpdatedAt)
	if err != nil {
		return GroceryItem{}, err
	}
	return h.GetGroceryItem(g.ID)
}

func (h *Health) GetGroceryItem(id string) (GroceryItem, error) {
	var g GroceryItem
	err := h.DB.QueryRow(`SELECT `+groceryCols+` FROM grocery_items WHERE id=?`, id).
		Scan(groceryScan(&g)...)
	if errors.Is(err, sql.ErrNoRows) {
		return GroceryItem{}, ErrNotFound
	}
	return g, err
}

// ListGrocery returns items unchecked-first then by age; checked filter when set.
func (h *Health) ListGrocery(checked *bool) ([]GroceryItem, error) {
	where := []string{"1=1"}
	args := []interface{}{}
	if checked != nil {
		if *checked {
			where = append(where, "checked=1")
		} else {
			where = append(where, "checked=0")
		}
	}
	q := `SELECT ` + groceryCols + ` FROM grocery_items WHERE ` + strings.Join(where, " AND ") +
		` ORDER BY checked ASC, created_at ASC`
	rows, err := h.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []GroceryItem{}
	for rows.Next() {
		var g GroceryItem
		if err := rows.Scan(groceryScan(&g)...); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

type GroceryUpdate struct {
	Name    *string
	Qty     *string
	Unit    **string
	Checked *bool
}

func (h *Health) UpdateGroceryItem(id string, u GroceryUpdate) (GroceryItem, error) {
	cur, err := h.GetGroceryItem(id)
	if err != nil {
		return GroceryItem{}, err
	}
	if u.Name != nil && strings.TrimSpace(*u.Name) != "" {
		cur.Name = *u.Name
	}
	if u.Qty != nil {
		cur.Qty = *u.Qty
	}
	if u.Unit != nil {
		cur.Unit = *u.Unit // ptr-to-nil clears
	}
	if u.Checked != nil {
		cur.Checked = *u.Checked
	}
	cur.UpdatedAt = NowRFC3339()

	var unit interface{}
	if cur.Unit != nil {
		unit = *cur.Unit
	}
	_, err = h.DB.Exec(
		`UPDATE grocery_items SET name=?, qty=?, unit=?, checked=?, updated_at=? WHERE id=?`,
		cur.Name, cur.Qty, unit, cur.Checked, cur.UpdatedAt, id)
	if err != nil {
		return GroceryItem{}, err
	}
	return h.GetGroceryItem(id)
}

func (h *Health) DeleteGroceryItem(id string) error {
	res, err := h.DB.Exec(`DELETE FROM grocery_items WHERE id=?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ClearCheckedGroceries removes ONLY checked=true rows; returns count removed.
func (h *Health) ClearCheckedGroceries() (int64, error) {
	res, err := h.DB.Exec(`DELETE FROM grocery_items WHERE checked=1`)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
