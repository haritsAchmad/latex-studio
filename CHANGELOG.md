# Changelog

Semua perubahan penting pada proyek ini dicatat di sini. Format mengikuti
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) dan versi mengikuti
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **Autocomplete perintah LaTeX**: `frontend/src/main.js` menambah
  `latexCompletions`, `CompletionSource` `@codemirror/autocomplete` yang
  memicu popup saat kursor persis setelah backslash diikuti nol atau
  lebih huruf (`context.matchBefore(/\\[a-zA-Z]*/)`), menampilkan
  `LATEX_COMMANDS` -- daftar statis perintah umum (struktur dokumen,
  formatting teks, matematika dasar, referensi/bibliografi), SENGAJA
  bukan daftar lengkap semua perintah/paket LaTeX (autocomplete yang
  sadar `\usepackage` di luar cakupan increment ini). Didaftarkan lewat
  `stexLanguage.data.of({autocomplete: latexCompletions})` -- cara resmi
  menambah completion source PER BAHASA di CodeMirror 6, BUKAN memanggil
  `autocompletion()` kedua kalinya (facet config-nya cuma mengambil
  pemanggilan PERTAMA yang sudah dilakukan `basicSetup`; pemanggilan
  kedua dengan `override` akan diam-diam DIABAIKAN, bukan digabung/
  menang). `@codemirror/autocomplete` ditambah eksplisit ke
  `package.json` (sebelumnya cuma dependency transitif lewat package
  `codemirror`).
  - Diverifikasi SEBAGIAN: `npx vite build`/`wails build` production
    sukses, screenshot aplikasi sungguhan menunjukkan editor tetap
    merender dan compile-otomatis-saat-startup tidak regresi (bukti
    tidak langsung construction-time wiring valid -- kalau
    `stexLanguage.data.of` gagal saat itu, `EditorView` gagal dibuat
    sama sekali). Popup autocomplete SENDIRI belum diverifikasi lewat
    ketikan sungguhan -- keterbatasan yang sama persis dengan
    verifikasi ketikan/klik yang sudah dicatat berulang kali di
    ROADMAP.md. Regex dan posisi replacement dikonfirmasi benar secara
    terpisah (`\sec` -> `\section`) lewat review manual sebelum commit.
  - **Ditemukan dan diperbaiki lewat code review sebelum commit**: draf
    awal `latexCompletions` menyalin guard `word.from === word.to &&
    !context.explicit` dari pola contoh resmi `@codemirror/autocomplete`
    (biasanya dipakai dengan regex yang boleh cocok string kosong, mis.
    `/\w*/`, di situ `context.explicit` berarti "Ctrl+Space walau belum
    ngetik apa-apa") -- TAPI regex di sini (`/\\[a-zA-Z]*/`) mewajibkan
    backslash LITERAL, jadi `matchBefore` cuma mengembalikan match
    non-null yang PANJANGNYA MINIMAL 1 (backslash-nya sendiri), tidak
    pernah zero-length. Guard itu jadi dead code -- kondisinya mustahil
    true, tidak salah secara perilaku (autocomplete tetap jalan normal),
    cuma komentarnya menyiratkan ada logic `context.explicit` yang
    sebenarnya tidak pernah tereksekusi. Dihapus, komentar fungsi ditulis
    ulang untuk menjelaskan kenapa `context.explicit` TIDAK relevan di
    sini, bukan cuma menghapus tanpa penjelasan.

- **Export PDF**: `app.go` menambah `ExportPDF(pdf []byte)` -- dialog
  native lokasi simpan, menulis lewat `atomicWriteFile` yang sama dengan
  `SaveDocument`, dilindungi `fileMu` yang sama juga. Menerima PDF
  LANGSUNG dari frontend (base64 apa adanya dari `Result.pdf` compile
  terakhir yang sukses, `[]byte` di parameter Go otomatis di-decode Wails
  dari string base64 JS lewat konvensi `encoding/json` standar) alih-alih
  membaca ulang `workDir/main.pdf` dari disk -- yang diekspor SELALU
  persis PDF yang sedang tampil di preview pane, tidak bergantung pada
  asumsi soal state file sementara di disk.
  - Tombol "Ekspor PDF" di toolbar (`frontend/index.html`), dinonaktifkan
    (atribut `disabled` + `refreshFileButtonsDisabled` di
    `frontend/src/main.js`) sampai ada compile yang sukses sama sekali
    (`lastPdfBase64`) -- beda dari tombol Baru/Buka/Simpan/Simpan sebagai
    yang cuma bergantung status `fileOperationInProgress`. Sama-sama ikut
    `withFileOperation` supaya tidak tumpang tindih dengan operasi file
    lain.
  - `app_test.go`: `TestExportPDF_EmptyRejectedBeforeDialog` -- penolakan
    PDF kosong dites hermetik (terjadi SEBELUM
    `wailsruntime.SaveFileDialog` yang butuh window OS sungguhan dan
    `a.ctx` terisi).
  - Diverifikasi: build Go + binding JS + `wails build` production
    sukses, screenshot aplikasi sungguhan menunjukkan tombol muncul di
    toolbar dan tampak aktif setelah compile pertama sukses. Klik
    tombolnya sendiri belum diverifikasi lewat interaksi sungguhan --
    keterbatasan yang sama dengan tombol Buka/Simpan (lihat catatan
    `SendKeys`/klik mouse terprogram di bawah).

- **Save/open file**: `app.go` menambah `OpenDocument` (dialog native,
  baca `.tex` terpilih ke memori), `SaveDocument` (tulis langsung ke path
  yang sudah diketahui, TANPA dialog), dan `SaveDocumentAs` (SELALU
  tampilkan dialog lokasi simpan) -- ketiganya dibind ke frontend lewat
  Wails, dilindungi `fileMu` (`sync.Mutex` BARU, terpisah dari `mu` yang
  dipakai `Compile`/`EnsureEngine` -- operasi file berdasar dialog tidak
  menyentuh `engineBin`/`workDir`, jadi membuka/menyimpan file tidak perlu
  menunggu kompilasi yang sedang berjalan sampai beberapa menit, lihat
  bagian Security untuk kenapa `fileMu` sendiri perlu ada).
  - Toolbar baru (Baru/Buka/Simpan/Simpan sebagai) di `frontend/index.html`
    + keyboard shortcut standar (Ctrl+O, Ctrl+S, Ctrl+Shift+S) di
    `frontend/src/main.js`.
  - Indikator "belum disimpan" (`•` di judul dokumen, `updateTitle`) --
    ditandai begitu editor berubah SETELAH dibuka/disimpan terakhir kali,
    dihapus lagi setelah simpan berikutnya sukses (lihat `docGeneration`
    di bagian Security untuk penanganan race saat operator mengetik
    SELAGI penyimpanan berlangsung). `suppressDirty` mencegah penggantian
    isi PROGRAMATIK (Buka/Baru mengganti seluruh isi editor lewat
    `view.dispatch`) sendiri keliru terhitung sebagai "perubahan operator"
    oleh `EditorView.updateListener`.
  - Konfirmasi `confirm()` native muncul kalau operator membuka file
    lain/membuat dokumen baru (`handleOpen`/`handleNew`) selagi ada
    perubahan belum disimpan -- SENGAJA belum ada perlindungan yang sama
    untuk MENUTUP APLIKASI (butuh `OnBeforeClose` di Go plus round-trip
    query status dirty ke frontend, dinilai layak jadi increment
    terpisah, lihat ROADMAP.md).
  - `app_test.go` (baru): unit test `SaveDocument` dan `atomicWriteFile`
    (lihat bagian Security) -- path kosong ditolak sebelum menyentuh
    disk, tulis berhasil, menimpa (bukan menambah/menggabung) file yang
    sudah ada, tidak ada file sementara tertinggal setelah sukses, gagal
    jelas kalau direktori tujuan tidak ada.

- Increment pertama latex-studio: editor LaTeX standalone (GUI Wails v2 --
  Go + WebView2, bukan Electron) dengan auto-recompile ke PDF.
  - `internal/tectonic`: bootstrap engine [Tectonic](https://tectonic-typesetting.github.io/)
    versi yang DIPIN (`tectonicVersion = "0.17.0"`, bukan "latest release"
    GitHub -- lihat bagian Security) -- deteksi platform (GOOS/GOARCH)
    untuk memilih asset rilis yang benar (`windows/amd64` ->
    `x86_64-pc-windows-msvc.zip`, `darwin/{amd64,arm64}` ->
    `*-apple-darwin.tar.gz`, `linux/{amd64,arm64}` ->
    `*-unknown-linux-{gnu,musl}.tar.gz`), unduh dengan batas ukuran dan
    timeout, verifikasi checksum SHA-256 terhadap nilai yang dipin
    (`tectonicChecksums`), ekstrak ke file sementara lalu `os.Rename`
    atomic ke cache lokal (`%LOCALAPPDATA%\latex-studio\bin` di Windows),
    dan eksekusi kompilasi (`Compile`) yang menulis source ke `main.tex`,
    menjalankan Tectonic, dan mengembalikan PDF + log gabungan
    stdout/stderr. Idempotent: panggilan `Ensure` setelah binary ada di
    cache cuma `os.Stat`, tidak menyentuh jaringan sama sekali -- inilah
    yang membuat kompilasi berikutnya bisa offline.
  - `app.go`: `EnsureEngine` (bootstrap + event progress ke frontend) dan
    `Compile` (source -> PDF+log) dibind ke frontend lewat Wails.
  - Editor CodeMirror 6 dengan syntax highlighting LaTeX
    (`@codemirror/legacy-modes/mode/stex`, dipilih karena paket
    `@codemirror/lang-latex` dedicated belum ada/matang di ekosistem
    CodeMirror 6 saat ini -- `legacy-modes` porting mode CodeMirror 5 yang
    sudah stabil lama), auto-recompile debounced ~900ms setelah editor
    berhenti berubah.
  - Preview PDF lewat `<iframe>` + `blob:` URL memakai penampil PDF bawaan
    WebView2 (Chromium) -- SENGAJA tidak menambah dependency PDF.js pada
    increment ini, penampil bawaan sudah cukup untuk preview sederhana
    (tanpa kontrol navigasi/anotasi custom).
  - Panel log ditampilkan otomatis kalau kompilasi LaTeX gagal (`Result.Ok
    == false`) -- dibedakan eksplisit dari error Go/infrastruktur (gagal
    tulis file, Tectonic tidak ditemukan, dst., yang jadi Go `error`
    biasa) supaya operator selalu lihat pesan error LaTeX yang sebenarnya,
    bukan "compile gagal" generik.
- Environment build disiapkan di mesin development: `gcc` (nuwen mingw
  15.2.0 lewat `scoop install main/gcc` -- Wails pakai cgo untuk binding
  WebView2, sebelumnya tidak ada compiler C sama sekali di PATH) dan Wails
  CLI v2.15.0 (`go install github.com/wailsapp/wails/v2/cmd/wails@latest`).
  `wails doctor` mengonfirmasi WebView2, Node 24, npm 11 sudah tersedia.

### Security

- **Ditemukan dan diperbaiki lewat code review sebelum commit pertama
  fitur save/open** -- dua temuan correctness plus satu kegagalan
  verifikasi build, semuanya pada draf awal `app.go`/`frontend/src/main.js`
  save/open, sebelum kode ini pernah di-commit:
  - **High -- perubahan baru bisa keliru dianggap sudah tersimpan**: draf
    awal `handleSave`/`handleSaveAs` (`frontend/src/main.js`) mengambil
    isi editor SEBELUM `await SaveDocument(...)`/`await
    SaveDocumentAs(...)`, lalu SELALU menjalankan `dirty = false` tanpa
    syarat setelah operasi selesai -- kalau operator mengetik LAGI selagi
    penyimpanan masih berlangsung (mis. file di network drive yang
    lambat), file di disk berisi snapshot LAMA, tapi indikator `•` di
    judul sudah hilang duluan, membuat operator mengira ketikan terbaru
    sudah aman padahal belum pernah tersimpan sama sekali. Diperbaiki
    lewat `docGeneration`, counter yang naik tiap ada perubahan operator
    sungguhan (`docChanged && !suppressDirty`) -- `handleSave`/
    `handleSaveAs` menyimpan nilainya SEBELUM `await`, dan cuma
    menjalankan `dirty = false` kalau `docGeneration` masih PERSIS sama
    setelah `await` selesai (berarti tidak ada ketikan baru di sela-sela
    penyimpanan).
  - **High -- operasi file paralel bisa selesai dengan urutan terbalik**:
    draf awal `OpenDocument`/`SaveDocument`/`SaveDocumentAs` tidak
    dilindungi mutex apa pun di backend, dan frontend juga tidak punya
    guard yang mencegah dua operasi file dipicu berdekatan (mis. klik
    tombol lalu buru-buru Ctrl+S) -- bisa menjalankan dua penyimpanan ke
    path yang sama, membuka beberapa dialog sekaligus, atau (paling
    berbahaya) membuat `currentPath`/status dirty di frontend ditentukan
    oleh Promise mana yang SELESAI paling akhir, bukan yang DIMULAI
    paling akhir (operasi lambat yang dipicu duluan bisa menimpa state
    dari operasi cepat yang dipicu belakangan). Diperbaiki DUA lapis,
    keduanya perlu (melindungi hal berbeda): `fileMu` (`sync.Mutex` baru
    di `App`, TERPISAH dari `mu` yang dipakai `Compile`/`EnsureEngine`,
    lihat komentar field-nya) menyerialkan urutan tulis-disk di backend;
    `fileOperationInProgress` plus `withFileOperation`
    (`frontend/src/main.js`) mencegah operasi file KEDUA dipicu sama
    sekali selagi satu masih berjalan di frontend (tombol toolbar juga
    dinonaktifkan sementara sebagai umpan balik visual) -- ini yang
    benar-benar mencegah masalah urutan Promise di atas, `fileMu`
    sendirian cuma menjamin tulis-disk tidak saling interleave tanpa
    menjamin urutan state frontend.
  - **Build lulus tapi `git diff --check` gagal**: binding TypeScript
    hasil `wails generate module`/`wails build`
    (`frontend/wailsjs/go/models.ts`, kadang juga `App.js`/`App.d.ts`)
    konsisten meninggalkan trailing whitespace (tab) di baris kosong
    template-nya -- klaim "diff check bersih" pada laporan sebelumnya
    TIDAK BENAR, belum pernah benar-benar dijalankan/diverifikasi.
    Dibersihkan manual (`sed 's/[ \t]*$//'`) dan DIKONFIRMASI MUNCUL LAGI
    di build berikutnya (bukan kejadian sekali doang) -- didokumentasikan
    di README.md sebagai langkah wajib sebelum commit tiap kali
    `frontend/wailsjs/` berubah, bukan diasumsikan sudah beres selamanya.
  - Catatan tambahan yang JUGA diperbaiki (ditandai non-blocking oleh
    reviewer, tapi murah dan langsung memperkuat jaminan "tidak
    kehilangan dokumen" yang jadi inti fitur ini): `SaveDocument`/
    `SaveDocumentAs` sekarang menulis lewat `atomicWriteFile` (file
    sementara di direktori yang sama -> `os.Rename`, pola yang sama
    dengan instalasi binary Tectonic) alih-alih `os.WriteFile` langsung
    ke path tujuan -- kegagalan di tengah penulisan (disk penuh, proses
    dibunuh paksa, koneksi network drive putus) tidak lagi bisa merusak
    dokumen yang sedang diedit, isi LAMA tetap utuh sampai penulisan BARU
    benar-benar selesai. `OpenDocument` sekarang menolak file lebih besar
    dari 50 MiB (`maxOpenFileBytes`) dan file yang isinya BUKAN UTF-8
    valid -- yang kedua bukan kosmetik: byte tidak valid UTF-8 yang
    diteruskan apa adanya akan diam-diam diganti `encoding/json` dengan
    U+FFFD saat dikirim ke frontend, dan kalau operator menyimpannya
    kembali tanpa sadar, file ASLI berenkoding lain (mis. Latin-1) akan
    tertimpa versi yang sudah rusak.
- **Ditemukan dan diperbaiki lewat code review sebelum commit pertama** --
  tiga temuan pada draf awal `internal/tectonic` dan `app.go`, sebelum
  kode ini pernah di-commit sama sekali:
  - **Critical -- risiko menghapus seluruh direktori temp OS**: draf awal
    `startup` (`app.go`) fallback ke `os.TempDir()` (root direktori temp
    milik SELURUH sistem, dipakai bersama proses/aplikasi lain) sebagai
    `workDir` kalau `os.MkdirTemp` gagal membuat direktori unik.
    `shutdown` menjalankan `os.RemoveAll(a.workDir)` tanpa syarat --
    kombinasi keduanya berarti kegagalan `MkdirTemp` (disk penuh,
    permission, dst., jarang tapi nyata) bisa membuat latex-studio
    menghapus SELURUH isi direktori temp OS saat ditutup, termasuk file
    milik aplikasi lain. Diperbaiki: fallback ke `os.TempDir()` DIHAPUS
    total -- kegagalan `MkdirTemp` sekarang disimpan sebagai
    `startupErr`, `EnsureEngine`/`Compile` menolak jalan sama sekali
    (fail closed) selama itu terisi, dan `shutdown` cuma menghapus
    `workDir` kalau field itu TERISI (yang sekarang cuma mungkin terjadi
    lewat `MkdirTemp` yang sukses membuat direktori unik).
  - **High -- timeout kompilasi lebih pendek dari waktu bootstrap nyata**:
    `compileTimeout` semula 2 menit, padahal pengujian manual sendiri
    mencatat compile PERTAMA (paket TeX belum ter-cache) butuh ~2m39s --
    berarti pengalaman pertama operator (justru yang paling penting untuk
    aplikasi yang mengklaim "standalone") kemungkinan besar SELALU
    diputus timeout sebelum sempat selesai, padahal compile ter-cache
    (yang kebetulan dipakai pada pengujian awal) selalu cepat sehingga
    masalah ini tidak kelihatan. Diperbaiki: `compileTimeout` dinaikkan
    ke 5 menit, dengan catatan eksplisit bahwa unduhan paket baru bisa
    terjadi kapan pun (bukan cuma compile pertama aplikasi -- kapan pun
    operator menambah `\usepackage` yang belum pernah dipakai). Juga
    ditambah `bootstrapTimeout` (10 menit) yang terpisah, khusus
    membungkus unduhan+ekstraksi BINARY Tectonic itu sendiri (beda
    operasi dari compile), plus `bootstrapHTTPClient` dengan `Timeout`
    eksplisit (`http.DefaultClient` yang dipakai draf awal TIDAK punya
    timeout sama sekali -- bisa menggantung selamanya kalau koneksi macet
    di tengah, bukan cuma gagal connect).
  - **High -- binary diunduh dan dijalankan tanpa verifikasi apa pun**:
    draf awal mengambil rilis "latest" dari GitHub API, mengunduh asset
    TANPA batas ukuran, mengekstrak LANGSUNG ke lokasi final tanpa
    verifikasi, lalu menjalankannya sebagai executable -- kalau unduhan
    terputus di tengah, binary PARSIAL bisa berakhir persis di lokasi
    cache; panggilan `Ensure` berikutnya cuma `os.Stat` (cek file ADA),
    bukan cek file itu UTUH, jadi instalasi rusak permanen sampai cache
    dihapus manual. Diperbaiki, empat lapis:
    1. **Versi dipin** (`tectonicVersion = "0.17.0"`, bukan "latest") --
       URL unduhan dibangun langsung dari versi+platform, TIDAK ada lagi
       panggilan GitHub API "latest release" sama sekali.
    2. **Checksum SHA-256 dipin** (`tectonicChecksums`, per platform) --
       diambil manual lewat `gh api .../releases/tags/tectonic@0.17.0
       --jq '.assets[].digest'` DAN di-cross-check terhadap `sha256sum`
       lokal atas file yang benar-benar diunduh (bukan dipercaya bulat-
       bulat dari satu sumber saja) sebelum ditulis ke source. `Ensure`
       menolak archive yang checksum-nya tidak cocok SEBELUM sempat
       diekstrak apalagi dijalankan (`verifySHA256`).
    3. **Batas ukuran unduhan** (`maxDownloadBytes`, 200 MiB) lewat
       `io.LimitReader` -- respons yang tidak wajar besarnya ditolak,
       bukan diam-diam mengunduh tanpa batas ke disk.
    4. **Ekstraksi atomic**: arsip diekstrak ke file SEMENTARA di
       direktori cache yang SAMA (bukan `os.TempDir()`, supaya
       `os.Rename` berikutnya tidak lintas volume) dulu, baru
       di-`os.Rename` ke lokasi final SETELAH checksum lolos dan
       ekstraksi sukses penuh -- `os.Rename` pada volume yang sama
       atomic baik di POSIX maupun Windows, jadi TIDAK ADA jalur kode
       yang bisa meninggalkan file parsial persis di lokasi yang nanti
       dipercaya `os.Stat` sebagai "sudah siap pakai". Ini sekaligus
       menjawab "validasi cached binary" -- keberadaan file di lokasi
       final SECARA KONSTRUKSI berarti dia utuh dan sudah lolos
       checksum, bukan asumsi yang dipercaya begitu saja tanpa dasar.
  - **Catatan lebih kecil, juga diperbaiki**: `App` (`app.go`) sekarang
    punya `sync.Mutex` yang menyerialkan `EnsureEngine`/`Compile` --
    draf awal tidak melindungi `engineBin`/`workDir` dari panggilan
    paralel, yang bisa membuat dua kompilasi saling menimpa
    `main.tex`/`main.pdf` milik satu sama lain di tengah proses Tectonic
    berjalan. Debounce di `frontend/src/main.js` sekarang membatalkan
    timer yang masih tertunda saat retry "compile ulang setelah compile
    aktif selesai" dipicu -- draf awal bisa memicu SATU compile tambahan
    yang redundan (bukan salah, cuma buang-buang kerja) kalau operator
    mengetik lagi tepat selagi compile sebelumnya masih berjalan.
  - Ditambah `internal/tectonic/tectonic_test.go`: test murni tanpa
    jaringan/Tectonic sungguhan untuk `tectonicChecksums` (format hex 64
    karakter, sinkron dengan platform yang `releaseTarget` dukung),
    `downloadURL`, dan `verifySHA256` (checksum cocok/tidak cocok/case-
    insensitive) -- bagian dari `Ensure`/`Compile` yang benar-benar
    bicara ke Tectonic/jaringan TETAP belum ada automated test (butuh
    Tectonic sungguhan, lihat ROADMAP.md), tapi logic keamanan yang baru
    ditambah ini sekarang tercakup.

### Testing

- **Manual, live, end-to-end** (automated test cuma mencakup logic murni
  bagian Security di atas -- `Ensure`/`Compile` yang bicara ke Tectonic
  sungguhan masih diverifikasi manual, lihat ROADMAP.md):
  - CLI Tectonic diuji manual dulu sebelum ditulis ke Go (`tectonic -p
    --outdir <dir> test.tex`) untuk memastikan flag dan nama file output
    (`<nama-input>.pdf`, BUKAN selalu `main.pdf` -- makanya `Compile`
    SENGAJA selalu menulis input sebagai `main.tex` supaya outputnya
    predictable `main.pdf`) sebelum diasumsikan di kode. Compile pertama
    (paket belum ter-cache) ~2m39s; compile kedua (paket sudah ter-cache)
    ~0.49s -- membuktikan klaim "offline setelah paket pertama kali
    dipakai" di README.md bukan asumsi kosong.
  - `wails build` production build berhasil menghasilkan
    `build/bin/latex-studio.exe`.
  - Aplikasi dijalankan sungguhan dan di-screenshot: **percobaan pertama**
    pakai `Graphics.CopyFromScreen` pada koordinat `GetWindowRect`
    ternyata menangkap window LAIN yang kebetulan menutupi area layar yang
    sama (`SetForegroundWindow` dari proses background gagal diam-diam,
    pembatasan standar Windows) -- screenshot yang dihasilkan sama sekali
    bukan latex-studio. Diperbaiki dengan `PrintWindow(hwnd, hdc,
    PW_RENDERFULLCONTENT)`, API yang secara eksplisit meminta window
    tertentu merender dirinya ke bitmap terlepas dari z-order/oklusi --
    screenshot kedua terbukti benar menampilkan latex-studio.
  - Screenshot yang benar menunjukkan: editor menampilkan dokumen default
    dengan syntax highlighting (command LaTeX hijau, math magenta),
    compile OTOMATIS berjalan saat aplikasi dibuka (tanpa aksi operator)
    dan PDF preview menampilkan title "Dokumen Baru" serta rumus
    `E = mc^2` ter-render dengan benar sebagai matematika (bukan teks
    mentah).
  - Percobaan memverifikasi jalur auto-recompile yang dipicu KETIKAN
    (lewat `SendKeys` dan COM `WScript.Shell.AppActivate`) TIDAK berhasil
    -- keystroke terprogram tidak sampai ke elemen contenteditable
    CodeMirror di dalam WebView2. Dicatat jujur sebagai belum
    diverifikasi lewat interaksi sungguhan di ROADMAP.md, bukan
    disembunyikan atau diklaim teruji.
  - **Setelah perbaikan bagian Security di atas**: `wails build` diulang,
    aplikasi dijalankan dan di-screenshot ulang (`PrintWindow`) -- fast
    path `Ensure` (binary Tectonic 0.17.0 yang sudah ter-cache dari
    pengujian sebelumnya, jadi konsisten dengan `tectonicVersion` yang
    baru dipin) tetap cuma `os.Stat` tanpa menyentuh jaringan, editor dan
    compile-otomatis-saat-startup terbukti masih bekerja identik dengan
    sebelum perbaikan -- perubahan keamanan/robustness ini TIDAK
    meregresi jalur yang sudah terverifikasi sebelumnya.
  - **Setelah menambah save/open file**: `wails build` diulang lagi,
    binding `OpenDocument`/`SaveDocument`/`SaveDocumentAs` dikonfirmasi
    ter-generate benar di `frontend/wailsjs/go/main/App.js` lewat `wails
    generate module`. Screenshot aplikasi sungguhan menunjukkan toolbar
    baru (Baru/Buka/Simpan/Simpan sebagai) tampil benar dengan judul
    "Dokumen baru".
  - Percobaan memverifikasi KLIK tombol Buka/Simpan lewat koordinat mouse
    terprogram (`mouse_event`+`SetCursorPos`) TIDAK berhasil dua kali
    berturut-turut -- percobaan pertama tanpa kesadaran DPI menghasilkan
    koordinat yang salah total (rasio ukuran window vs koordinat klik
    tidak masuk akal); diperbaiki dengan
    `SetProcessDpiAwarenessContext(DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2)`
    di skrip PowerShell (mengonfirmasi Windows scaling 125% adalah
    penyebabnya -- client rect sebelum/sesudah fix beda rasio ~1.25x
    persis), tapi percobaan KEDUA (koordinat proporsional dari
    screenshot sebelumnya) tetap salah sasaran -- klik yang dimaksudkan
    ke tombol "Buka" malah jatuh di area editor. Dihentikan di titik ini
    (bukan dikejar sampai berhasil) -- risiko dinilai rendah karena
    kode-nya straightforward (event listener standar -> method
    Wails-bound -> `wailsruntime.OpenFileDialog`/`SaveFileDialog`, API
    Wails yang umum dipakai, bukan logic custom yang rawan bug), dicatat
    jujur sebagai belum diverifikasi lewat klik sungguhan di ROADMAP.md,
    sama seperti keterbatasan `SendKeys` untuk ketikan sebelumnya.
    Verifikasi mendatang untuk KEDUANYA (ketikan dan klik) butuh driver
    berbasis WebView2 DevTools Protocol yang berinteraksi lewat DOM
    (`element.click()`, `dispatchEvent`), bukan input Win32 sintetis di
    level OS.
