package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"path/filepath"
	"strings"
)

// ---- Receipt attachments (phase 13b) ----
//
// Files live on disk under a configurable directory; the transactions row
// records the relative file name + original name. The store never touches the
// filesystem itself — handlers own I/O and pass relative names in.

var receiptExts = map[string]bool{
	".pdf": true, ".jpg": true, ".jpeg": true, ".png": true, ".webp": true, ".heic": true,
}

// ValidReceiptExt reports whether the uploaded file's extension is allowed.
func ValidReceiptExt(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return receiptExts[ext]
}

// NewReceiptName generates the on-disk name for an attachment.
func NewReceiptName(original string) (string, error) {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf) + strings.ToLower(filepath.Ext(original)), nil
}

func (f *Finance) SetReceipt(txnID, fileName, originalName string) error {
	res, err := f.DB.Exec(`UPDATE transactions SET receipt_file=?, receipt_name=? WHERE id=?`,
		fileName, originalName, txnID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	logChange(f.DB, "transaction", txnID, "update", "receipt attached")
	return nil
}

func (f *Finance) ClearReceipt(txnID string) (string, error) {
	var file string
	err := f.DB.QueryRow(`SELECT COALESCE(receipt_file,'') FROM transactions WHERE id=?`, txnID).Scan(&file)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	if _, err := f.DB.Exec(`UPDATE transactions SET receipt_file=NULL, receipt_name=NULL WHERE id=?`, txnID); err != nil {
		return "", err
	}
	logChange(f.DB, "transaction", txnID, "update", "receipt removed")
	return file, nil
}
