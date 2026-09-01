import './style.css'
import {EditorView, basicSetup} from 'codemirror'
import {StreamLanguage} from '@codemirror/language'
import {stex} from '@codemirror/legacy-modes/mode/stex'
import {Compile, EnsureEngine} from '../wailsjs/go/main/App'
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
                scheduleCompile(view)
            }
        }),
    ],
    parent: document.getElementById('editor-pane'),
})

EventsOn('engine:progress', (stage) => {
    setStatus(stage)
})

setStatus('menyiapkan engine Tectonic (unduhan pertama bisa beberapa menit)...')
EnsureEngine()
    .then(() => runCompile(view))
    .catch((err) => {
        setStatus('gagal menyiapkan engine')
        showLog(String(err))
    })
