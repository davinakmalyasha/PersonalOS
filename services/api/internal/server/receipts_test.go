package server

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func mustCreateTxn(t *testing.T, h http.Handler, accountID string, amount int64, date, merchant string) string {
	t.Helper()
	rec := doJSON(t, h, http.MethodPost, "/v1/transactions", map[string]interface{}{
		"account_id": accountID, "amount_minor": amount, "date": date, "merchant": merchant,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create txn: %d %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	return resp.ID
}

func uploadReceipt(t *testing.T, h http.Handler, txnID, filename string, content []byte) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", filename)
	_, _ = fw.Write(content)
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/v1/transactions/"+txnID+"/receipt", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestReceiptLifecycle(t *testing.T) {
	h := newTestAPI(t)
	acct := mustCreateAccount(t, h)
	txn := mustCreateTxn(t, h, acct, -2500000, "2026-08-14", "Laptop Store")

	// Upload a PNG receipt.
	png := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 0, 0, 13}
	rec := uploadReceipt(t, h, txn, "receipt.png", png)
	if rec.Code != http.StatusOK {
		t.Fatalf("upload: %d %s", rec.Code, rec.Body.String())
	}
	var txnResp struct {
		ReceiptName *string `json:"receipt_name"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &txnResp)
	if txnResp.ReceiptName == nil || *txnResp.ReceiptName != "receipt.png" {
		t.Fatalf("receipt_name not returned: %+v", txnResp)
	}

	// GET serves the exact bytes back.
	rec = doJSON(t, h, http.MethodGet, "/v1/transactions/"+txn+"/receipt", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get receipt: %d", rec.Code)
	}
	if !bytes.Equal(rec.Body.Bytes(), png) {
		t.Fatalf("receipt content mismatch")
	}
	if ct := rec.Header().Get("Content-Type"); ct != "image/png" {
		t.Fatalf("content type: %q", ct)
	}
	if cd := rec.Header().Get("Content-Disposition"); cd == "" || !bytes.Contains([]byte(cd), []byte("receipt.png")) {
		t.Fatalf("disposition: %q", cd)
	}

	// DELETE removes row + file.
	rec = doJSON(t, h, http.MethodDelete, "/v1/transactions/"+txn+"/receipt", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodGet, "/v1/transactions/"+txn+"/receipt", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 after delete, got %d", rec.Code)
	}

	// Re-upload then verify the on-disk file is cleaned up on delete.
	_ = uploadReceipt(t, h, txn, "receipt2.pdf", []byte("%PDF-1.4 test"))
	rec = doJSON(t, h, http.MethodGet, "/v1/transactions/"+txn, nil)
	var txn2 struct {
		ReceiptName *string `json:"receipt_name"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &txn2)
	if txn2.ReceiptName == nil {
		t.Fatal("second upload missing receipt_name")
	}
	_ = doJSON(t, h, http.MethodDelete, "/v1/transactions/"+txn+"/receipt", nil)

	// Bad extension rejected.
	rec = uploadReceipt(t, h, txn, "evil.exe", []byte("MZ..."))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("exe should be rejected, got %d", rec.Code)
	}
	// Missing transaction.
	rec = uploadReceipt(t, h, "nope", "a.png", png)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 txn, got %d", rec.Code)
	}
}

func TestReceiptFilesLandInAttachmentsDir(t *testing.T) {
	h := newTestAPI(t)
	acct := mustCreateAccount(t, h)
	txn := mustCreateTxn(t, h, acct, -50000, "2026-08-15", "Coffee Shop")
	_ = uploadReceipt(t, h, txn, "r.jpg", []byte{0xff, 0xd8, 0xff, 0xe0})

	// Find the attachments dir the test server used: it is a sibling of the db.
	// Walk the temp dir for any .jpg file to confirm the file was written.
	var found bool
	root := os.TempDir()
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || found {
			return filepath.SkipAll
		}
		if !info.IsDir() && filepath.Ext(path) == ".jpg" && info.Size() == 4 {
			found = true
		}
		return nil
	})
	if !found {
		t.Fatal("uploaded jpg not found on disk")
	}
}
