package server

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/davinakmalyasha/PersonalOS/services/api/internal/store"
	"github.com/go-chi/chi/v5"
)

// ---- Receipt attachments (phase 13b) ----

const maxReceiptBytes = 10 << 20 // 10 MiB

func (s *Server) receiptPath(fileName string) string {
	return filepath.Join(s.attachmentsDir, fileName)
}

// POST /transactions/{id}/receipt  (multipart, field "file")
func (s *Server) handleUploadReceipt(w http.ResponseWriter, r *http.Request) {
	txnID := chi.URLParam(r, "id")
	if _, err := s.finance.GetTransaction(txnID); err != nil {
		fail(w, http.StatusNotFound, "transaction not found")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxReceiptBytes)
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		fail(w, http.StatusBadRequest, "file too large (max 10 MiB) or bad multipart", fieldError{"file", err.Error()})
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		fail(w, http.StatusBadRequest, "missing file field", fieldError{"file", "multipart field 'file' required"})
		return
	}
	defer file.Close()

	original := filepath.Base(header.Filename)
	if !store.ValidReceiptExt(original) {
		fail(w, http.StatusBadRequest, "unsupported file type (pdf/jpg/jpeg/png/webp/heic only)", fieldError{"file", original})
		return
	}
	name, err := store.NewReceiptName(original)
	if err != nil {
		fail(w, http.StatusInternalServerError, "could not generate file name")
		return
	}
	if err := os.MkdirAll(s.attachmentsDir, 0o755); err != nil {
		fail(w, http.StatusInternalServerError, "attachments dir unavailable")
		return
	}
	dst, err := os.Create(s.receiptPath(name))
	if err != nil {
		fail(w, http.StatusInternalServerError, "could not save file")
		return
	}
	defer dst.Close()
	if _, err := io.Copy(dst, file); err != nil {
		_ = os.Remove(s.receiptPath(name))
		fail(w, http.StatusInternalServerError, "could not write file")
		return
	}
	if err := s.finance.SetReceipt(txnID, name, original); err != nil {
		_ = os.Remove(s.receiptPath(name))
		fail(w, http.StatusInternalServerError, "could not attach receipt")
		return
	}
	txn, _ := s.finance.GetTransaction(txnID)
	writeJSON(w, http.StatusOK, txn)
}

// GET /transactions/{id}/receipt — serves the stored file.
func (s *Server) handleGetReceipt(w http.ResponseWriter, r *http.Request) {
	txnID := chi.URLParam(r, "id")
	txn, err := s.finance.GetTransaction(txnID)
	if err != nil || txn.ReceiptFile == nil || *txn.ReceiptFile == "" {
		fail(w, http.StatusNotFound, "no receipt attached")
		return
	}
	path := s.receiptPath(*txn.ReceiptFile)
	f, err := os.Open(path)
	if err != nil {
		fail(w, http.StatusNotFound, "receipt file missing on disk")
		return
	}
	defer f.Close()
	if txn.ReceiptName != nil {
		w.Header().Set("Content-Disposition",
			fmt.Sprintf(`inline; filename="%s"`, strings.ReplaceAll(*txn.ReceiptName, `"`, "")))
	}
	ext := strings.ToLower(filepath.Ext(*txn.ReceiptFile))
	if ct := mime.TypeByExtension(ext); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	http.ServeContent(w, r, *txn.ReceiptFile, modTimeOf(path), f)
}

func modTimeOf(path string) time.Time {
	if fi, err := os.Stat(path); err == nil {
		return fi.ModTime()
	}
	return time.Time{}
}

// DELETE /transactions/{id}/receipt
func (s *Server) handleDeleteReceipt(w http.ResponseWriter, r *http.Request) {
	txnID := chi.URLParam(r, "id")
	file, err := s.finance.ClearReceipt(txnID)
	if err != nil {
		fail(w, http.StatusNotFound, "transaction or receipt not found")
		return
	}
	if file != "" {
		_ = os.Remove(s.receiptPath(file))
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
}
