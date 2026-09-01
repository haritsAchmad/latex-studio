// Package tectonic mengurus engine LaTeX yang dipakai latex-studio:
// mem-bootstrap binary Tectonic (https://tectonic-typesetting.github.io/)
// kalau belum ada secara lokal, dan menjalankannya untuk mengompilasi
// dokumen .tex jadi PDF.
//
// Tectonic dipilih (bukan mewajibkan TeX Live/MiKTeX terinstall) karena dia
// self-contained: satu binary, TIDAK butuh instalasi distribusi TeX
// terpisah yang beratnya bisa gigabyte-an. Paket TeX yang dibutuhkan
// dokumen diunduh on-demand dari bundle resource default Tectonic lalu
// di-cache lokal oleh Tectonic sendiri -- internet cuma dibutuhkan sekali
// per paket baru yang belum pernah dipakai, setelah itu kompilasi ulang
// jalan offline. Ini yang membuat latex-studio bisa disebut "standalone":
// operator tidak perlu install apa pun secara manual sebelum memakainya.
package tectonic

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// tectonicVersion adalah versi Tectonic yang DIPIN, bukan "latest release"
// GitHub -- SENGAJA, dua alasan: (1) reproducibility, semua operator dapat
// binary yang identik dan sudah diverifikasi kerja dengan latex-studio,
// bukan versi baru yang mungkin mengubah flag CLI/nama asset/perilaku
// tanpa latex-studio pernah mengujinya; (2) keamanan, checksum SHA-256 di
// bawah (tectonicChecksums) cuma valid untuk asset versi PERSIS ini --
// memakai "latest" akan membuat pin checksum itu sendiri tidak berguna
// (checksum harus diperbarui manual tiap kali versi dinaikkan, lihat
// komentar tectonicChecksums). Naikkan bareng tectonicChecksums kalau mau
// update Tectonic, jangan salah satu saja.
const tectonicVersion = "0.17.0"

// tectonicChecksums berisi SHA-256 (hex) resmi GitHub untuk asset rilis
// Tectonic tectonicVersion di atas, per platform yang didukung
// releaseTarget. Diambil manual lewat
//
//	gh api repos/tectonic-typesetting/tectonic/releases/tags/tectonic@0.17.0 \
//	  --jq '.assets[] | "\(.name) \(.digest)"'
//
// dan di-cross-check terhadap `sha256sum` lokal atas file yang BENAR-BENAR
// diunduh (bukan cuma disalin dari API tanpa verifikasi independen) sebelum
// ditulis ke sini. Ensure menolak binary yang checksum-nya tidak cocok
// SEBELUM diekstrak dan dijalankan -- lihat verifySHA256. Nilai ini SENGAJA
// hardcoded di source, bukan diambil live dari API tiap kali Ensure
// berjalan: kalau diambil live dari sumber yang SAMA dengan yang mau
// diverifikasi, checksum itu tidak melindungi dari apa pun (respons yang
// dipalsukan akan menyertakan checksum palsu yang "cocok" juga). Titik
// kepercayaannya ada di REVIEW MANUSIA atas nilai yang di-commit ke source
// ini, bukan di jaringan. HARUS diperbarui manual (dengan proses yang sama)
// kalau tectonicVersion dinaikkan.
var tectonicChecksums = map[string]string{
	"windows/amd64": "f61ce51f0b0ade1015b7de7ef368541c5424e9756ecbd0d7af97d6d48030845f",
	"darwin/amd64":  "7c90ef5b6ddb1eb1937e4337add5237b79338e4b9676459fa91187d24d6cdf80",
	"darwin/arm64":  "a3f1cac7c5678f01661a92212f58480ae3b0634115d880dbc59e2953ded45667",
	"linux/amd64":   "1a715688baf591e650c8aeb160ae934e181685eecbb38b317de30b269ac5d606",
	"linux/arm64":   "b10954a95404f3ab2328d2fa59a5ebab8e657f893fab096f98be8db7c0c979b8",
}

// maxDownloadBytes adalah batas atas ukuran asset yang diunduh Ensure --
// rilis Tectonic hari ini paling besar ~25MB (macOS arm64), 200MiB memberi
// margin besar untuk pertumbuhan rilis mendatang sambil tetap menolak
// respons yang tidak wajar (mis. proxy/CDN yang salah konfigurasi
// mengirim balik halaman error raksasa, atau kondisi jaringan aneh
// lainnya) alih-alih diam-diam mengunduh tanpa batas ke disk.
const maxDownloadBytes = 200 * 1024 * 1024

// bootstrapTimeout membatasi SELURUH proses Ensure (cek asset, unduh,
// verifikasi checksum, ekstrak) -- terpisah dari compileTimeout (lihat di
// bawah) karena ini murni operasi jaringan+disk, tidak menjalankan
// Tectonic sama sekali. 10 menit memberi margin besar untuk koneksi
// lambat mengunduh satu asset ~20MB.
const bootstrapTimeout = 10 * time.Minute

// bootstrapHTTPClient dipakai KHUSUS untuk mengunduh asset Tectonic --
// http.DefaultClient TIDAK punya Timeout sama sekali (bisa menggantung
// selamanya kalau koneksi macet di tengah, bukan cuma gagal connect), jadi
// dipakai client terpisah dengan Timeout eksplisit di sini sebagai lapis
// pertahanan tambahan DI LUAR context.Context yang sudah dipasang di
// pemanggilan (defense in depth -- context timeout melindungi dari sisi
// pemanggil, client.Timeout melindungi dari sisi http.Client itu sendiri).
var bootstrapHTTPClient = &http.Client{Timeout: bootstrapTimeout}

// ProgressFunc dipanggil Ensure untuk melaporkan tahapan bootstrap ke
// pemanggil (biasanya diteruskan ke frontend lewat Wails event) -- supaya
// operator tahu latex-studio sedang mengunduh engine (bisa puluhan MB),
// bukan diam/hang tanpa penjelasan pada peluncuran pertama.
type ProgressFunc func(stage string)

func noopProgress(string) {}

// CacheDir mengembalikan direktori tempat binary Tectonic disimpan setelah
// diunduh: %LOCALAPPDATA%\latex-studio\bin di Windows, atau
// $XDG_CACHE_HOME/latex-studio/bin (fallback ~/.cache/latex-studio/bin) di
// platform lain. Terpisah dari cache resource TeX milik Tectonic sendiri
// (yang diurus Tectonic ke direktori cache OS standar miliknya sendiri,
// bukan urusan package ini).
func CacheDir() (string, error) {
	if runtime.GOOS == "windows" {
		dir := os.Getenv("LOCALAPPDATA")
		if dir == "" {
			return "", fmt.Errorf("environment variable LOCALAPPDATA tidak diset")
		}
		return filepath.Join(dir, "latex-studio", "bin"), nil
	}
	dir := os.Getenv("XDG_CACHE_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("tentukan lokasi cache default: %w", err)
		}
		dir = filepath.Join(home, ".cache")
	}
	return filepath.Join(dir, "latex-studio", "bin"), nil
}

func binaryName() string {
	if runtime.GOOS == "windows" {
		return "tectonic.exe"
	}
	return "tectonic"
}

// platformKey adalah kunci "GOOS/GOARCH" yang dipakai releaseTarget DAN
// tectonicChecksums -- diekstrak jadi fungsi sendiri supaya dua tempat itu
// selalu sinkron (mengambil dari runtime.GOOS/runtime.GOARCH yang sama),
// bukan berisiko salah ketik konstanta yang berbeda di dua tempat.
func platformKey() string {
	return runtime.GOOS + "/" + runtime.GOARCH
}

// releaseTarget memetakan platformKey ke akhiran nama asset rilis Tectonic
// di GitHub (lihat https://github.com/tectonic-typesetting/tectonic/releases)
// beserta jenis arsipnya. Cuma platform dengan build resmi Tectonic DAN
// checksum yang sudah diverifikasi di tectonicChecksums yang didaftarkan
// di sini -- kombinasi lain mengembalikan error jelas, bukan mencoba
// menebak nama asset atau melewati verifikasi checksum.
func releaseTarget() (suffix, archiveExt string, err error) {
	switch platformKey() {
	case "windows/amd64":
		return "x86_64-pc-windows-msvc", "zip", nil
	case "darwin/amd64":
		return "x86_64-apple-darwin", "tar.gz", nil
	case "darwin/arm64":
		return "aarch64-apple-darwin", "tar.gz", nil
	case "linux/amd64":
		return "x86_64-unknown-linux-gnu", "tar.gz", nil
	case "linux/arm64":
		return "aarch64-unknown-linux-musl", "tar.gz", nil
	default:
		return "", "", fmt.Errorf("tectonic tidak punya rilis resmi terverifikasi untuk %s -- lihat https://github.com/tectonic-typesetting/tectonic/releases untuk platform yang didukung upstream, lalu tambahkan checksum-nya ke tectonicChecksums", platformKey())
	}
}

// downloadURL membangun URL unduhan langsung dari tectonicVersion + suffix
// platform -- TIDAK memanggil GitHub API "latest release" sama sekali
// (lihat komentar tectonicVersion untuk alasannya), jadi tidak ada
// panggilan jaringan tambahan sebelum unduhan asset yang sesungguhnya.
func downloadURL(suffix, archiveExt string) string {
	return fmt.Sprintf(
		"https://github.com/tectonic-typesetting/tectonic/releases/download/tectonic@%s/tectonic-%s-%s.%s",
		tectonicVersion, tectonicVersion, suffix, archiveExt,
	)
}

// Ensure mengembalikan path ke binary Tectonic yang siap dipakai --
// mengunduh, memverifikasi checksum, dan mengekstraknya ke CacheDir()
// dulu kalau belum ada. Aman dipanggil berulang kali (idempotent):
// panggilan setelah yang pertama cuma stat file yang sudah ada tanpa
// menyentuh jaringan sama sekali, itulah yang membuat kompilasi
// berikutnya bisa berjalan offline.
//
// Sebuah file yang SUDAH ADA di binPath dipercaya tanpa verifikasi ulang
// (tidak di-hash lagi tiap Ensure dipanggil) -- ini AMAN karena binPath
// HANYA PERNAH ditulis lewat os.Rename dari file sementara yang sudah lolos
// verifySHA256 secara penuh (lihat di bawah); rename itu atomic (POSIX
// rename(2) maupun Windows MoveFileEx) dan cuma dipanggil SETELAH seluruh
// proses ekstraksi selesai tanpa error. Artinya tidak ada jalur kode yang
// bisa meninggalkan file PARSIAL/korup persis di binPath -- keberadaan
// file di sana secara konstruksi berarti dia utuh dan sudah pernah lolos
// verifikasi, bukan asumsi yang dipercaya begitu saja.
func Ensure(ctx context.Context, progress ProgressFunc) (string, error) {
	if progress == nil {
		progress = noopProgress
	}
	dir, err := CacheDir()
	if err != nil {
		return "", err
	}
	binPath := filepath.Join(dir, binaryName())
	if info, err := os.Stat(binPath); err == nil && !info.IsDir() {
		return binPath, nil
	}

	suffix, archiveExt, err := releaseTarget()
	if err != nil {
		return "", err
	}
	checksum, ok := tectonicChecksums[platformKey()]
	if !ok {
		return "", fmt.Errorf("tidak ada checksum terverifikasi untuk %s pada Tectonic %s -- lihat tectonicChecksums", platformKey(), tectonicVersion)
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("buat direktori cache %s: %w", dir, err)
	}

	bootCtx, cancel := context.WithTimeout(ctx, bootstrapTimeout)
	defer cancel()

	progress(fmt.Sprintf("mengunduh Tectonic %s...", tectonicVersion))
	assetURL := downloadURL(suffix, archiveExt)
	// archivePath sengaja dibuat di dir cache yang sama (bukan os.TempDir())
	// -- lihat alasan atomic-rename di komentar ekstraksi di bawah, alasan
	// yang sama berlaku di sini supaya semua file sementara ada di volume
	// yang sama dengan tujuan akhirnya.
	archivePath, err := downloadToTemp(bootCtx, dir, assetURL)
	if err != nil {
		return "", err
	}
	defer os.Remove(archivePath)

	progress("memverifikasi checksum Tectonic...")
	if err := verifySHA256(archivePath, checksum); err != nil {
		return "", err
	}

	progress("mengekstrak Tectonic...")
	// Ekstrak ke file SEMENTARA di direktori yang SAMA dengan binPath
	// dulu, baru os.Rename ke binPath di akhir SETELAH semuanya sukses --
	// supaya proses yang terganggu di tengah ekstraksi (crash, dicabut
	// paksa, kehabisan disk, dst.) TIDAK PERNAH meninggalkan file PARSIAL
	// persis di binPath yang nanti keliru dianggap "sudah siap dipakai"
	// oleh pengecekan os.Stat di atas. os.Rename pada volume yang sama
	// bersifat atomic baik di POSIX (rename(2)) maupun Windows
	// (MoveFileEx) -- makanya file sementara ini SENGAJA dibuat di
	// direktori dir yang sama (bukan os.TempDir(), yang pada Windows
	// biasanya ada di drive/volume BERBEDA dari %LOCALAPPDATA%, membuat
	// rename lintas volume gagal atau diam-diam jadi copy+delete yang
	// tidak atomic).
	tmpBin, err := os.CreateTemp(dir, "tectonic-extract-*")
	if err != nil {
		return "", fmt.Errorf("buat file sementara ekstraksi: %w", err)
	}
	tmpBinPath := tmpBin.Name()
	tmpBin.Close()
	defer os.Remove(tmpBinPath) // no-op kalau rename di bawah sudah sukses

	if archiveExt == "zip" {
		err = extractZip(archivePath, binaryName(), tmpBinPath)
	} else {
		err = extractTarGz(archivePath, binaryName(), tmpBinPath)
	}
	if err != nil {
		return "", err
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(tmpBinPath, 0o755); err != nil {
			return "", fmt.Errorf("set permission executable pada %s: %w", tmpBinPath, err)
		}
	}
	if err := os.Rename(tmpBinPath, binPath); err != nil {
		return "", fmt.Errorf("pasang binary Tectonic ke %s: %w", binPath, err)
	}
	progress("Tectonic siap.")
	return binPath, nil
}

// downloadToTemp mengunduh url ke file sementara di dir (dir HARUS sama
// dengan tujuan akhir file yang akan dibangun dari unduhan ini, supaya
// rename berikutnya atomic -- lihat komentar Ensure) dan membatasi ukuran
// unduhan ke maxDownloadBytes -- respons yang lebih besar dari itu
// dianggap tidak wajar dan ditolak (file sementara dihapus), bukan
// diam-diam diterima sebagian lalu gagal ambigu di langkah verifikasi
// checksum.
func downloadToTemp(ctx context.Context, dir, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := bootstrapHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("unduh %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unduh %s: status %d", url, resp.StatusCode)
	}
	f, err := os.CreateTemp(dir, "tectonic-download-*")
	if err != nil {
		return "", err
	}
	defer f.Close()

	limited := io.LimitReader(resp.Body, maxDownloadBytes+1)
	n, err := io.Copy(f, limited)
	if err != nil {
		os.Remove(f.Name())
		return "", fmt.Errorf("simpan unduhan Tectonic: %w", err)
	}
	if n > maxDownloadBytes {
		os.Remove(f.Name())
		return "", fmt.Errorf("unduhan Tectonic melebihi batas %d byte -- dihentikan, response dianggap tidak wajar", maxDownloadBytes)
	}
	return f.Name(), nil
}

// verifySHA256 menolak archivePath kalau hash SHA-256-nya tidak PERSIS
// sama dengan wantHex (lihat tectonicChecksums) -- menangkap unduhan yang
// korup/terputus di tengah jalan MAUPUN asset yang isinya berubah dari
// yang diharapkan (mis. kompromi supply-chain pada sumber unduhan),
// sebelum archivePath sempat diekstrak apalagi dijalankan sebagai
// executable.
func verifySHA256(archivePath, wantHex string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("buka unduhan Tectonic untuk verifikasi checksum: %w", err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("hitung checksum unduhan Tectonic: %w", err)
	}
	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(got, wantHex) {
		return fmt.Errorf("checksum SHA-256 unduhan Tectonic tidak cocok (dapat %s, ingin %s) -- unduhan mungkin rusak/terputus atau berubah dari yang diharapkan, TIDAK dipakai", got, wantHex)
	}
	return nil
}

// extractZip mencari satu entry bernama wantName di dalam archive zip dan
// menuliskan isinya ke destPath -- entry lain (lisensi, dst.) diabaikan.
func extractZip(archivePath, wantName, destPath string) error {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("buka arsip Tectonic: %w", err)
	}
	defer r.Close()
	for _, f := range r.File {
		if filepath.Base(f.Name) != wantName {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("baca %s di dalam arsip: %w", wantName, err)
		}
		defer rc.Close()
		out, err := os.OpenFile(destPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
		if err != nil {
			return fmt.Errorf("tulis %s: %w", destPath, err)
		}
		defer out.Close()
		if _, err := io.Copy(out, rc); err != nil {
			return fmt.Errorf("tulis %s: %w", destPath, err)
		}
		return nil
	}
	return fmt.Errorf("arsip Tectonic tidak berisi %s", wantName)
}

// extractTarGz sama seperti extractZip, untuk arsip .tar.gz (macOS/Linux).
func extractTarGz(archivePath, wantName, destPath string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("buka arsip Tectonic: %w", err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("buka gzip arsip Tectonic: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("baca arsip Tectonic: %w", err)
		}
		if filepath.Base(hdr.Name) != wantName {
			continue
		}
		out, err := os.OpenFile(destPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
		if err != nil {
			return fmt.Errorf("tulis %s: %w", destPath, err)
		}
		defer out.Close()
		if _, err := io.Copy(out, tr); err != nil {
			return fmt.Errorf("tulis %s: %w", destPath, err)
		}
		return nil
	}
	return fmt.Errorf("arsip Tectonic tidak berisi %s", wantName)
}

// compileTimeout adalah batas waktu satu kompilasi. SENGAJA dibuat
// longgar (bukan sekadar cukup untuk dokumen kecil ter-cache) -- Tectonic
// bisa mengunduh paket TeX baru YANG BELUM PERNAH DIPAKAI kapan saja
// selama kompilasi, bukan cuma pada kompilasi pertama aplikasi (mis.
// operator menambah \usepackage{tikz} untuk pertama kali di sesi mana
// pun). Diverifikasi manual: dokumen `article` paling dasar dengan SEMUA
// paket belum ter-cache butuh ~2m39s -- batas 5 menit di sini memberi
// margin untuk dokumen yang butuh lebih banyak paket baru sekaligus atau
// koneksi yang lebih lambat, tanpa lag tak berhingga kalau memang ada
// masalah jaringan.
const compileTimeout = 5 * time.Minute

// Result adalah hasil satu kompilasi: PDF yang dihasilkan (nil kalau
// kompilasi gagal), dan Log gabungan stdout+stderr Tectonic apa adanya --
// pesan error LaTeX (baris/paket bermasalah, dst.) ada di sini, ditampilkan
// ke operator supaya bisa mendiagnosis sendiri, bukan cuma "compile
// failed" generik.
type Result struct {
	PDF []byte `json:"pdf"`
	Log string `json:"log"`
	Ok  bool   `json:"ok"`
}

// Compile menulis source ke "main.tex" di dalam workDir (dibuat kalau
// belum ada) lalu menjalankan Tectonic untuk mengompilasinya jadi PDF di
// direktori yang sama. workDir sengaja per-dokumen (dikelola pemanggil,
// lihat app.go) -- bukan cuma supaya file sementara tidak bentrok antar
// kompilasi, tapi juga supaya asset relatif dokumen (gambar, dst. -- belum
// didukung UI pada increment ini, lihat ROADMAP.md) punya working
// directory yang konsisten untuk pengembangan berikutnya.
func Compile(ctx context.Context, binPath, source, workDir string) (Result, error) {
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return Result{}, fmt.Errorf("buat direktori kerja %s: %w", workDir, err)
	}
	texPath := filepath.Join(workDir, "main.tex")
	if err := os.WriteFile(texPath, []byte(source), 0o644); err != nil {
		return Result{}, fmt.Errorf("tulis %s: %w", texPath, err)
	}

	ctx, cancel := context.WithTimeout(ctx, compileTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, binPath, "-p", "--color", "never", "-o", workDir, texPath)
	var out strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &out
	runErr := cmd.Run()

	res := Result{Log: out.String(), Ok: runErr == nil}
	if runErr == nil {
		pdfPath := filepath.Join(workDir, "main.pdf")
		pdf, err := os.ReadFile(pdfPath)
		if err != nil {
			return Result{}, fmt.Errorf("baca hasil PDF %s: %w", pdfPath, err)
		}
		res.PDF = pdf
	}
	return res, nil
}
