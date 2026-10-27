# Credits

LaTeX Studio dibangun di atas karya orang lain. File ini mencatat library,
program, dan aset pihak lain yang dipakai, beserta pembuat dan lisensinya.
Repo ini belum punya file LICENSE sendiri.

Setiap kali menambah library, program, font, atau gambar dari pihak lain,
tambahkan juga ke file ini di PR yang sama.

Terakhir diperiksa: 27 Oktober 2026.

## Library Go (ikut di dalam `latex-studio.exe`)

Daftar dari `go list -deps ./...` (tanpa dependensi khusus tes); lisensi
dibaca dari file LICENSE di module cache. Toolchain: [Go](https://go.dev)
(BSD-3-Clause).

| Modul | Versi | Lisensi |
| --- | --- | --- |
| [github.com/wailsapp/wails/v2](https://github.com/wailsapp/wails) | v2.15.0 | MIT |
| [github.com/wailsapp/go-webview2](https://github.com/wailsapp/go-webview2) | v1.0.22 | MIT |
| [github.com/leaanthony/go-ansi-parser](https://github.com/leaanthony/go-ansi-parser) | v1.6.1 | MIT |
| [github.com/leaanthony/slicer](https://github.com/leaanthony/slicer) | v1.6.0 | MIT |
| [github.com/leaanthony/u](https://github.com/leaanthony/u) | v1.1.1 | MIT |
| [github.com/pkg/errors](https://github.com/pkg/errors) | v0.9.1 | BSD-2-Clause |
| [github.com/rivo/uniseg](https://github.com/rivo/uniseg) | v0.4.7 | MIT |
| [golang.org/x/sys](https://pkg.go.dev/golang.org/x/sys) | v0.46.0 | BSD-3-Clause |

## Frontend (npm)

[CodeMirror 6](https://codemirror.net/) (`codemirror`,
`@codemirror/autocomplete`, `@codemirror/legacy-modes`) oleh Marijn
Haverbeke dan kontributor -- MIT. Total 16 paket produksi, semuanya MIT.
[Vite](https://vitejs.dev/) (MIT) hanya untuk build.

## Program

- **[Tectonic](https://tectonic-typesetting.github.io/) 0.17.0** -- engine
  LaTeX, diunduh otomatis saat pertama dipakai (tidak ikut di repo). MIT.
  Paket LaTeX yang diunduh Tectonic dari bundel TeX Live punya lisensi
  masing-masing (umumnya LPPL).
- **Microsoft Edge WebView2 Runtime** -- komponen Windows untuk tampilan
  aplikasi dan preview PDF; tidak dibundel.

## Aset

- `build/appicon.png` dan `build/windows/icon.ico`: ikon bawaan template
  [Wails](https://wails.io/) (MIT), belum diganti.

Ada yang kurang atau salah? Buka issue atau PR.
