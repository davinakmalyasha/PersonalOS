package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/davinakmalyasha/PersonalOS/services/api/internal/domain/finance"
)

type Transaction struct {
	ID             string   `json:"id"`
	AccountID      string   `json:"account_id"`
	AmountMinor    int64    `json:"amount_minor"`
	Currency       string   `json:"currency"`
	Date           string   `json:"date"`
	Merchant       string   `json:"merchant"`
	RawDescription string   `json:"raw_description"`
	CategoryID     *string  `json:"category_id"`
	CategoryName   *string  `json:"category_name,omitempty"`
	Tags           []string `json:"tags"`
	Notes          string   `json:"notes"`
	IsTransfer     bool     `json:"is_transfer"`
	ReceiptFile    *string  `json:"-"` // relative name on disk
	ReceiptName    *string  `json:"receipt_name,omitempty"`
	CreatedAt      string   `json:"created_at"`

	tagsRaw string
}

type TxnFilter struct {
	AccountID  string
	CategoryID string
	Uncat      bool
	From, To   string
	Min, Max   *int64
	Tag        string
	Q          string
	Page       int
	PageSize   int
}

func (f *Finance) CreateTransaction(accountID string, amount int64, date, merchant, rawDesc, notes string, categoryID *string, tags []string, currency string) (Transaction, error) {
	if _, err := f.GetAccount(accountID); err != nil {
		return Transaction{}, err
	}
	if currency == "" {
		currency = f.BaseCurrency()
	}
	if categoryID != nil && *categoryID != "" {
		if _, err := f.GetCategory(*categoryID); err != nil {
			return Transaction{}, err
		}
	} else {
		categoryID = nil
	}
	if rawDesc == "" && merchant != "" {
		rawDesc = merchant
	}
	if merchant == "" {
		merchant = firstN(rawDesc, 60)
	}
	merchant = f.ApplyAlias(merchant)
	tagJSON := joinTags(normalizeTagList(tags))
	t := Transaction{
		ID: NewID(), AccountID: accountID, AmountMinor: amount, Currency: currency,
		Date: date, Merchant: merchant, RawDescription: rawDesc,
		CategoryID: categoryID, Notes: notes, CreatedAt: NowRFC3339(),
	}
	hash := finance.DescriptionHash(rawDesc)
	_, err := f.DB.Exec(`
		INSERT INTO transactions (id,account_id,amount,currency,date,merchant,raw_description,category_id,hash,notes,tags,created_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
		t.ID, t.AccountID, t.AmountMinor, t.Currency, t.Date, t.Merchant, t.RawDescription,
		t.CategoryID, hash, t.Notes, tagJSON, t.CreatedAt)
	if isUniqueErr(err) {
		return Transaction{}, ErrConflict // duplicate natural key (date, amount, hash)
	}
	if err != nil {
		return Transaction{}, err
	}
	f.pairDetectFor(t.ID)
	logChange(f.DB, "transaction", t.ID, "create", t.Merchant)
	return f.GetTransaction(t.ID)
}

func txnScan(dest *Transaction) []interface{} {
	return []interface{}{&dest.ID, &dest.AccountID, &dest.AmountMinor, &dest.Currency,
		&dest.Date, &dest.Merchant, &dest.RawDescription, &dest.CategoryID,
		&dest.CategoryName, &dest.tagsRaw, &dest.Notes, &dest.IsTransfer,
		&dest.ReceiptFile, &dest.ReceiptName, &dest.CreatedAt}
}

const txnSelect = `
	SELECT t.id,t.account_id,t.amount,t.currency,t.date,t.merchant,t.raw_description,
	       t.category_id,c.name,t.tags,t.notes,t.is_transfer,t.receipt_file,t.receipt_name,t.created_at
	FROM transactions t LEFT JOIN categories c ON c.id=t.category_id`

func (f *Finance) GetTransaction(id string) (Transaction, error) {
	var t Transaction
	err := f.DB.QueryRow(txnSelect+` WHERE t.id=?`, id).Scan(txnScan(&t)...)
	if errors.Is(err, sql.ErrNoRows) {
		return Transaction{}, ErrNotFound
	}
	if err != nil {
		return Transaction{}, err
	}
	t.Tags = splitTags(t.tagsRaw)
	return t, err
}

func (f *Finance) buildTxnWhere(fl TxnFilter) (string, []interface{}) {
	where := []string{"1=1"}
	args := []interface{}{}
	if fl.AccountID != "" {
		where = append(where, "t.account_id=?")
		args = append(args, fl.AccountID)
	}
	if fl.CategoryID != "" {
		where = append(where, "t.category_id=?")
		args = append(args, fl.CategoryID)
	}
	if fl.Uncat {
		where = append(where, "t.category_id IS NULL")
	}
	if fl.From != "" {
		where = append(where, "t.date>=?")
		args = append(args, fl.From)
	}
	if fl.To != "" {
		where = append(where, "t.date<=?")
		args = append(args, fl.To+"~") // '~' sorts after '-' so YYYY-MM prefix matches whole month
	}
	if fl.Min != nil {
		where = append(where, "t.amount>=?")
		args = append(args, *fl.Min)
	}
	if fl.Max != nil {
		where = append(where, "t.amount<=?")
		args = append(args, *fl.Max)
	}
	if fl.Q != "" {
		where = append(where, "(LOWER(t.merchant) LIKE ? OR LOWER(t.raw_description) LIKE ?)")
		pat := "%" + strings.ToLower(fl.Q) + "%"
		args = append(args, pat, pat)
	}
	if fl.Tag != "" {
		where = append(where, "t.tags LIKE ?")
		args = append(args, `%"`+fl.Tag+`"%`)
	}
	return strings.Join(where, " AND "), args
}

// ListTransactions returns one page plus total count for the same filter.
func (f *Finance) ListTransactions(fl TxnFilter) ([]Transaction, int, error) {
	if fl.Page < 1 {
		fl.Page = 1
	}
	if fl.PageSize < 1 || fl.PageSize > 100 {
		fl.PageSize = 20
	}
	whereSQL, args := f.buildTxnWhere(fl)

	var total int
	if err := f.DB.QueryRow(`SELECT COUNT(*) FROM transactions t WHERE `+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	q := txnSelect + ` WHERE ` + whereSQL + ` ORDER BY t.date DESC, t.created_at DESC LIMIT ? OFFSET ?`
	pageArgs := append(append([]interface{}{}, args...), fl.PageSize, (fl.Page-1)*fl.PageSize)
	rows, err := f.DB.Query(q, pageArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []Transaction{}
	for rows.Next() {
		var t Transaction
		if err := rows.Scan(txnScan(&t)...); err != nil {
			return nil, 0, err
		}
		t.Tags = splitTags(t.tagsRaw)
		out = append(out, t)
	}
	return out, total, rows.Err()
}

func (f *Finance) UpdateTransaction(id string, upd TransactionUpdate) (Transaction, error) {
	cur, err := f.GetTransaction(id)
	if err != nil {
		return Transaction{}, err
	}
	if upd.Date != nil {
		cur.Date = *upd.Date
	}
	if upd.Amount != nil {
		cur.AmountMinor = *upd.Amount
	}
	if upd.Merchant != nil {
		cur.Merchant = *upd.Merchant
	}
	if upd.RawDescription != nil {
		cur.RawDescription = *upd.RawDescription
	}
	if upd.Notes != nil {
		cur.Notes = *upd.Notes
	}
	if upd.CategoryID != nil {
		if *upd.CategoryID == "" {
			cur.CategoryID = nil
		} else {
			if _, err := f.GetCategory(*upd.CategoryID); err != nil {
				return Transaction{}, err
			}
			cur.CategoryID = upd.CategoryID
		}
	}
	if upd.Tags != nil {
		cur.Tags = normalizeTagList(*upd.Tags)
	}
	hash := finance.DescriptionHash(cur.RawDescription)
	_, err = f.DB.Exec(`
		UPDATE transactions SET date=?, amount=?, merchant=?, raw_description=?, notes=?, category_id=?, tags=?, hash=?
		WHERE id=?`,
		cur.Date, cur.AmountMinor, cur.Merchant, cur.RawDescription, cur.Notes, cur.CategoryID, joinTags(cur.Tags), hash, id)
	if isUniqueErr(err) {
		return Transaction{}, ErrConflict
	}
	if err != nil {
		return Transaction{}, err
	}
	logChange(f.DB, "transaction", id, "update", cur.Merchant)
	return f.GetTransaction(id)
}

type TransactionUpdate struct {
	Date           *string
	Amount         *int64
	Merchant       *string
	RawDescription *string
	Notes          *string
	CategoryID     *string  // empty string clears
	Tags           *[]string
}

func (f *Finance) DeleteTransaction(id string) error {
	cur, err := f.GetTransaction(id)
	if err != nil {
		return err
	}
	res, err := f.DB.Exec(`DELETE FROM transactions WHERE id=?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	logChange(f.DB, "transaction", id, "delete", cur.Merchant)
	return nil
}

// ExistingKeys returns dedupe keys for an account (used by import).
func (f *Finance) ExistingKeys(accountID string) (map[string]struct{}, error) {
	rows, err := f.DB.Query(`SELECT date, amount, hash FROM transactions WHERE account_id=?`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]struct{})
	for rows.Next() {
		var d string
		var a int64
		var h string
		if err := rows.Scan(&d, &a, &h); err != nil {
			return nil, err
		}
		out[finance.DedupeKey(d, a, h)] = struct{}{}
	}
	return out, rows.Err()
}

// ImportTransactions batch-inserts drafts in one transaction. Rows violating
// the natural key are silently ignored (race-safe idempotence); the number of
// actually inserted rows is returned.
func (f *Finance) ImportTransactions(accountID, currency string, drafts []finance.Draft) (int64, error) {
	tx, err := f.DB.Begin()
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.Prepare(`
		INSERT OR IGNORE INTO transactions (id,account_id,amount,currency,date,merchant,raw_description,category_id,hash,notes,created_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,'')`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	var inserted int64
	var newIDs []string
	created := NowRFC3339()
	for _, d := range drafts {
		var cat interface{}
		if d.CategoryID != "" {
			cat = d.CategoryID
		}
		id := NewID()
		res, err := stmt.Exec(id, accountID, d.Amount, currency, d.Date, f.ApplyAlias(d.Merchant), d.RawDesc, cat, d.Hash, created)
		if err != nil {
			return inserted, err
		}
		n, _ := res.RowsAffected()
		inserted += n
		if n > 0 {
			newIDs = append(newIDs, id)
		}
	}
	if err := tx.Commit(); err != nil {
		return inserted, err
	}
	if inserted > 0 {
		logChange(f.DB, "transaction", accountID, "create",
			fmt.Sprintf("imported %d transactions", inserted))
	}
	// Transfer pairing after commit (best-effort).
	for _, id := range newIDs {
		f.pairDetectFor(id)
	}
	return inserted, nil
}

func firstN(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
