# latex-studio

Editor LaTeX standalone dengan GUI: tulis dokumen `.tex` di panel kiri,
lihat hasil PDF-nya di panel kanan lewat auto-recompile (debounced, ~900ms
setelah berhenti mengetik). "Standalone" di sini berarti operator TIDAK
perlu install TeX Live/MiKTeX secara terpisah -- engine LaTeX-nya
(Tectonic) di-bootstrap otomatis oleh aplikasi sendiri saat pertama kali
dijalankan.

## Stack

- **Backend**: Go + [Wails v2](https://wails.io) -- satu `.exe` native,
  bukan Electron/Chromium yang di-bundle terpisah (WebView2 bawaan Windows
  dipakai sebagai renderer).
- **Frontend**: vanilla JS + [CodeMirror 6](https://codemirror.net/)
  (syntax highlighting LaTeX lewat `@codemirror/legacy-modes/mode/stex`)
  + `<iframe>` PDF bawaan WebView2 (Chromium) untuk preview -- TIDAK pakai
  PDF.js pada increment ini, penampil bawaan browser sudah cukup untuk
  preview sederhana.
- **Engine LaTeX**: [Tectonic](https://tectonic-typesetting.github.io/) --
  self-contained (satu binary), versi DIPIN (lihat `tectonicVersion` di
  `internal/tectonic/tectonic.go`, bukan "latest release" GitHub) dan
  diverifikasi checksum SHA-256 sebelum dipakai. Paket TeX yang dibutuhkan
  dokumen diunduh on-demand dari bundle resource default lalu di-cache
  lokal oleh Tectonic sendiri. Internet cuma dibutuhkan sekali per paket
  baru yang belum pernah dipakai; kompilasi berikutnya terhadap paket yang
  sama jalan offline.

## Struktur

```text
main.go                       entry point Wails, wiring OnStartup/OnShutdown/Bind
app.go                        struct App -- method public-nya dibind ke frontend (EnsureEngine, Compile)
internal/tectonic/            bootstrap (unduh+ekstrak) dan eksekusi Tectonic
frontend/src/main.js           editor CodeMirror, debounce compile, render PDF
frontend/src/style.css         layout split-pane
frontend/wailsjs/               binding JS yang di-generate otomatis dari app.go (JANGAN edit manual)
```

## Kenapa Tectonic, bukan TeX Live/MiKTeX

TeX Live/MiKTeX butuh instalasi terpisah yang beratnya bisa gigabyte-an
sebelum operator bisa mulai memakai aplikasi ini sama sekali -- bertentangan
dengan tujuan "standalone". Tectonic satu binary (~50MB, diunduh otomatis
oleh `internal/tectonic.Ensure` ke cache lokal, lihat di bawah), dan paket
TeX individual diunduh on-demand persis saat dokumen membutuhkannya, bukan
semuanya di depan.

Konsekuensinya: kompilasi PERTAMA terhadap kombinasi paket yang belum
pernah dipakai butuh internet dan bisa lambat (diverifikasi manual: ~2m30s
untuk dokumen `article` paling dasar, sebagian besar waktu itu unduhan
paket font/class). Kompilasi BERIKUTNYA terhadap paket yang sama cepat dan
offline (diverifikasi manual: ~0.5s).

## Cache

- **Binary Tectonic**: `%LOCALAPPDATA%\latex-studio\bin\tectonic.exe`
  (Windows) atau `$XDG_CACHE_HOME/latex-studio/bin/tectonic` (platform
  lain) -- diunduh sekali oleh `internal/tectonic.Ensure` (versi dipin +
  checksum SHA-256 diverifikasi + ekstraksi atomic, lihat CHANGELOG.md
  bagian Security), dipakai ulang selamanya setelah itu tanpa menyentuh
  jaringan. Kalau perlu memaksa unduh ulang (mis. curiga cache korup),
  hapus manual filenya -- `Ensure` akan mengunduh dan memverifikasi lagi
  dari awal.
- **Cache paket TeX**: diurus Tectonic sendiri ke direktori cache OS
  standar miliknya (di luar kendali/pengetahuan langsung latex-studio).

## Build dan jalankan

Prasyarat: Go, Node.js, dan C compiler (Wails pakai cgo untuk binding
WebView2) -- di Windows, `scoop install main/gcc` kalau belum ada.

```powershell
go install github.com/wailsapp/wails/v2/cmd/wails@latest
wails dev      # mode development, hot reload frontend
wails build    # build production -> build/bin/latex-studio.exe
```

## Batasan pada increment ini (lihat ROADMAP.md untuk detail)

- **Belum ada save/open file** -- dokumen cuma ada di memori editor
  selama aplikasi berjalan, ditulis ulang ke direktori kerja sementara
  (dihapus saat aplikasi ditutup) setiap kali dikompilasi. Menutup
  aplikasi tanpa menyalin isi editor keluar berarti dokumen hilang.
- **Satu dokumen, satu file** -- tidak ada multi-tab, tidak ada `\input`/
  `\include` lintas file, tidak ada dukungan asset (gambar, bibliografi).
- Format PDF/JSON compile diverifikasi manual (screenshot aplikasi
  sungguhan, lihat CHANGELOG.md) -- `internal/tectonic` sudah punya unit
  test untuk logic murni (checksum, URL, verifikasi hash), tapi jalur
  yang benar-benar bicara ke Tectonic/jaringan (`Ensure`/`Compile`) dan
  `app.go` masih belum ada automated test sama sekali, lihat ROADMAP.md.
