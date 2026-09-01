package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSaveDocument menguji SaveDocument secara hermetik -- fungsi ini
// TIDAK menampilkan dialog apa pun (beda dari OpenDocument/SaveDocumentAs
// yang butuh window OS sungguhan, lihat ROADMAP.md), murni menulis
// content ke path yang sudah diketahui, jadi bisa dites langsung dengan
// &App{} kosong tanpa context Wails sungguhan.
func TestSaveDocument(t *testing.T) {
	t.Run("empty path rejected before touching disk", func(t *testing.T) {
		a := &App{}
		err := a.SaveDocument("", "isi apa saja")
		if err == nil {
			t.Fatal("SaveDocument(\"\", ...) error = nil, ingin error")
		}
		if !strings.Contains(err.Error(), "SaveDocumentAs") {
			t.Errorf("SaveDocument() error = %q, ingin menyarankan SaveDocumentAs", err.Error())
		}
	})

	t.Run("writes content to the given path", func(t *testing.T) {
		a := &App{}
		path := filepath.Join(t.TempDir(), "dokumen.tex")
		content := "\\documentclass{article}\n\\begin{document}\nhalo\n\\end{document}\n"
		if err := a.SaveDocument(path, content); err != nil {
			t.Fatalf("SaveDocument() error = %v, ingin nil", err)
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("baca file hasil SaveDocument: %v", err)
		}
		if string(got) != content {
			t.Errorf("isi file = %q, ingin %q", string(got), content)
		}
	})

	t.Run("overwrites existing file", func(t *testing.T) {
		a := &App{}
		path := filepath.Join(t.TempDir(), "dokumen.tex")
		if err := os.WriteFile(path, []byte("isi lama"), 0o644); err != nil {
			t.Fatalf("tulis isi lama: %v", err)
		}
		if err := a.SaveDocument(path, "isi baru"); err != nil {
			t.Fatalf("SaveDocument() error = %v, ingin nil", err)
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("baca file: %v", err)
		}
		if string(got) != "isi baru" {
			t.Errorf("isi file = %q, ingin \"isi baru\" (SaveDocument seharusnya menimpa, bukan menambah)", string(got))
		}
	})

	t.Run("no leftover temp file after a successful save", func(t *testing.T) {
		a := &App{}
		dir := t.TempDir()
		path := filepath.Join(dir, "dokumen.tex")
		if err := a.SaveDocument(path, "isi"); err != nil {
			t.Fatalf("SaveDocument() error = %v, ingin nil", err)
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("baca direktori: %v", err)
		}
		if len(entries) != 1 || entries[0].Name() != "dokumen.tex" {
			names := make([]string, len(entries))
			for i, e := range entries {
				names[i] = e.Name()
			}
			t.Errorf("isi direktori setelah SaveDocument = %v, ingin cuma [dokumen.tex] -- file sementara atomicWriteFile seharusnya sudah di-rename, bukan tertinggal", names)
		}
	})
}

// TestAtomicWriteFile menguji helper penulisan atomic secara langsung
// (dipakai SaveDocument dan SaveDocumentAs) -- lihat komentarnya di app.go
// untuk kenapa ini dipakai alih-alih os.WriteFile langsung.
func TestAtomicWriteFile(t *testing.T) {
	t.Run("creates a new file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "baru.tex")
		if err := atomicWriteFile(path, []byte("halo"), 0o644); err != nil {
			t.Fatalf("atomicWriteFile() error = %v, ingin nil", err)
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("baca file: %v", err)
		}
		if string(got) != "halo" {
			t.Errorf("isi = %q, ingin \"halo\"", string(got))
		}
	})

	t.Run("replaces existing file content fully, not appended", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "ada.tex")
		if err := os.WriteFile(path, []byte("isi lama yang jauh lebih panjang dari isi baru"), 0o644); err != nil {
			t.Fatalf("tulis isi lama: %v", err)
		}
		if err := atomicWriteFile(path, []byte("baru"), 0o644); err != nil {
			t.Fatalf("atomicWriteFile() error = %v, ingin nil", err)
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("baca file: %v", err)
		}
		if string(got) != "baru" {
			t.Errorf("isi = %q, ingin \"baru\" (bukan gabungan dengan isi lama)", string(got))
		}
	})

	t.Run("fails clearly when directory does not exist, without leaving a temp file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "direktori-tidak-ada", "dokumen.tex")
		if err := atomicWriteFile(path, []byte("isi"), 0o644); err == nil {
			t.Error("atomicWriteFile() error = nil, ingin error (direktori tujuan tidak ada)")
		}
	})
}

// TestExportPDF_EmptyRejectedBeforeDialog menguji satu-satunya bagian
// ExportPDF yang bisa dites tanpa window OS sungguhan: penolakan PDF
// kosong terjadi SEBELUM wailsruntime.SaveFileDialog dipanggil (yang
// butuh a.ctx terisi) -- kalau urutannya kebalik, panggilan ini akan
// panic (a.ctx nil pada &App{} kosong) alih-alih mengembalikan error yang
// jelas.
func TestExportPDF_EmptyRejectedBeforeDialog(t *testing.T) {
	a := &App{}
	_, err := a.ExportPDF(nil)
	if err == nil {
		t.Fatal("ExportPDF(nil) error = nil, ingin error")
	}
	if !strings.Contains(err.Error(), "compile") {
		t.Errorf("ExportPDF() error = %q, ingin menyarankan compile dulu", err.Error())
	}
}
