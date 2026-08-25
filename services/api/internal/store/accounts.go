package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
)

type Account struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Currency  string `json:"currency"`
	CreatedAt string `json:"created_at"`
}

type AccountWithBalance struct {
	Account
	BalanceMinor int64 `json:"balance_minor"`
}

func (f *Finance) CreateAccount(name, typ, currency string) (Account, error) {
	if currency == "" {
		currency = "IDR"
	}
	a := Account{ID: NewID(), Name: name, Type: typ, Currency: currency, CreatedAt: NowRFC3339()}
	_, err := f.DB.Exec(`INSERT INTO accounts (id,name,type,currency,created_at) VALUES (?,?,?,?,?)`,
		a.ID, a.Name, a.Type, a.Currency, a.CreatedAt)
	if isUniqueErr(err) {
		return Account{}, ErrConflict
	}
	if err != nil {
		return Account{}, err
	}
	logChange(f.DB, "account", a.ID, "create", a.Name)
	return a, nil
}

func (f *Finance) ListAccounts() ([]AccountWithBalance, error) {
	rows, err := f.DB.Query(`
		SELECT a.id,a.name,a.type,a.currency,a.created_at,
		       COALESCE((SELECT SUM(amount) FROM transactions t WHERE t.account_id=a.id),0)
		FROM accounts a ORDER BY a.created_at, a.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AccountWithBalance
	for rows.Next() {
		var a AccountWithBalance
		if err := rows.Scan(&a.ID, &a.Name, &a.Type, &a.Currency, &a.CreatedAt, &a.BalanceMinor); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (f *Finance) GetAccount(id string) (AccountWithBalance, error) {
	var a AccountWithBalance
	err := f.DB.QueryRow(`
		SELECT a.id,a.name,a.type,a.currency,a.created_at,
		       COALESCE((SELECT SUM(amount) FROM transactions t WHERE t.account_id=a.id),0)
		FROM accounts a WHERE a.id=?`, id).
		Scan(&a.ID, &a.Name, &a.Type, &a.Currency, &a.CreatedAt, &a.BalanceMinor)
	if errors.Is(err, sql.ErrNoRows) {
		return AccountWithBalance{}, ErrNotFound
	}
	return a, err
}

func (f *Finance) UpdateAccount(id string, name, typ *string) (AccountWithBalance, error) {
	cur, err := f.GetAccount(id)
	if err != nil {
		return AccountWithBalance{}, err
	}
	if name != nil {
		cur.Name = *name
	}
	if typ != nil {
		cur.Type = *typ
	}
	_, err = f.DB.Exec(`UPDATE accounts SET name=?, type=? WHERE id=?`, cur.Name, cur.Type, id)
	if isUniqueErr(err) {
		return AccountWithBalance{}, ErrConflict
	}
	if err != nil {
		return AccountWithBalance{}, err
	}
	return f.GetAccount(id)
}

func (f *Finance) DeleteAccount(id string) error {
	var n int
	if err := f.DB.QueryRow(`SELECT COUNT(*) FROM transactions WHERE account_id=?`, id).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return ErrConflict // reassign or delete transactions first
	}
	res, err := f.DB.Exec(`DELETE FROM accounts WHERE id=?`, id)
	if err != nil {
		return err
	}
	if n2, _ := res.RowsAffected(); n2 == 0 {
		return ErrNotFound
	}
	return nil
}

func isUniqueErr(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}

// ---- Import profile (per-account CSV mapping persistence) ----

// ImportProfile is the persisted column mapping + date layout for an account.
type ImportProfile struct {
	Mapping    map[string]int `json:"mapping"`
	DateFormat string         `json:"date_format"`
}

// GetImportProfile returns the saved profile, or nil when none exists.
func (f *Finance) GetImportProfile(accountID string) (*ImportProfile, error) {
	var raw string
	err := f.DB.QueryRow(`SELECT settings FROM accounts WHERE id=?`, accountID).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var settings struct {
		ImportProfile *ImportProfile `json:"import_profile"`
	}
	if err := json.Unmarshal([]byte(raw), &settings); err != nil {
		return nil, nil // corrupt settings â€” treat as absent
	}
	return settings.ImportProfile, nil
}

// SaveImportProfile merges the profile into the account's settings JSON.
func (f *Finance) SaveImportProfile(accountID string, p ImportProfile) error {
	var raw string
	err := f.DB.QueryRow(`SELECT settings FROM accounts WHERE id=?`, accountID).Scan(&raw)
	if err != nil {
		return err
	}
	var settings map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &settings); err != nil || settings == nil {
		settings = map[string]interface{}{}
	}
	settings["import_profile"] = p
	b, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	_, err = f.DB.Exec(`UPDATE accounts SET settings=? WHERE id=?`, string(b), accountID)
	return err
}
