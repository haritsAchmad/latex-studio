import './style.css'
import {EditorView, basicSetup} from 'codemirror'
import {StreamLanguage} from '@codemirror/language'
import {stex} from '@codemirror/legacy-modes/mode/stex'
import {Compile, EnsureEngine, OpenDocument, SaveDocument, SaveDocumentAs} from '../wailsjs/go/main/App'
import {EventsOn} from '../wailsjs/runtime/runtime'

const DEBOUNCE_MS = 900

const DEFAULT_SOURCE = `\\documentclass{article}
\\title{Dokumen Baru}
\\author{}
\\date{}

\\begin{document}
\\maketitle

Halo, \\LaTeX!

$E = mc^2$

\\end{document}
`

const titleEl = document.getElementById('app-title')
const statusEl = document.getElementById('status')
const pdfFrame = document.getElementById('pdf-frame')
const logPane = document.getElementById('log-pane')

let currentPdfUrl = null
let debounceTimer = null
let compiling = false
// pendingAfterCurrent SENGAJA cuma boolean, bukan antrean isi -- kalau ada
// perubahan lagi SELAGI compile berjalan, kompilasi berikutnya cukup
// membaca isi editor TERKINI (view.state.doc) saat dia jalan, bukan isi
// pada saat perubahan itu terjadi. Menumpuk banyak compile job sekaligus
// cuma buang waktu untuk hasil yang toh akan ditimpa oleh compile
// berikutnya yang lebih baru.
let pendingAfterCurrent = false

// currentPath null berarti dokumen belum pernah disimpan/dibuka dari disk
// -- "Simpan" pada kondisi ini berperilaku sama dengan "Simpan sebagai"
// (lihat handleSave). dirty melacak ada tidaknya perubahan SEJAK terakhir
// dibuka/disimpan, ditampilkan lewat titik di judul (updateTitle) supaya
// operator tahu kapan perlu menyimpan -- TIDAK ada pencegahan menutup
// aplikasi dengan perubahan belum tersimpan pada increment ini (beda
// fitur, butuh intersepsi window-close yang lebih rumit, lihat
// ROADMAP.md), indikator ini murni visual.
let currentPath = null
let dirty = false
// suppressDirty dipasang true SEBENTAR selagi loadContent mengganti isi
// dokumen secara programatik (Buka/Baru) -- tanpa ini, penggantian isi
// itu sendiri akan terhitung sebagai "perubahan operator" oleh
// updateListener dan keliru menandai dokumen yang BARU SAJA dibuka dari
// disk sebagai "belum disimpan".
let suppressDirty = false
// docGeneration dinaikkan setiap kali ADA perubahan operator sungguhan
// (docChanged && !suppressDirty) -- dipakai handleSave/handleSaveAs untuk
// tahu apakah editor masih berisi PERSIS isi yang baru saja ditulis ke
// disk begitu await SaveDocument/SaveDocumentAs selesai. Tanpa ini: kalau
// operator mengetik LAGI selagi penyimpanan masih berlangsung (mis. file
// di network drive yang lambat), snapshot yang ditulis ke disk sudah basi
// duluan sebelum tulisnya selesai, tapi kode yang cuma `dirty = false`
// tanpa syarat akan menghapus indikator "belum disimpan" walau ketikan
// TERBARU itu belum pernah tersimpan sama sekali -- operator bisa
// mengira semuanya sudah aman padahal tidak. Ditemukan lewat code review
// sebelum commit pertama fitur save/open ini.
let docGeneration = 0
// fileOperationInProgress mencegah Buka/Simpan/Simpan sebagai/Baru
// SALING TUMPANG TINDIH -- tanpa guard ini, dua shortcut/tombol yang
// dipicu berdekatan bisa membuka beberapa dialog sekaligus, menjalankan
// dua penyimpanan ke path yang sama, atau (kasus paling berbahaya)
// membuat currentPath/dirty akhir ditentukan oleh Promise mana yang
// SELESAI paling akhir alih-alih yang DIMULAI paling akhir -- operasi
// lambat (mis. tersimpan) mengembalikan hasil belakangan meski dipicu
// duluan, override state dari operasi yang sebenarnya lebih baru.
// Backend (app.go, fileMu) menyerialkan urutan tulis-disknya, tapi cuma
// guard di sini yang mencegah state currentPath/dirty di FRONTEND jadi
// tidak konsisten. Ditemukan lewat code review yang sama dengan
// docGeneration di atas.
let fileOperationInProgress = false

function setStatus(text) {
    statusEl.textContent = text
}

function showLog(text) {
    if (!text) {
        logPane.hidden = true
        logPane.textContent = ''
        return
    }
    logPane.hidden = false
    logPane.textContent = text
}

function fileNameOf(path) {
    if (!path) {
        return 'Dokumen baru'
    }
    const parts = path.split(/[\\/]/)
    return parts[parts.length - 1]
}

function updateTitle() {
    const name = fileNameOf(currentPath)
    titleEl.textContent = dirty ? `${name} •` : name
    titleEl.title = currentPath || '(belum pernah disimpan)'
}

// b64ToBlobUrl mengubah string base64 (field `pdf` pada tectonic.Result --
// Wails otomatis meng-encode []byte Go jadi base64 saat di-marshal ke JSON)
// jadi blob: URL supaya bisa ditampilkan lewat <iframe>, memakai penampil
// PDF bawaan WebView2 (Chromium) -- tidak butuh library PDF.js terpisah
// untuk increment ini.
function b64ToBlobUrl(b64) {
    const binary = atob(b64)
    const bytes = new Uint8Array(binary.length)
    for (let i = 0; i < binary.length; i++) {
        bytes[i] = binary.charCodeAt(i)
    }
    return URL.createObjectURL(new Blob([bytes], {type: 'application/pdf'}))
}

async function runCompile(view) {
    if (compiling) {
        pendingAfterCurrent = true
        return
    }
    compiling = true
    setStatus('mengompilasi...')
    try {
        const source = view.state.doc.toString()
        const result = await Compile(source)
        if (result.ok && result.pdf) {
            const url = b64ToBlobUrl(result.pdf)
            pdfFrame.src = url
            if (currentPdfUrl) {
                URL.revokeObjectURL(currentPdfUrl)
            }
            currentPdfUrl = url
            showLog('')
            setStatus('sukses')
        } else {
            showLog(result.log || '(tidak ada log)')
            setStatus('gagal compile -- lihat log di bawah preview')
        }
    } catch (err) {
        showLog(String(err))
        setStatus('error')
    } finally {
        compiling = false
        if (pendingAfterCurrent) {
            pendingAfterCurrent = false
            // Batalkan debounce timer yang mungkin MASIH tertunda dari
            // keystroke yang terjadi selagi compile ini berjalan --
            // tanpa ini, timer itu bisa tetap menyala dan memicu SATU
            // compile tambahan yang redundan beberapa saat setelah retry
            // di bawah ini selesai, walau kedua-duanya cuma mengompilasi
            // isi editor yang sama (tidak ada keystroke baru di antara
            // keduanya). Aman dibatalkan karena runCompile SELALU membaca
            // view.state.doc TERKINI (bukan snapshot), jadi retry di
            // bawah ini sudah mencakup perubahan yang tadinya mau dipicu
            // oleh timer itu.
            if (debounceTimer) {
                clearTimeout(debounceTimer)
                debounceTimer = null
            }
            runCompile(view)
        }
    }
}

function scheduleCompile(view) {
    if (debounceTimer) {
        clearTimeout(debounceTimer)
    }
    debounceTimer = setTimeout(() => {
        debounceTimer = null
        runCompile(view)
    }, DEBOUNCE_MS)
}

const view = new EditorView({
    doc: DEFAULT_SOURCE,
    extensions: [
        basicSetup,
        StreamLanguage.define(stex),
        EditorView.updateListener.of((update) => {
            if (update.docChanged) {
                if (!suppressDirty) {
                    docGeneration++
                    dirty = true
                    updateTitle()
                }
                scheduleCompile(view)
            }
        }),
    ],
    parent: document.getElementById('editor-pane'),
})

// loadContent mengganti SELURUH isi editor secara programatik (dipakai
// handleOpen dan handleNew) -- lihat suppressDirty untuk kenapa
// penggantian ini sendiri tidak ikut menandai dokumen sebagai dirty.
function loadContent(text, path) {
    suppressDirty = true
    view.dispatch({changes: {from: 0, to: view.state.doc.length, insert: text}})
    suppressDirty = false
    currentPath = path
    dirty = false
    updateTitle()
}

function confirmDiscardIfDirty(message) {
    if (!dirty) {
        return true
    }
    return confirm(message)
}

const fileButtons = [
    document.getElementById('btn-new'),
    document.getElementById('btn-open'),
    document.getElementById('btn-save'),
    document.getElementById('btn-save-as'),
]

// withFileOperation menyerialkan Buka/Simpan/Simpan sebagai/Baru DI SISI
// FRONTEND -- lihat komentar fileOperationInProgress. Tombol toolbar
// dinonaktifkan sementara juga supaya operator dapat umpan balik visual
// (bukan cuma klik/shortcut yang diam-diam diabaikan) selagi satu operasi
// file sedang berlangsung.
async function withFileOperation(fn) {
    if (fileOperationInProgress) {
        setStatus('operasi file lain sedang berjalan, tunggu sebentar...')
        return
    }
    fileOperationInProgress = true
    fileButtons.forEach((b) => (b.disabled = true))
    try {
        await fn()
    } finally {
        fileOperationInProgress = false
        fileButtons.forEach((b) => (b.disabled = false))
    }
}

async function handleOpen() {
    await withFileOperation(async () => {
        if (!confirmDiscardIfDirty('Ada perubahan belum disimpan. Buka file lain dan buang perubahan ini?')) {
            return
        }
        try {
            const result = await OpenDocument()
            if (result.cancelled) {
                return
            }
            loadContent(result.content, result.path)
            runCompile(view)
        } catch (err) {
            showLog(String(err))
            setStatus('gagal membuka file')
        }
    })
}

async function handleSave() {
    if (!currentPath) {
        return handleSaveAs()
    }
    await withFileOperation(async () => {
        // Snapshot generation SEBELUM await -- lihat komentar docGeneration
        // untuk kenapa ini wajib, bukan sekadar `dirty = false` tanpa
        // syarat setelah save selesai.
        const savedGeneration = docGeneration
        const content = view.state.doc.toString()
        try {
            await SaveDocument(currentPath, content)
            if (docGeneration === savedGeneration) {
                dirty = false
            }
            updateTitle()
            setStatus('tersimpan')
        } catch (err) {
            showLog(String(err))
            setStatus('gagal menyimpan')
        }
    })
}

async function handleSaveAs() {
    await withFileOperation(async () => {
        const savedGeneration = docGeneration
        const content = view.state.doc.toString()
        try {
            const result = await SaveDocumentAs(content)
            if (result.cancelled) {
                return
            }
            currentPath = result.path
            if (docGeneration === savedGeneration) {
                dirty = false
            }
            updateTitle()
            setStatus('tersimpan')
        } catch (err) {
            showLog(String(err))
            setStatus('gagal menyimpan')
        }
    })
}

async function handleNew() {
    await withFileOperation(async () => {
        if (!confirmDiscardIfDirty('Ada perubahan belum disimpan. Buat dokumen baru dan buang perubahan ini?')) {
            return
        }
        loadContent(DEFAULT_SOURCE, null)
        runCompile(view)
    })
}

document.getElementById('btn-new').addEventListener('click', handleNew)
document.getElementById('btn-open').addEventListener('click', handleOpen)
document.getElementById('btn-save').addEventListener('click', handleSave)
document.getElementById('btn-save-as').addEventListener('click', handleSaveAs)

window.addEventListener('keydown', (e) => {
    const mod = e.ctrlKey || e.metaKey
    if (!mod) {
        return
    }
    const key = e.key.toLowerCase()
    if (key === 's' && e.shiftKey) {
        e.preventDefault()
        handleSaveAs()
    } else if (key === 's') {
        e.preventDefault()
        handleSave()
    } else if (key === 'o') {
        e.preventDefault()
        handleOpen()
    }
})

EventsOn('engine:progress', (stage) => {
    setStatus(stage)
})

updateTitle()
setStatus('menyiapkan engine Tectonic (unduhan pertama bisa beberapa menit)...')
EnsureEngine()
    .then(() => runCompile(view))
    .catch((err) => {
        setStatus('gagal menyiapkan engine')
        showLog(String(err))
    })
