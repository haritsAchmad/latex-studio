package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"unicode/utf8"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"latex-studio/internal/tectonic"
)

// texFileFilters adalah filter file yang dipakai dialog Open dan Save --
// diekstrak jadi satu nilai bersama supaya keduanya konsisten.
var texFileFilters = []wailsruntime.FileFilter{
	{DisplayName: "Dokumen LaTeX (*.tex)", Pattern: "*.tex"},
	{DisplayName: "Semua file (*.*)", Pattern: "*.*"},
}

// App adalah struct tunggal yang method public-nya dibind ke frontend lewat
// Wails (lihat main.go, Bind: []interface{}{app}) -- method dengan huruf
// besar di sini otomatis muncul sebagai fungsi async di
// frontend/wailsjs/go/main/App setelah `wails dev`/`wails build`.
type App struct {
	ctx context.Context

	// mu menyerialkan EnsureEngine dan Compile -- keduanya membaca/menulis
	// engineBin dan workDir, dan Compile menjalankan Tectonic di workDir
	// yang SAMA untuk setiap panggilan (lihat komentar workDir). Tanpa
	// mutex ini, dua panggilan Compile paralel dari frontend (mis. race
	// antara auto-recompile debounce dan trigger manual) bisa saling
	// menimpa main.tex/main.pdf milik satu sama lain di tengah proses
	// Tectonic berjalan, menghasilkan PDF yang salah/korup untuk salah
	// satu atau kedua panggilan.
	mu sync.Mutex

	// fileMu menyerialkan OpenDocument/SaveDocument/SaveDocumentAs --
	// TERPISAH dari mu (operasi file tidak menyentuh engineBin/workDir,
	// jadi sengaja tidak dibuat menunggu Compile yang mungkin sedang
	// berjalan sampai beberapa menit, lihat komentar OpenDocument). Tanpa
	// mutex INI, dua panggilan file (mis. Simpan dipicu dua kali
	// berdekatan lewat tombol dan Ctrl+S) bisa saling menimpa penulisan
	// ke path yang sama, atau membuat hasil Go-side selesai dengan urutan
	// yang tidak terduga -- ditemukan lewat code review sebelum commit.
	// Frontend (frontend/src/main.js, fileOperationInProgress) juga
	// menyerialkan pemicunya sendiri supaya state currentPath/dirty di
	// SISI FRONTEND tidak kacau kalau dua operasi diizinkan tumpang
	// tindih -- dua lapis ini melindungi hal yang berbeda (disk vs. state
	// UI), bukan duplikat.
	fileMu sync.Mutex

	// workDir adalah direktori kerja SATU sesi aplikasi (dibuat di startup,
	// dihapus di shutdown) tempat main.tex/main.pdf ditulis ulang setiap
	// kompilasi. Sengaja satu direktori dipakai ulang, bukan direktori baru
	// per kompilasi -- dokumen yang sedang diedit cuma satu pada increment
	// ini (belum ada multi-tab/multi-file, lihat ROADMAP.md), jadi tidak
	// ada risiko tabrakan nama.
	//
	// KOSONG kalau dan HANYA kalau startupErr != nil -- lihat startup di
	// bawah. TIDAK PERNAH diisi os.TempDir() (root direktori temp OS)
	// sebagai fallback: shutdown menjalankan os.RemoveAll(workDir), dan
	// menghapus os.TempDir() itu sendiri akan menyapu bersih file
	// sementara milik proses/aplikasi LAIN yang kebetulan berbagi direktori
	// temp yang sama -- insiden yang ditemukan lewat code review sebelum
	// commit pertama, bukan spekulatif.
	workDir string
	// startupErr diisi kalau os.MkdirTemp gagal membuat direktori kerja
	// UNIK di startup -- EnsureEngine dan Compile menolak jalan sama sekali
	// selama ini terisi (fail closed), bukan diam-diam memakai direktori
	// yang tidak aman.
	startupErr error

	// engineBin di-cache setelah EnsureEngine sukses sekali supaya
	// kompilasi berikutnya tidak perlu stat ulang CacheDir tiap saat --
	// murni optimisasi, EnsureEngine sendiri sudah idempotent.
	engineBin string
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup dipanggil Wails sekali saat aplikasi mulai (lihat main.go,
// OnStartup) -- context di sini WAJIB dipakai untuk semua pemanggilan
// wailsruntime (event, dialog, dst.), bukan context.Background(), supaya
// Wails bisa mengaitkannya ke window yang benar.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	dir, err := os.MkdirTemp("", "latex-studio-")
	if err != nil {
		// TIDAK fallback ke os.TempDir() -- lihat komentar field workDir
		// untuk kenapa itu berbahaya (shutdown menghapusnya lewat
		// RemoveAll). startupErr membuat EnsureEngine/Compile menolak
		// jalan sama sekali sampai operator restart aplikasi di kondisi
		// yang memungkinkan pembuatan direktori sementara (disk penuh,
		// permission, dst. -- kegagalan MkdirTemp yang genuinely jarang
		// tapi harus ditangani eksplisit, bukan diasumsikan tidak pernah
		// terjadi).
		a.startupErr = fmt.Errorf("buat direktori kerja sementara: %w", err)
		return
	}
	a.workDir = dir
}

// shutdown membersihkan workDir saat aplikasi ditutup -- dokumen yang
// sedang dikerjakan TIDAK disimpan di sini sama sekali (lihat ROADMAP.md
// bagian "belum ada save/open file"), jadi tidak ada isi berharga yang
// hilang, cuma file .tex/.pdf sementara hasil kompilasi terakhir. Cuma
// menghapus kalau workDir terisi (lihat komentar field workDir -- kosong
// berarti startup gagal membuatnya, tidak ada apa pun untuk dihapus).
func (a *App) shutdown(ctx context.Context) {
	if a.workDir != "" {
		os.RemoveAll(a.workDir)
	}
}

// EnsureEngine memicu bootstrap Tectonic (unduh+verifikasi checksum kalau
// belum ada secara lokal, lihat internal/tectonic) dan melaporkan
// progresnya ke frontend lewat event Wails "engine:progress" -- dipanggil
// frontend sekali saat aplikasi baru dibuka, SEBELUM compile pertama,
// supaya operator lihat status "mengunduh Tectonic..." alih-alih UI
// diam/hang di percobaan compile pertama yang bisa makan waktu lama
// (unduhan awal + paket TeX yang belum ter-cache, lihat
// internal/tectonic/tectonic.go).
func (a *App) EnsureEngine() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.startupErr != nil {
		return a.startupErr
	}
	bin, err := tectonic.Ensure(a.ctx, func(stage string) {
		wailsruntime.EventsEmit(a.ctx, "engine:progress", stage)
	})
	if err != nil {
		return err
	}
	a.engineBin = bin
	return nil
}

// ensureEngineLocked adalah isi EnsureEngine tanpa mengambil a.mu --
// dipakai Compile saat engineBin belum siap SUPAYA tidak deadlock
// memanggil EnsureEngine (yang juga mengambil a.mu) dari dalam Compile
// yang sudah memegang lock yang sama.
func (a *App) ensureEngineLocked() error {
	if a.startupErr != nil {
		return a.startupErr
	}
	bin, err := tectonic.Ensure(a.ctx, func(stage string) {
		wailsruntime.EventsEmit(a.ctx, "engine:progress", stage)
	})
	if err != nil {
		return err
	}
	a.engineBin = bin
	return nil
}

// Compile menjalankan Tectonic terhadap source (isi lengkap dokumen .tex
// dari editor) dan mengembalikan PDF hasilnya beserta log mentah Tectonic.
// Error Go (gagal tulis file, Tectonic tidak ditemukan, dst.) BEDA dari
// kompilasi LaTeX yang gagal karena salah syntax -- yang kedua tetap
// dikembalikan sebagai Result sukses secara Go (err == nil) dengan
// Result.Ok == false dan Result.Log berisi pesan error LaTeX-nya, supaya
// frontend bisa menampilkan log itu ke operator, bukan cuma "compile
// gagal" generik tanpa detail.
func (a *App) Compile(source string) (tectonic.Result, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.startupErr != nil {
		return tectonic.Result{}, a.startupErr
	}
	if a.engineBin == "" {
		if err := a.ensureEngineLocked(); err != nil {
			return tectonic.Result{}, err
		}
	}
	return tectonic.Compile(a.ctx, a.engineBin, source, a.workDir)
}

// OpenResult adalah hasil OpenDocument. Cancelled true (Path/Content
// kosong, Err Go tetap nil) kalau operator menutup dialog tanpa memilih
// file -- frontend membedakan ini dari kegagalan sungguhan (file tidak
// bisa dibaca, dst., yang jadi Go error biasa) supaya "batal buka file"
// tidak menampilkan pesan error yang membingungkan.
type OpenResult struct {
	Path      string `json:"path"`
	Content   string `json:"content"`
	Cancelled bool   `json:"cancelled"`
}

// maxOpenFileBytes membatasi ukuran file yang mau dibaca OpenDocument --
// 50 MiB jauh lebih besar dari dokumen .tex wajar mana pun (biasanya
// puluhan/ratusan KB), tapi mencegah file yang salah pilih/rusak/bukan
// dokumen teks sama sekali membebani memori editor tanpa batas.
const maxOpenFileBytes = 50 * 1024 * 1024

// OpenDocument menampilkan dialog file native, membaca file .tex yang
// dipilih operator, dan mengembalikan isinya. Dilindungi fileMu (lihat
// komentar field-nya) supaya tidak tumpang tindih dengan
// SaveDocument/SaveDocumentAs -- TETAP terpisah dari mu (Compile/
// EnsureEngine), jadi membuka file tidak perlu menunggu kompilasi yang
// mungkin sedang berjalan (bisa sampai beberapa menit kalau ada paket TeX
// baru yang diunduh, lihat compileTimeout).
//
// Menolak file yang lebih besar dari maxOpenFileBytes atau yang isinya
// BUKAN UTF-8 valid -- yang kedua bukan sekadar validasi kosmetik: kalau
// byte tidak valid UTF-8 diteruskan apa adanya sebagai Go string lalu
// di-marshal ke JSON untuk dikirim ke frontend, encoding/json diam-diam
// MENGGANTI byte yang tidak valid dengan U+FFFD (replacement character)
// supaya hasilnya tetap UTF-8 valid -- operator yang membuka file
// berenkoding lain (mis. Latin-1) lalu menyimpannya kembali lewat
// SaveDocument tanpa sadar akan menimpa file ASLI dengan versi yang sudah
// rusak. Ditemukan lewat code review sebelum commit pertama fitur ini.
func (a *App) OpenDocument() (OpenResult, error) {
	a.fileMu.Lock()
	defer a.fileMu.Unlock()

	path, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title:   "Buka dokumen LaTeX",
		Filters: texFileFilters,
	})
	if err != nil {
		return OpenResult{}, fmt.Errorf("dialog buka file: %w", err)
	}
	if path == "" {
		return OpenResult{Cancelled: true}, nil
	}

	info, err := os.Stat(path)
	if err != nil {
		return OpenResult{}, fmt.Errorf("baca info %s: %w", path, err)
	}
	if info.Size() > maxOpenFileBytes {
		return OpenResult{}, fmt.Errorf("%s berukuran %d byte, melebihi batas %d byte yang didukung latex-studio", path, info.Size(), maxOpenFileBytes)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return OpenResult{}, fmt.Errorf("baca %s: %w", path, err)
	}
	if !utf8.Valid(data) {
		return OpenResult{}, fmt.Errorf("%s bukan teks UTF-8 valid -- latex-studio cuma mendukung dokumen .tex berenkoding UTF-8 pada increment ini, buka sudah ditolak supaya tidak berisiko menimpanya dengan versi yang rusak kalau disimpan lagi", path)
	}
	return OpenResult{Path: path, Content: string(data)}, nil
}

// SaveResult adalah hasil SaveDocumentAs -- Cancelled true (Path kosong)
// kalau operator menutup dialog tanpa memilih lokasi, dibedakan dari
// error sungguhan seperti OpenResult.Cancelled.
type SaveResult struct {
	Path      string `json:"path"`
	Cancelled bool   `json:"cancelled"`
}

// SaveDocumentAs SELALU menampilkan dialog lokasi simpan (dipakai untuk
// "Simpan Sebagai" maupun "Simpan" pertama kali saat dokumen belum
// punya path -- lihat SaveDocument), menulis content ke path yang dipilih
// operator, dan mengembalikan path itu supaya frontend bisa mengingatnya
// untuk panggilan SaveDocument berikutnya. Dilindungi fileMu yang sama
// dengan OpenDocument/SaveDocument -- lihat komentar field-nya.
func (a *App) SaveDocumentAs(content string) (SaveResult, error) {
	a.fileMu.Lock()
	defer a.fileMu.Unlock()

	path, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		Title:           "Simpan dokumen LaTeX",
		DefaultFilename: "dokumen.tex",
		Filters:         texFileFilters,
	})
	if err != nil {
		return SaveResult{}, fmt.Errorf("dialog simpan file: %w", err)
	}
	if path == "" {
		return SaveResult{Cancelled: true}, nil
	}
	if err := atomicWriteFile(path, []byte(content), 0o644); err != nil {
		return SaveResult{}, fmt.Errorf("tulis %s: %w", path, err)
	}
	return SaveResult{Path: path}, nil
}

// SaveDocument menulis content langsung ke path yang SUDAH DIKETAHUI
// (dokumen yang sebelumnya dibuka lewat OpenDocument atau sudah pernah
// disimpan lewat SaveDocumentAs) -- TANPA menampilkan dialog apa pun.
// Frontend wajib memanggil SaveDocumentAs dulu (bukan fungsi ini) kalau
// dokumen belum pernah punya path. Dilindungi fileMu yang sama dengan
// OpenDocument/SaveDocumentAs -- lihat komentar field-nya.
func (a *App) SaveDocument(path, content string) error {
	a.fileMu.Lock()
	defer a.fileMu.Unlock()

	if path == "" {
		return errors.New("path kosong -- dokumen belum pernah disimpan, panggil SaveDocumentAs dulu")
	}
	if err := atomicWriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("tulis %s: %w", path, err)
	}
	return nil
}

// atomicWriteFile menulis data ke path lewat file sementara di DIREKTORI
// YANG SAMA dulu, baru os.Rename ke path -- supaya kegagalan di tengah
// penulisan (disk penuh, proses dibunuh paksa, koneksi ke network drive
// putus, dst.) TIDAK PERNAH meninggalkan path dalam kondisi terpotong/
// rusak. Isi LAMA di path tetap utuh sampai penulisan BARU benar-benar
// selesai penuh dan siap menggantikannya -- os.Rename pada volume yang
// sama atomic baik di POSIX maupun Windows (pola yang sama dengan
// instalasi binary Tectonic, lihat internal/tectonic/tectonic.go).
// Menulis LANGSUNG ke path (os.WriteFile biasa, seperti draf awal fungsi
// ini) berisiko merusak dokumen yang sedang diedit operator kalau
// penulisan gagal di tengah jalan -- ditemukan lewat code review sebelum
// commit pertama.
func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".latex-studio-save-*")
	if err != nil {
		return fmt.Errorf("buat file sementara di %s: %w", dir, err)
	}
	tmpPath := tmp.Name()
	_, writeErr := tmp.Write(data)
	closeErr := tmp.Close()
	if writeErr != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("tulis file sementara: %w", writeErr)
	}
	if closeErr != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("tutup file sementara: %w", closeErr)
	}
	if err := os.Chmod(tmpPath, perm); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("set permission file sementara: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("pasang file ke %s: %w", path, err)
	}
	return nil
}
