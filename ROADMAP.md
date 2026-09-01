# Roadmap

## MVP editor + preview

- [x] Scaffold Wails v2 (Go + vanilla JS/Vite), environment build (gcc via
      scoop, Wails CLI) disiapkan dan diverifikasi lewat `wails doctor`.
- [x] `internal/tectonic`: bootstrap Tectonic (deteksi platform GOOS/GOARCH
      -> asset rilis GitHub yang benar, unduh, ekstrak dari
      zip/tar.gz sesuai platform, cache ke `%LOCALAPPDATA%\latex-studio\bin`
      atau setara di platform lain) dan eksekusi kompilasi (`tectonic -p
      --color never -o <workdir> main.tex` -> `main.pdf` + log gabungan
      stdout/stderr).
- [x] `app.go`: method `EnsureEngine` (bootstrap + event progress
      `engine:progress` ke frontend) dan `Compile` (tulis source ke
      `main.tex` di direktori kerja sesi, jalankan Tectonic, kembalikan
      PDF+log) dibind ke frontend lewat Wails.
- [x] Editor CodeMirror 6 dengan syntax highlighting LaTeX
      (`@codemirror/legacy-modes/mode/stex`), auto-recompile debounced
      (~900ms setelah berhenti mengetik, lihat `frontend/src/main.js`).
- [x] Preview PDF lewat `<iframe>` + blob URL (penampil PDF bawaan
      WebView2/Chromium, tanpa PDF.js) -- panel log error di bawah preview
      muncul otomatis kalau kompilasi LaTeX gagal (bukan Go error, beda
      kasus, lihat komentar `Compile` di `app.go`).
- [x] **Diverifikasi manual end-to-end** lewat build production
      (`wails build`) + screenshot aplikasi sungguhan (`PrintWindow`,
      bukan `CopyFromScreen` yang ternyata menangkap window LAIN yang
      kebetulan menutupi area layar yang sama -- lihat catatan di
      CHANGELOG.md): dokumen default (`\documentclass{article}` + `\maketitle`
      + rumus `$E=mc^2$`) berhasil dikompilasi otomatis saat aplikasi
      dibuka (compile pertama pakai cache Tectonic yang sudah dipanaskan
      manual sebelumnya, ~0.5s) dan PDF-nya tampil benar di preview pane
      dengan title, author area, dan rumus matematika ter-render dengan
      benar.
- [ ] **Belum diverifikasi langsung**: auto-recompile yang dipicu
      KETIKAN (debounce di `EditorView.updateListener`) -- percobaan
      mengirim keystroke terprogram (Windows `SendKeys` maupun COM
      `WScript.Shell.AppActivate`) ke jendela aplikasi TIDAK sampai ke
      elemen contenteditable CodeMirror di dalam WebView2 (keterbatasan
      umum: WebView2/Chromium modern tidak selalu merespons pesan
      Win32 `WM_CHAR`/`WM_KEYDOWN` sintetis lewat jalur input klasik
      seperti kontrol native). Path kode compile yang dipicu debounce
      MEMANGGIL FUNGSI YANG SAMA PERSIS (`runCompile`) dengan yang sudah
      terbukti jalan saat startup, jadi risikonya dinilai rendah -- tapi
      tetap dicatat jujur sebagai belum dibuktikan lewat interaksi
      sungguhan, bukan diklaim teruji penuh. Verifikasi mendatang butuh
      driver berbasis WebView2 DevTools Protocol (setara Playwright, lihat
      catatan `run` skill di sesi ini), bukan `SendKeys`.
- [x] **Hardening keamanan/robustness `internal/tectonic` dan `app.go`**
      -- tiga temuan dari code review sebelum commit pertama, semuanya
      diperbaiki dan diverifikasi ulang (rebuild + screenshot, fast path
      `Ensure` terbukti tidak regresi): `workDir` TIDAK PERNAH lagi
      fallback ke `os.TempDir()` (sebelumnya berisiko `os.RemoveAll`
      menyapu direktori temp OS -- lihat CHANGELOG.md bagian Security
      untuk skenario lengkapnya), `compileTimeout` dinaikkan 2 menit ->
      5 menit plus `bootstrapTimeout` (10 menit) terpisah untuk
      unduhan+ekstraksi binary, dan unduhan Tectonic sekarang versi
      dipin + checksum SHA-256 dipin + batas ukuran + ekstraksi atomic
      (extract-ke-temp-lalu-rename) alih-alih "latest release" tanpa
      verifikasi apa pun. Detail lengkap tiap temuan dan fix-nya di
      CHANGELOG.md bagian Security.

## Save/open file

- [x] `app.go`: `OpenDocument` (dialog native, baca `.tex` terpilih),
      `SaveDocument` (tulis langsung ke path yang sudah diketahui, TANPA
      dialog), `SaveDocumentAs` (SELALU tampilkan dialog lokasi) dibind ke
      frontend -- ketiganya dilindungi `fileMu` (`sync.Mutex` TERPISAH
      dari `mu` yang dipakai `Compile`/`EnsureEngine`, lihat komentar
      field-nya di `app.go`): operasi file berdasar dialog tidak
      menyentuh `engineBin`/`workDir`, jadi membuka/menyimpan file tidak
      perlu menunggu kompilasi yang mungkin sedang berjalan sampai
      beberapa menit, TAPI tetap perlu diserialkan terhadap SESAMA
      operasi file (ditemukan lewat code review, lihat CHANGELOG.md
      bagian Security). `SaveDocument`/`SaveDocumentAs` menulis lewat
      `atomicWriteFile` (temp sibling + `os.Rename`), `OpenDocument`
      menolak file >50 MiB atau yang bukan UTF-8 valid.
- [x] Toolbar (Baru/Buka/Simpan/Simpan sebagai) + keyboard shortcut
      (Ctrl+O, Ctrl+S, Ctrl+Shift+S) di `frontend/src/main.js`. Indikator
      "belum disimpan" (titik di judul dokumen) muncul begitu editor
      berubah SETELAH dibuka/disimpan, hilang lagi setelah simpan
      berikutnya sukses SELAMA tidak ada ketikan baru selagi penyimpanan
      berlangsung (`docGeneration`, lihat CHANGELOG.md bagian Security).
      `suppressDirty` mencegah penggantian isi programatik (Buka/Baru)
      sendiri keliru dihitung sebagai perubahan operator.
      `fileOperationInProgress`/`withFileOperation` mencegah Buka/Simpan/
      Simpan sebagai/Baru tumpang tindih (tombol toolbar dinonaktifkan
      sementara selagi satu operasi berjalan). Konfirmasi `confirm()`
      native muncul kalau operator membuka file lain/membuat dokumen baru
      selagi ada perubahan belum disimpan.
- [x] **Diverifikasi**: build Go (`go build`, `go vet`, `gofmt`) dan
      binding JS (`wails generate module`) sukses, `wails build` production
      berhasil (diulang lagi setelah perbaikan Security, tidak regresi --
      screenshot ulang menunjukkan toolbar dan compile-otomatis-saat-startup
      masih identik), `go test ./...` lulus termasuk `app_test.go` baru
      (`TestSaveDocument`, `TestAtomicWriteFile`). `git diff --check`
      SEKARANG BENAR-BENAR bersih (exit 0) -- klaim "bersih" pada laporan
      sebelum perbaikan Security TIDAK AKURAT, lihat CHANGELOG.md.
- [ ] **Belum diverifikasi lewat interaksi sungguhan**: klik tombol
      Buka/Simpan (memicu dialog file native) dan alur simpan/buka
      end-to-end. Percobaan mengklik lewat koordinat mouse sintetis
      (`mouse_event` + `SetCursorPos`, sudah dikoreksi untuk DPI scaling
      lewat `SetProcessDpiAwarenessContext`) TIDAK presisi -- klik yang
      dimaksudkan ke tombol "Buka" malah jatuh di area editor,
      kemungkinan karena WebView2/Chromium me-render tata letak dengan
      cara yang tidak 1:1 dengan asumsi koordinat proporsional dari
      screenshot sebelumnya. Sama seperti keterbatasan `SendKeys` yang
      sudah dicatat sebelumnya -- kode-nya sendiri straightforward
      (event listener standar memanggil method Wails-bound yang
      membungkus `wailsruntime.OpenFileDialog`/`SaveFileDialog`, API
      Wails yang umum dipakai), tapi belum dibuktikan lewat klik
      sungguhan. Verifikasi mendatang butuh driver WebView2 DevTools
      Protocol yang bisa klik lewat DOM (`element.click()`) alih-alih
      koordinat layar, sama seperti catatan verifikasi ketikan di atas.
- [ ] Belum ada peringatan/pencegahan menutup APLIKASI (bukan cuma ganti
      dokumen) selagi ada perubahan belum disimpan -- indikator dirty di
      judul adalah satu-satunya sinyal saat ini. Butuh `OnBeforeClose`
      (`options.App`, Go) yang menanyakan status dirty ke frontend
      sebelum window benar-benar tertutup -- di luar cakupan increment
      ini, SENGAJA tidak ditambah sekarang supaya tidak menambah
      kompleksitas round-trip Go<->JS yang belum teruji cuma untuk fitur
      pelengkap (dibanding kemampuan inti simpan/buka yang sudah ada).

## Belum dikerjakan (di luar cakupan MVP ini)

- [ ] **Multi-file project**: `\input`/`\include`, gambar/asset relatif,
      bibliografi (`.bib`) -- `internal/tectonic.Compile` sudah menerima
      `workDir` per sesi yang bisa jadi basis untuk ini, tapi UI-nya
      (file tree, dst.) belum ada sama sekali.
- [ ] **Multi-tab / multi-dokumen** dalam satu jendela aplikasi.
- [ ] Sinkronisasi posisi kursor <-> halaman PDF (SyncTeX) -- Tectonic
      sudah punya flag `--synctex` yang belum dipakai.
- [ ] Autocomplete/snippet perintah LaTeX di editor (CodeMirror mendukung
      lewat `autocompletion()`, belum diaktifkan).
- [ ] Export/simpan PDF hasil compile ke lokasi pilihan operator (saat ini
      PDF cuma ada di memori sebagai blob URL, hilang saat aplikasi
      ditutup).
- [ ] Automated test untuk `Ensure`/`Compile` yang benar-benar bicara ke
      Tectonic/jaringan -- `internal/tectonic/tectonic_test.go` sudah
      menutup logic murni (checksum, URL, verifikasi hash, lihat
      CHANGELOG.md bagian Security), tapi jalur yang benar-benar
      mengunduh/menjalankan Tectonic masih cuma diverifikasi manual
      (lihat bagian MVP di atas). Butuh HTTP test server palsu
      (`httptest.Server`, pola yang sama dipakai `internal/ollama` pada
      local-agent-playground) untuk menguji `downloadToTemp`/`Ensure`
      tanpa GitHub sungguhan, dan Tectonic asli tetap dibutuhkan untuk
      menguji `Compile`. `app_test.go` sudah menutup `SaveDocument`
      (satu-satunya method file I/O yang TIDAK menampilkan dialog, jadi
      bisa dites hermetik: path kosong ditolak, tulis, timpa file yang
      sudah ada). `EnsureEngine`/`Compile` (butuh Tectonic sungguhan) dan
      `OpenDocument`/`SaveDocumentAs` (butuh window OS sungguhan untuk
      dialog native `wailsruntime.OpenFileDialog`/`SaveFileDialog`, tidak
      bisa dipanggil dari `go test` biasa) masih belum ada test sama
      sekali.
- [ ] Packaging installer (NSIS, sudah terdeteksi opsional oleh
      `wails doctor` tapi belum dikonfigurasi) untuk distribusi di luar
      `wails build` mentah.
