# Review 7 Issues — Laporan Lengkap

## Daftar Isi
1. [Issue 1 — Edit PC Cadangan Row/Col 0,0 + Label Tertukar](#issue-1--edit-pc-cadangan-rowcol-00--label-tertukar)
2. [Issue 2 — OS Input Fleksibel (Create/Edit/Filter)](#issue-2--os-input-fleksibel-createeditfilter)
3. [Issue 3 — Schedule List Grouped by Hari + Mata Kuliah](#issue-3--schedule-list-grouped-by-hari--mata-kuliah)
4. [Issue 4 — Users Page: Hapus View + Akun Utama Pindah](#issue-4--users-page-hapus-view--akun-utama-pindah)
5. [Issue 5 — Search/Filter/Sort Seragam + Entity/User Dynamic](#issue-5--searchfiltersort-seragam--entityuser-dynamic)
6. [Issue 6 — Nav-tabs Devices Border Tidak Visible](#issue-6--nav-tabs-devices-border-tidak-visible)
7. [Issue 7 — Label Filter/Sort Hilang + "Search" vs "Cari"](#issue-7--label-filtersort-hilang--search-vs-cari)
8. [Additional Findings](#additional-findings)

---

## Issue 1 — Edit PC Cadangan Row/Col 0,0 + Label Tertukar

### Problem
1. **`min="1"` pada column field** (`edit.html:29`): Cadangan punya `column=0`, tapi kolom input column punya `min="1"` — form tidak bisa submit untuk PC cadangan.
2. **Label Baris/Kolom tertukar**: Input `#column` (id="column", name="column") diberi label "Baris", tapi seharusnya "Kolom". Input `#row` (id="row", name="row") diberi label "Kolom", tapi seharusnya "Baris".
3. **`pc/list.html:98`** juga tertukar: `Baris {{ .Column }}, Kolom {{ .Row }}` — seharusnya `Baris {{ .Row }}, Kolom {{ .Column }}`.

### File
- `web/templates/pc/edit.html` line 26–36
- `web/templates/pc/list.html` line 98

### Current Code (edit.html)
```html
<div class="col-md-4 mb-3">
    <label for="column" class="form-label">Baris</label>  <!-- SALAH: label Baris untuk input column -->
    <input type="number" class="form-control" id="column" name="column" min="1" max="8" value="{{ .pc.Column }}">
    <small class="text-muted">1-8</small>
</div>
<div class="col-md-4 mb-3">
    <label for="row" class="form-label">Kolom</label>  <!-- SALAH: label Kolom untuk input row -->
    <input type="number" class="form-control" id="row" name="row" min="0" max="99" value="{{ .pc.Row }}">
    <small class="text-muted">0-99</small>
</div>
```

### Fix
1. `min="1"` → `min="0"` pada input column
2. Swap label: "Baris" → input `#row`, "Kolom" → input `#column`
3. Fix `pc/list.html:98`: `Baris {{ .Row }}, Kolom {{ .Column }}

---

## Issue 2 — OS Input Fleksibel (Create/Edit/Filter)

### Problem
1. **Create/Edit**: Input OS adalah `<input type="text">` tanpa datalist — user harus mengetik manual. Tidak ada dropdown saran.
2. **Filter (list)**: Dropdown OS hardcoded di HTML — hanya 3 opsi Windows. Tidak bisa filter Linux atau OS lain.
3. **Backend**: Tidak ada mekanisme untuk mendapatkan `SELECT DISTINCT operating_system` dari DB.

### File
- `web/templates/pc/create.html` line 179–195 (input)
- `web/templates/pc/list.html` line 49–57 (filter, hardcoded)
- `internal/handlers/pc.go` — `PCList`, `PCCreatePage`, `PCEditPage`

### Current Code (create.html)
```html
<input type="text" class="form-control" id="operating_system" name="operating_system"
    placeholder="Contoh: Windows 11 Pro 25H2" required />
<small class="text-muted">Contoh: Windows 11 Pro 25H2, Windows 10 Pro 23H2, Ubuntu 22.04 LTS</small>
```

### Current Code (list.html — filter)
```html
<select class="form-select" id="os" name="os">
    <option value="">Semua Versi</option>
    <option value="Windows 11 Pro 23H2">Windows 11 Pro 23H2</option>
    <option value="Windows 11 Pro 25H2">Windows 11 Pro 25H2</option>
    <option value="Windows 10 Pro 22H2">Windows 10 Pro 22H2</option>
</select>
```

### Fix
1. **Create/Edit**: Tambah `<input list="os-list">` + `<datalist id="os-list">` dengan opsi default + dynamic dari DB
2. **Filter**: Populate dari `SELECT DISTINCT operating_system` (backend query + template render)
3. **Handler** `PCCreatePage`, `PCEditPage`, `PCList`: pass `operatingSystems []string` ke template
4. **Repo/Service**: tambah method `GetDistinctOS()` (atau pakai query langsung)

---

## Issue 3 — Schedule List Grouped by Hari + Mata Kuliah

### Problem
- Schedule list adalah flat table — semua jadwal ditampilkan berurutan tanpa grouping.
- Tidak ada collapsible header berdasarkan Hari → Mata Kuliah.
- Tidak ada JS untuk grouping.

### File
- `web/templates/schedule/list.html`
- `web/static/js/main.js` (no schedule logic)
- `internal/handlers/schedule.go`

### Current Structure
```
[Day filter] [Search] [Sort by] [Filter] [Reset]
[Flat table: No | Day | Course | Class | Lecturer | Time Start | Time End | Action]
```

### Fix (Client-side JS grouping)
Goal: Group rows by Hari (collapsible, default collapsed if more than 1 jadwal di hari itu) → Mata Kuliah (collapsible, default expanded, collapse if >1 jadwal di MK yang sama).

Approach:
1. Backend: pastikan data sudah terurut (handler: `ORDER BY day_order, course_name, time_start`).
2. Inject JS di `schedule/list.html`:
   - Iterate `<tbody>` rows
   - Detect changes in `data-day` (hari) → inject group header `<tr class="group-header-day">`
   - Within same day, detect changes in `data-course` → inject sub-header `<tr class="group-header-course">`
   - Click header → toggle visibility of children rows
3. CSS: `group-header` dengan background, cursor pointer, icon collapse/expand.

---

## Issue 4 — Users Page: Hapus View Button + Akun Utama Pindah

### Problem
1. **View button** (`bi bi-eye`) ada di semua user — tidak informatif (hanya redirect ke detail yang datanya sama dengan tabel). User minta View dihapus untuk akun tertentu atau semua.
2. **"Akun utama"** badge saat ini berada di kolom **Aksi** (sebagai pengganti Edit/Delete untuk admin & rekan). User minta dipindah ke kolom **Nama Lengkap**.

### File
- `web/templates/user/list.html`
- `internal/handlers/user.go`

### Current Code
```html
<!-- Kolom Nama Lengkap (line 56) -->
<td>{{ .FullName }}</td>

<!-- Kolom Aksi (line 63-78) -->
<td class="text-nowrap">
    <a href="/admin/users/{{ .ID }}" class="btn btn-sm btn-info me-1" title="Detail">
        <i class="bi bi-eye"></i>
    </a>
    {{ if and (ne .Username "admin") (ne .Username "rekan") }}
    <a href="/admin/users/{{ .ID }}/edit" class="btn btn-sm btn-warning me-1" title="Edit">
        <i class="bi bi-pencil"></i>
    </a>
    <form method="POST" action="/admin/users/{{ .ID }}/delete" style="display: inline;"
          onsubmit="return confirm('Hapus user ini?')">
        <button type="submit" class="btn btn-sm btn-danger" title="Hapus">
            <i class="bi bi-trash"></i>
        </button>
    </form>
    {{ else }}
    <span class="text-muted small">Akun utama</span>
    {{ end }}
</td>
```

### Fix
1. **Hapus View button** (line 64-66) untuk SEMUA user (atau untuk akun utama saja — tergantung user preference)
2. **Pindah "Akun utama"**: di kolom Nama Lengkap, tambah `<span class="badge ...">Akun utama</span>` setelah `{{ .FullName }}` untuk user admin & rekan
3. Kolom Aksi: hapus View button, hanya Edit + Delete + conditional (atau hapus View sepenuhnya)

---

## Issue 5 — Search/Filter/Sort Seragam + Entity/User Dynamic

### Problem Overview
Beberapa halaman memiliki urutan filter yang tidak konsisten, ada yang missing filter/sort, entity_type dan user filter di activity_log masih hardcoded/dari sumber salah.

### 5A. Standard Order (seragam)
Semua list page harus mengikuti urutan:
```
[Search] → [Filter dropdowns] → [Sort By] → [Sort Order (if any)] → [Per Page (if any)] → [Filter button "Filter"] → [Reset button "Reset"]
```

### 5B. Per-Halaman Analysis

#### 1. device/list.html — SEARCH → Category → Condition → Sort By → Filter → Reset
- Search: NO label (hanya placeholder)
- Category: NO label
- Condition: NO label
- Sort By: NO label
- ✓ Order sudah benar

#### 2. device_installation/list.html — SEARCH → Sort By → Filter → Reset
- Search: NO label
- Sort By: NO label
- ✓ Order sudah benar

#### 3. device_usage/list.html — SEARCH → Sort By → Filter → Reset
- Search: NO label
- Sort By: NO label
- ✓ Order sudah benar

#### 4. device_loan/list.html — SEARCH → Status → Sort By → Filter → Reset
- Search: NO label
- Status: NO label
- Sort By: NO label
- ✓ Order sudah benar

#### 5. user/list.html — SEARCH → **Cari (button)** → Reset
- **Tidak ada filter** (role: admin/dosen/mahasiswa) → perlu ditambah
- **Tidak ada sort dropdown** → perlu ditambah (username, full_name, role, created_at)
- **Button text**: "Cari" → harus "Filter" (karena akan ada filter+sort tambahan)
- **Label search**: "Cari User" → konsisten dengan "Cari" saja

#### 6. activity_log/list.html — Date From → Date To → Action → Entity → User → Status → **SEARCH (di bawah!)** → Sort By → Filter
- **Search di posisi salah**: setelah semua filter, harusnya di atas
- **Entity filter hardcoded**: hanya `pc, device, logbook, auth` — tapi entityMap di export (`activity_log.go:100`) punya: `pc, device, software, logbook, user, auth, device_loan, device_usage, schedule` — banyak yang missing
- **User filter dari activity_logs**: `GetUsernames()` query `SELECT DISTINCT username FROM activity_logs` — user yang belum pernah aktivitas tidak muncul (termasuk "rekan" seed user)
- **Tidak ada reset button** — perlu ditambah
- **Label search**: "Search" → harus "Cari"
- **Order salah**: harusnya Search → Date From → Date To → Action → Entity → User → Status → Sort By → Filter → Reset

#### 7. logbook/list.html — Date From → Date To → Search → Sort By → Sort Order → Per Page → Filter → **Reset Filter**
- ✓ Order sudah benar
- Reset button teks: "Reset Filter" → harus "Reset" (konsisten dengan halaman lain)

#### 8. software/list.html — SEARCH → Category → Sort By → Filter → Reset
- Search: NO label
- Category: NO label
- Sort By: NO label
- ✓ Order sudah benar

#### 9. pc/list.html — SEARCH → Status → Placement → OS → Sort By → Filter → Reset
- ✓ Semua sudah benar (labels, order, button text)

#### 10. schedule/list.html — Day → SEARCH → Sort By → Filter → Reset
- Day: NO label
- Search: NO label
- Sort By: NO label
- Order: Search harus di atas (Day setelah Search)
- ✓ Tidak ada issue lain

### 5C. Dynamic Entity Filter — Source of Truth = DB, No Hardcode
**Prinsip**: Semua entity type harus dari `SELECT DISTINCT entity_type FROM activity_logs`. Tidak ada hardcode. Jika entity baru ditambahkan di masa depan, filter akan otomatis mencakupnya.

- **Backend**: Tambah method `GetAllEntityTypes() []string` di `ActivityLogService` → query `SELECT DISTINCT entity_type FROM activity_logs`
- **Handler**: panggil method, pass ke template sebagai `.entityTypes`
- **Template**: range `.entityTypes` untuk dropdown filter — tidak ada `<option>` hardcode sama sekali

**Tambahan — entityMap di export harus lengkap**: Saat ini `activity_log.go:100` missing `"device_installation"`. EntityMap harus mencakup SEMUA entity yang mungkin muncul:
```
pc, device, software, logbook, user, auth, device_loan, device_usage, device_installation, schedule
```
Tambah `"device_installation": "Device Installation"` ke entityMap.

### 5D. Dynamic User Filter — UNION (Activity Logs + Users Table)
**Keputusan (disetujui user)**: Opsi A — UNION approach.

- **Backend**: Tambah method `GetAllUsernames() []string` di `ActivityLogService`:
  ```sql
  SELECT DISTINCT username FROM activity_logs
  UNION
  SELECT username FROM users
  ```
  Query ini mencakup:
  - Semua user yang pernah punya aktivitas (termasuk yang sudah dihapus dari users table tapi masih punya log)
  - Semua user yang terdaftar di users table (termasuk yang belum pernah login/aktivitas, seperti seed user "rekan")
- **Handler**: panggil `GetAllUsernames()` (ganti dari `GetUsernames()`)
- **Template**: range `.usernames` — sama seperti sekarang (tidak perlu ubah HTML, hanya sumber datanya yang berubah)

### Files affected
- `internal/services/activity_log.go` — tambah `GetEntityTypes()`
- `internal/handlers/activity_log.go` — panggil dynamic entity types + user dari users table
- `internal/handlers/user.go` — tambah filter role + sort
- `web/templates/activity_log/list.html` — reorder, dynamic entity/user, tambah reset
- `web/templates/user/list.html` — tambah filter role + sort dropdown
- `web/templates/logbook/list.html` — "Reset Filter" → "Reset"
- `web/templates/schedule/list.html` — reorder (Search di atas Day)

---

## Issue 6 — Nav-tabs Devices Border Tidak Visible

### Problem
Navigasi tab di halaman Devices (4 tabs: Perangkat, Peminjaman, Pemakaian, Instalasi) memiliki struktur:

```html
<ul class="nav nav-tabs mb-4">...</ul>
{{ template "layout/alert.html" . }}
<div class="card shadow-sm">
    <div class="card-body">...</div>
</div>
```

Ada `alert` di antara `nav-tabs` dan `card`, sehingga:
- Bottom border `nav-tabs` tidak terhubung visual dengan card
- Tampak seperti tab "mengambang" terpisah dari konten

### File
- `web/templates/device/_tab_nav.html`
- `web/templates/device/list.html`
- `web/templates/device_installation/list.html`
- `web/templates/device_usage/list.html`
- `web/templates/device_loan/list.html`
- `web/static/css/style.css`

### Fix Options
**Option A — CSS fix (minimal):**
```css
.nav-tabs { border-bottom: 1px solid #dee2e6; }
.nav-tabs ~ .card.shadow-sm {
    border-top-left-radius: 0;
    border-top-right-radius: 0;
    border-top: none;
}
```

**Option B — Restructure:**
Pindahkan `card` di luar dan gunakan `card-header-tabs`:
```html
<div class="card shadow-sm">
    <div class="card-header p-0">
        <ul class="nav nav-tabs card-header-tabs">...</ul>
    </div>
    {{ template "layout/alert.html" . }}
    <div class="card-body">...</div>
</div>
```
(Alert pindah ke dalam card, muncul setelah header tabs.)

---

## Issue 7 — Label Filter/Sort Hilang + "Search" vs "Cari"

### Problem
Beberapa halaman tidak memiliki label pada search/filter/sort input, dan ada inkonsistensi "Search" vs "Cari".

### 7A. Missing Labels

| Page | Element | Missing Label |
|------|---------|---------------|
| device/list | Search | ✓ Missing |
| device/list | Category filter | ✓ Missing |
| device/list | Condition filter | ✓ Missing |
| device/list | Sort By | ✓ Missing |
| device_installation/list | Search | ✓ Missing |
| device_installation/list | Sort By | ✓ Missing |
| device_usage/list | Search | ✓ Missing |
| device_usage/list | Sort By | ✓ Missing |
| device_loan/list | Search | ✓ Missing |
| device_loan/list | Status filter | ✓ Missing |
| device_loan/list | Sort By | ✓ Missing |
| schedule/list | Day filter | ✓ Missing |
| schedule/list | Search | ✓ Missing |
| schedule/list | Sort By | ✓ Missing |
| software/list | Search | ✓ Missing |
| software/list | Category filter | ✓ Missing |
| software/list | Sort By | ✓ Missing |

### 7B. "Search" vs "Cari"

| Page | Current Text | Should Be |
|------|-------------|-----------|
| activity_log/list.html | Label: "Search" | "Cari" |
| user/list.html | Label: "Cari User" | "Cari" (konsisten) |
| user/list.html | Button: "Cari" | "Filter" (karena ada filter+sort nantinya) |

### 7C. Button/Text Inconsistencies

| Page | Current Text | Should Be |
|------|-------------|-----------|
| logbook/list.html | "Reset Filter" | "Reset" |

### Fix
- Tambah `<label for="...">Cari</label>` untuk search input di halaman yang missing
- Tambah label pada filter dropdowns: e.g., "Kategori", "Kondisi", "Status", "Urutkan Berdasarkan"
- Ganti "Search" → "Cari" di activity_log/list.html
- Ganti "Reset Filter" → "Reset" di logbook/list.html
- Ganti "Cari User" → "Cari" di user/list.html

---

## Additional Findings

1. **`pc/list.html:98` — Label Baris/Kolom tertukar**: `Baris {{ .Column }}, Kolom {{ .Row }}` harus dibalik. (Related to Issue 1)

2. **Entity map di export (`activity_log.go:100`) missing `device_installation`**: Map export = `pc, device, software, logbook, user, auth, device_loan, device_usage, schedule` — tapi `device_installation` tidak ada padahal entity ini sudah digunakan di log untuk operasi instalasi perangkat. (Related to Issue 5C)

   **Implikasi**: Prinsip "source of truth = database" berarti entity filter harus 100% dinamis dari `SELECT DISTINCT entity_type`. Hardcode → rawan inkonsistensi seperti ini. Dengan filter dinamis, `device_installation` akan otomatis muncul begitu ada log dengan entity_type tersebut.

3. **`GetUsernames()` dari `activity_logs` table**: Hanya return user yang pernah melakukan aktivitas. User seed seperti "rekan" tidak muncul jika belum login/melakukan aksi. **Fix**: UNION dengan `users` table — `GetAllUsernames()` mencakup kedua sumber. (Related to Issue 5D)

4. **User list handler tidak punya filter selain search**: Tidak ada `role` filter, tidak ada `sort_by`/`sort_order`. (Related to Issue 5B-5)

5. **Activity log tidak punya reset button**: Semua list page lain punya reset link, activity_log tidak. (Related to Issue 5B-6)

---

## Summary Files Affected

| File | Issues |
|------|--------|
| `web/templates/pc/edit.html` | #1 |
| `web/templates/pc/list.html` | #1 (label swapped) |
| `web/templates/pc/create.html` | #2 |
| `internal/services/activity_log.go` | #5 (tambah GetEntityTypes) |
| `internal/handlers/activity_log.go` | #5 (entity/user dynamic + reorder) |
| `internal/handlers/user.go` | #4, #5 (tambah filter role + sort) |
| `internal/handlers/pc.go` | #2 (pass OS list ke template) |
| `web/templates/activity_log/list.html` | #5, #7 |
| `web/templates/user/list.html` | #4, #5, #7 |
| `web/templates/logbook/list.html` | #5, #7 |
| `web/templates/schedule/list.html` | #3, #5, #7 |
| `web/templates/device/list.html` | #7 |
| `web/templates/device_installation/list.html` | #7 |
| `web/templates/device_usage/list.html` | #7 |
| `web/templates/device_loan/list.html` | #7 |
| `web/templates/software/list.html` | #7 |
| `web/templates/device/_tab_nav.html` | #6 |
| `web/static/css/style.css` | #6 |
| `web/static/js/main.js` | #3 (tambah schedule grouping) |

---

## Link to Relevant Files

| File (relative to `poc_prototype/`) | Issues |
|----------|--------|
| `web/templates/pc/edit.html` | #1 |
| `web/templates/pc/list.html` | #1, #7 |
| `web/templates/pc/create.html` | #2 |
| `web/templates/pc/list.html` | #2 |
| `web/templates/schedule/list.html` | #3, #5, #7 |
| `web/static/js/main.js` | #3 |
| `web/templates/user/list.html` | #4, #5, #7 |
| `internal/handlers/user.go` | #4, #5 |
| `internal/handlers/activity_log.go` | #5 |
| `internal/services/activity_log.go` | #5 |
| `internal/handlers/pc.go` | #2 |
| `web/templates/activity_log/list.html` | #5, #7 |
| `web/templates/logbook/list.html` | #5, #7 |
| `web/templates/device/_tab_nav.html` | #6 |
| `web/templates/device/list.html` | #6, #7 |
| `web/templates/device_installation/list.html` | #6, #7 |
| `web/templates/device_usage/list.html` | #6, #7 |
| `web/templates/device_loan/list.html` | #6, #7 |
| `web/templates/software/list.html` | #7 |
| `web/static/css/style.css` | #6 |

---

## Complexity Estimate

| Issue | Complexity | Files Changed | Risk |
|-------|-----------|---------------|------|
| #1 | Easy | 2 templates | Low |
| #2 | Medium | 3 templates + 1 handler + maybe repo/service | Low |
| #3 | Medium | 1 template + JS | Low |
| #4 | Easy | 1 template | Low |
| #5 | **High** | 6-8 templates + 2 handlers + 1 service | Medium (ordering, entity/user) |
| #6 | Easy | 1 CSS + 5 templates (restructure) | Low |
| #7 | Easy-Medium | 7+ templates | Low |

**Total**: Mostly low-risk template changes, with Issue #5 being the most complex due to backend changes (entity types, user source switching, sort/filter additions).
