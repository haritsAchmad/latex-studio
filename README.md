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
  (syntax highlighting LaTeX lewat `@codemirror/legacy-modes/mode/stex`,
  autocomplete perintah LaTeX lewat `@codemirror/autocomplete`)
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
app.go                        struct App -- method public-nya dibind ke frontend (EnsureEngine, Compile, OpenDocument, SaveDocument, SaveDocumentAs, ExportPDF)
app_test.go                   unit test untuk SaveDocument, atomicWriteFile, dan bagian ExportPDF tanpa dialog (lihat ROADMAP.md)
internal/tectonic/            bootstrap (unduh+ekstrak) dan eksekusi Tectonic
frontend/src/main.js           editor CodeMirror, debounce compile, render PDF, toolbar Buka/Simpan/Simpan sebagai
frontend/src/style.css         layout split-pane + toolbar
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

**Sebelum commit setelah `wails build`/`wails generate module`**: generator
TypeScript Wails (`frontend/wailsjs/go/models.ts`, kadang juga
`App.js`/`App.d.ts`) konsisten meninggalkan trailing whitespace (tab) di
baris-baris tertentu -- ditemukan lewat `git diff --check` yang gagal
sebelum commit pertama fitur save/open, dan MUNCUL LAGI di build
berikutnya (bukan kejadian sekali doang, sudah dikonfirmasi). Jalankan ini
sebelum commit kalau ada perubahan di `frontend/wailsjs/`:

```bash
sed -i 's/[ \t]*$//' frontend/wailsjs/go/models.ts frontend/wailsjs/go/main/App.js frontend/wailsjs/go/main/App.d.ts frontend/wailsjs/runtime/runtime.js frontend/wailsjs/runtime/runtime.d.ts
git diff --check   # harus tidak menghasilkan output apa pun (exit 0)
```

**Sebelum commit setelah `wails build -nsis`**: `build/windows/installer/wails_tools.nsh`
ikut di-RENDER Wails jadi berisi nilai asli (`INFO_COMPANYNAME`, dst.),
padahal yang seharusnya di-commit adalah versi TEMPLATE-nya
(`{{.Info.CompanyName}}`, dst.) -- `wails build -nsis` merender ulang
file itu setiap kali dipanggil (sama seperti binding TypeScript di atas),
jadi commit versi template lebih tahan lama/tidak basi dibanding
hardcode nilai hasil render. Kembalikan sebelum commit:

```bash
git checkout -- build/windows/installer/wails_tools.nsh
```

## Installer (NSIS)

```powershell
scoop bucket add extras   # sekali saja
scoop install extras/nsis
wails build -nsis   # -> build/bin/latex-studio-amd64-installer.exe
```

Metadata installer (nama perusahaan/produk/versi/copyright) diambil dari
field `info` di `wails.json`, bukan placeholder default Wails. Installer
default minta elevasi admin (install ke `Program Files`) -- menjalankannya
memicu prompt UAC yang wajib disetujui/dibatalkan operator secara
interaktif, tidak bisa diotomasi/di-skip dengan cara apa pun.

## Save/open file

Toolbar (Baru/Buka/Simpan/Simpan sebagai) dan keyboard shortcut standar
(Ctrl+O, Ctrl+S, Ctrl+Shift+S) lewat dialog file native
(`wailsruntime.OpenFileDialog`/`SaveFileDialog`). Judul dokumen di toolbar
menampilkan nama file dan titik (`•`) kalau ada perubahan belum disimpan.
Menutup aplikasi TIDAK PERNAH mencegah/mengingatkan soal perubahan belum
disimpan pada increment ini -- lihat ROADMAP.md.

## Export PDF

Tombol "Ekspor PDF" di toolbar menyimpan PDF hasil compile TERAKHIR yang
sukses (bukan baca ulang dari disk -- selalu persis PDF yang sedang
tampil di preview pane) ke lokasi pilihan operator. Nonaktif sampai ada
compile yang sukses sama sekali.

## Autocomplete

Ketik `\` diikuti beberapa huruf untuk memunculkan saran perintah LaTeX
umum (struktur dokumen, formatting, matematika dasar, referensi) --
daftar statis pendek (`LATEX_COMMANDS` di `frontend/src/main.js`), BUKAN
autocomplete yang sadar paket/`\usepackage`. Ctrl+Space memicu popup
biar pun belum ada huruf setelah `\`.

## Batasan pada increment ini (lihat ROADMAP.md untuk detail)

- **Satu dokumen, satu file** -- tidak ada multi-tab, tidak ada `\input`/
  `\include` lintas file, tidak ada dukungan asset (gambar, bibliografi).
- **Tidak ada peringatan menutup aplikasi dengan perubahan belum
  disimpan** -- indikator titik di judul adalah satu-satunya sinyal.
- Format PDF/JSON compile dan toolbar save/open/ekspor diverifikasi lewat
  build + screenshot aplikasi sungguhan, TAPI klik tombol Buka/Simpan/
  Ekspor PDF dan popup autocomplete lewat interaksi terprogram (koordinat
  mouse/keystroke sintetis) belum berhasil diverifikasi -- lihat
  CHANGELOG.md/ROADMAP.md untuk detail keterbatasan otomasi GUI ini.
  `internal/tectonic` dan bagian `app.go` yang tidak
  butuh dialog (`SaveDocument`, `atomicWriteFile`, penolakan awal
  `ExportPDF`, lihat `app_test.go`) sudah punya unit test;
  `Ensure`/`Compile`/`OpenDocument`/`SaveDocumentAs`/`ExportPDF` (bagian
  yang menampilkan dialog) masih belum ada
  automated test sama sekali, lihat ROADMAP.md.
- **`latex-studio.exe` (BUKAN installer-nya) tidak menampilkan metadata
  versi lewat PowerShell `Get-Item .VersionInfo`/.NET `FileVersionInfo`**
  -- CompanyName/ProductName/dst. semua kosong biar pun sudah diisi lewat
  `wails.json`. Dikonfirmasi BUKAN berarti datanya hilang: string yang
  sama ditemukan utuh di dalam binary lewat pencarian UTF-16 mentah, dan
  installer-nya sendiri (dibuat dari mekanisme BERBEDA, `VIAddVersionKey`
  langsung di `project.nsi`) menampilkan metadata dengan benar lewat
  pembacaan yang sama. Kemungkinan besar kuirk upstream Wails --
  `build/windows/info.json` scaffold default memakai kunci bahasa
  `"0000"` (neutral, tanpa codepage eksplisit) yang tampaknya tidak
  dikenali pembaca .NET, BUKAN sesuatu yang diperkenalkan di increment
  ini (file itu tidak diubah sama sekali). Belum dicoba diperbaiki --
  dampaknya kosmetik (tab Properti file di luar installer), bukan
  fungsional.

## Credits

Library, program, font, dan aset pihak lain beserta lisensinya dicatat di [CREDITS.md](CREDITS.md).
