package main

import (
	"context"
	"fmt"
	"os"
	"sync"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"latex-studio/internal/tectonic"
)

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
