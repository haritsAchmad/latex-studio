package tectonic

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestTectonicChecksumsWellFormed menjaga tectonicChecksums (nilai yang
// dipercaya secara manual, lihat komentarnya) tetap berbentuk SHA-256 hex
// yang valid -- 64 karakter heksadesimal -- untuk SETIAP platform yang
// releaseTarget klaim didukung. Ini bukan verifikasi bahwa nilainya BENAR
// (itu tanggung jawab review manusia terhadap proses di komentar
// tectonicChecksums), cuma jaring pengaman kesalahan ketik/transkripsi
// paling dasar (kependekan, karakter non-hex, dst.) yang akan lolos
// review sekilas tapi membuat verifySHA256 SELALU gagal untuk platform itu.
func TestTectonicChecksumsWellFormed(t *testing.T) {
	hexRE := regexp.MustCompile(`^[0-9a-f]{64}$`)
	platforms := []string{"windows/amd64", "darwin/amd64", "darwin/arm64", "linux/amd64", "linux/arm64"}
	for _, p := range platforms {
		t.Run(p, func(t *testing.T) {
			sum, ok := tectonicChecksums[p]
			if !ok {
				t.Fatalf("tectonicChecksums tidak punya entry untuk %s, padahal releaseTarget mendukungnya", p)
			}
			if !hexRE.MatchString(sum) {
				t.Errorf("tectonicChecksums[%q] = %q, bukan 64 karakter hex lowercase", p, sum)
			}
		})
	}
}

// TestReleaseTargetMatchesChecksums memastikan releaseTarget dan
// tectonicChecksums selalu sinkron: setiap platform yang releaseTarget
// terima juga punya checksum, dan sebaliknya -- keduanya HARUS berubah
// bersamaan, drift di antara keduanya berarti satu platform akan lolos
// releaseTarget tapi gagal di Ensure dengan pesan "tidak ada checksum
// terverifikasi", atau sebaliknya (checksum menganggur tidak terpakai).
func TestReleaseTargetMatchesChecksums(t *testing.T) {
	supported := []string{"windows/amd64", "darwin/amd64", "darwin/arm64", "linux/amd64", "linux/arm64"}
	for _, p := range supported {
		if _, ok := tectonicChecksums[p]; !ok {
			t.Errorf("%s didukung releaseTarget (asumsi test ini) tapi tidak ada di tectonicChecksums", p)
		}
	}
	for p := range tectonicChecksums {
		found := false
		for _, s := range supported {
			if s == p {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("tectonicChecksums punya entry %q yang tidak ada di daftar platform yang didukung test ini -- releaseTarget dan daftar supported di atas perlu disinkronkan", p)
		}
	}
}

func TestDownloadURL(t *testing.T) {
	got := downloadURL("x86_64-pc-windows-msvc", "zip")
	want := "https://github.com/tectonic-typesetting/tectonic/releases/download/tectonic@" + tectonicVersion + "/tectonic-" + tectonicVersion + "-x86_64-pc-windows-msvc.zip"
	if got != want {
		t.Errorf("downloadURL() = %q, ingin %q", got, want)
	}
}

func sha256Hex(t *testing.T, data []byte) string {
	t.Helper()
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func TestVerifySHA256(t *testing.T) {
	t.Run("matching checksum passes", func(t *testing.T) {
		data := []byte("konten arsip palsu untuk testing")
		path := filepath.Join(t.TempDir(), "archive.bin")
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatalf("tulis file test: %v", err)
		}
		if err := verifySHA256(path, sha256Hex(t, data)); err != nil {
			t.Errorf("verifySHA256() error = %v, ingin nil (checksum cocok)", err)
		}
	})

	t.Run("mismatched checksum rejected", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "archive.bin")
		if err := os.WriteFile(path, []byte("isi asli"), 0o644); err != nil {
			t.Fatalf("tulis file test: %v", err)
		}
		wrongSum := sha256Hex(t, []byte("isi yang beda sama sekali"))
		err := verifySHA256(path, wrongSum)
		if err == nil {
			t.Fatal("verifySHA256() error = nil, ingin error (checksum tidak cocok)")
		}
		if !strings.Contains(err.Error(), "tidak cocok") {
			t.Errorf("verifySHA256() error = %q, ingin menyebut checksum tidak cocok", err.Error())
		}
	})

	t.Run("case-insensitive comparison", func(t *testing.T) {
		data := []byte("data lain")
		path := filepath.Join(t.TempDir(), "archive.bin")
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatalf("tulis file test: %v", err)
		}
		upper := strings.ToUpper(sha256Hex(t, data))
		if err := verifySHA256(path, upper); err != nil {
			t.Errorf("verifySHA256() dengan checksum UPPERCASE error = %v, ingin nil", err)
		}
	})
}

func TestPlatformKeyMatchesReleaseTargetSwitch(t *testing.T) {
	// releaseTarget menggunakan platformKey() secara internal -- test ini
	// cuma memastikan platform SAAT INI (mesin yang menjalankan test) tidak
	// diam-diam berakhir di jalur default/error kalau memang seharusnya
	// didukung (mis. typo GOOS/GOARCH yang lolos tanpa disadari).
	_, _, err := releaseTarget()
	supported := map[string]bool{
		"windows/amd64": true, "darwin/amd64": true, "darwin/arm64": true,
		"linux/amd64": true, "linux/arm64": true,
	}
	if supported[platformKey()] && err != nil {
		t.Errorf("releaseTarget() error = %v untuk platform %s yang seharusnya didukung", err, platformKey())
	}
}
