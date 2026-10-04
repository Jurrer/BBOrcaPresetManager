# BBOrcaProfsync — Design Plan

A local web application for **manually synchronizing print profiles** between
BambuStudio and OrcaSlicer on the same machine. No automatic syncing — every
write is a deliberate, on-screen action. A local git **vault** records a
timeline of every change so you can review and roll back.

Written in Go (single static binary, embedded web UI, no runtime dependencies).

---

## 1. Settled decisions

These were resolved during planning and are not open questions:

| Decision | Resolution |
|---|---|
| Deliverable | This `plan.md` — a build-ready spec for an agent to scaffold from. |
| Interface | Web UI only. No CLI. |
| Sync model | **Manual only.** No background/automatic sync. Every write is user-driven. |
| Sync granularity | **Per-field selective.** The diff shows source vs dest values per field; the user picks which value wins (or enters a custom value) for each field. |
| Change tracking | **Vault git repo** — a dedicated repo (in app data dir or user-chosen path) that mirrors profile files. Each sync stages + commits. Rollback restores from vault. |
| Profile matching | **Auto-suggest by name, with manual override.** `setting_id` flags "already synced" but isn't the primary key (formats differ across slicers). |
| Category scope | **All three** (machine, process, filament) from the start — uniform file format. |
| Config | **Auto-detect + config file + UI Settings.** Zero-config on first launch; `~/.bborcaprofsync.yaml` for persistent overrides; Settings tab for interactive changes. |
| Tests | **Unit + integration.** Pure-function tests for transform/diff; round-trip tests with real fixture profiles; API handler tests. |
| Canonical term | **"profile"** for all three categories. "preset" is dropped to avoid ambiguity. |
| `plan.md` / `AGENTS.md` split | `plan.md` = this design spec. `AGENTS.md` = agent operating rules only. |

---

## 2. Domain model

See `GLOSSARY.md` for canonical terms. Summary:

- **Profile** — a single user-created configuration file pair (`.json` + `.info`) in one of three categories:
  - **Machine profile** (`machine/`) — printer settings per model + nozzle.
  - **Process profile** (`process/`) — print settings: layer height, speeds, infill, support.
  - **Filament profile** (`filament/`) — filament temperature / cooling per material.
- **Setting field** — a key→value pair in the `.json` that is a tunable value (e.g. `inner_wall_speed`). Selectable for per-field sync.
- **Identity fields** — `name`, `print_settings_id` / `machine_id` / `filament_id`, `from`. Shown as context, not selectable. Syncing these would break profile identity.
- **Lineage fields** — `inherits` (JSON) names the system profile; `base_id` (`.info`) is that system profile's `setting_id`. Auto-remapped on write.
- **`setting_id`** — account-scoped profile identifier in `.info`. Format differs: BambuStudio uses `PPUS` + hex; OrcaSlicer native accounts use UUID.
- **Vault** — the app's git repo that mirrors profile files and records every sync as a commit.
- **Sync** — a manual, per-field write operation driven by the UI. Never automatic.

### Observed format divergences (from real `0.2 base` pair)

1. **Array vs scalar**: BS wraps many speed values in arrays (`"inner_wall_speed": ["400"]`); Orca uses scalars (`"inner_wall_speed": "400"`). But some fields are arrays in both (`print_extruder_id`, `print_extruder_variant`). Field-specific, not a blanket rule.
2. **Value divergence**: `top_surface_pattern` is `"zig-zag"` in BS, `"rectilinear"` in Orca — same profile name, different content. Exactly what the diff must surface.
3. **`setting_id` format**: BS = `PPUS249810809cffcb`; Orca native = UUID. Numeric Orca user dir is a hand-copy from BS (same `setting_id`, same `updated_time`).
4. **`base_id` matches** (`GP079`) for this pair. May not hold universally — see transform rules.
5. **`version` matches** (`1.10.0.32`) for this pair.

### Profile counts on this machine

| Slicer / user dir | machine | process | filament |
|---|---|---|---|
| BambuStudio / `2240525152` | 0 | 6 | 41 |
| OrcaSlicer / `11b99db1-…` (UUID, native) | 0 | 7 | 38 |
| OrcaSlicer / `2240525152` (numeric, BS copy) | 0 | 7 | 34 |

No user machine profiles exist (machines are system-only here). The engine handles all three categories regardless; an empty category just shows nothing to sync.

---

## 3. Architecture

```
┌──────────────────────────────────────────────────┐
│  Go HTTP server (net/http or chi)                 │
│  ┌──────────────┐  ┌──────────────────────────┐  │
│  │ REST API     │  │ Embedded web UI (embed)   │  │
│  │ /api/*       │  │ single-page, tabbed       │  │
│  └──────┬───────┘  └──────────────────────────┘  │
│         │                                        │
│  ┌──────▼────────────────────────────────────┐  │
│  │ Sync engine                                │  │
│  │  - profile discovery + listing             │  │
│  │  - field-level diff (format-normalized)    │  │
│  │  - per-field apply + transform             │  │
│  └──────┬────────────────────────────────────┘  │
│         │                                        │
│  ┌──────▼────────────────────────────────────┐  │
│  │ Vault (git)                                │  │
│  │  - mirror profile files                    │  │
│  │  - stage + commit per sync                 │  │
│  │  - history + rollback                      │  │
│  └──────┬────────────────────────────────────┘  │
│         │                                        │
│  ┌──────▼────────────────────────────────────┐  │
│  │ Slicer adapters (BambuStudio, OrcaSlicer)   │  │
│  │  - default paths per OS                     │  │
│  │  - read/write JSON + .info                   │  │
│  │  - field-type registry (array/scalar)       │  │
│  └────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────┘
```

### Core components

- **`slicer`** — `SlicerAdapter` interface + BambuStudio/OrcaSlicer implementations. Knows default profile dir per OS, reads/writes JSON + `.info`, and carries the field-type registry (which fields are array-typed vs scalar-typed in that slicer).
- **`profile`** — `Profile`, `Info`, `Category`, `Field` types. Parse + normalize on read; denormalize + write.
- **`diff`** — field-level comparison with format normalization (treats `["400"]` and `"400"` as equal). Returns per-field: source value, dest value, status (match/differ/only-source/only-dest), selectable flag.
- **`sync`** — applies a set of selected field values to a dest profile, runs transform rules (version stamp, setting_id gen for new profiles, base_id remap), writes via the adapter.
- **`vault`** — git operations on the vault repo: stage mirror files, commit with descriptive message, history log, rollback to a commit.
- **`api`** — REST handlers.
- **`paths`** — OS-specific path detection + user_id selection.
- **`config`** — load/save `~/.bborcaprofsync.yaml`, merge with auto-detected defaults.

---

## 4. Data model

### Internal representation

On read, a profile is parsed into:

```go
type Profile struct {
    Category   Category          // machine | process | filament
    Name       string            // from JSON "name"
    JSON       map[string]Value  // normalized field → value
    Info       Info              // parsed .info sidecar
    Source     SlicerID          // which slicer it came from
}

type Value struct {
    Raw      any    // original value (string, []string, etc.)
    Canonical any   // normalized for comparison (scalar string for single-value fields)
}

type Info struct {
    SyncInfo     string
    UserID       string
    SettingID    string
    BaseID       string
    UpdatedTime  int64
}
```

### Field categorization

Every field in the JSON is classified:

| Category | Examples | In diff? | Selectable for sync? | Transform on write? |
|---|---|---|---|---|
| **Setting** | `inner_wall_speed`, `top_surface_pattern`, `sparse_infill_pattern` | yes | yes | format-normalize to dest |
| **Identity** | `name`, `print_settings_id`, `from` | yes (context) | no | preserve dest's on existing; set on new |
| **Lineage** | `inherits` (JSON), `base_id` (`.info`) | yes (context) | no | auto-remap `base_id` |
| **Version** | `version` | yes (context) | no | stamp dest slicer version |

---

## 5. Transform rules

### 5.1 Array/scalar normalization

- **For diff comparison**: unwrap single-element arrays. `["400"]` == `"400"`. Multi-element arrays compared element-wise.
- **For writing**: consult the dest slicer's **field-type registry** — a per-slicer map of field name → `array` | `scalar`. If dest expects array and source value is scalar, wrap `[value]`. If dest expects scalar and source is single-element array, unwrap.
- **Registry source**: populated by sampling each slicer's system + user profiles at startup (or a hardcoded table built from sampling during development). The registry is slicer-version-aware if formats change between versions.

### 5.2 `version` stamping

- On write, set dest profile's `version` field to the **dest slicer's version**.
- Detect dest slicer version: read `version` from the most recent existing dest user profile (all profiles from a slicer share the same version). Fallback: configurable in Settings.

### 5.3 `setting_id` generation

- **Syncing into an existing dest profile**: preserve the dest's existing `setting_id`. Never change identity of an existing profile.
- **Creating a new dest profile**: generate a new `setting_id` in the dest slicer's format:
  - BambuStudio: `PPUS` + random hex (observed ~13 hex chars; research exact convention).
  - OrcaSlicer: UUID v4.
- Do **not** preserve the source `setting_id` — formats differ and the dest slicer manages its own ID space.

### 5.4 `base_id` remapping

- `inherits` (JSON) names the system profile the user profile derives from. `base_id` (`.info`) is that system profile's `setting_id`.
- On write, look up `inherits` name in the dest slicer's system profiles. If found, remap `base_id` to the dest system profile's `setting_id`.
- If `inherits` matches and `base_id` is already correct (same across slicers, as with `GP079`), no change.
- If no equivalent system profile in dest, flag to the user in the UI (don't silently write a wrong `base_id`).
- **Research task**: confirm whether system profile `setting_id`s differ across BS/Orca for the same `inherits` name across the full profile set.

### 5.5 `.info` sidecar

- On write, regenerate `.info` with: preserved or generated `setting_id`, remapped `base_id`, dest `user_id`, current `updated_time` (unix), empty `sync_info`.
- Write both `.json` and `.info` atomically (write to temp, rename).

### 5.6 Fields unique to one slicer

- Fields that exist in source but not in dest's field-type registry: kept as-is (dest slicer ignores unknown fields). Not flagged as errors.
- Fields removed in dest slicer version: preserved on source side; not forced onto dest unless explicitly selected.

---

## 6. Diff model

The diff is the heart of the UI. For a selected profile pair (source vs dest, same category):

```
┌─────────────────────────────────────────────────────────────────┐
│ Profile: 0.2 base    Category: process                          │
│ BambuStudio (PPUS249810809cffcb)  vs  OrcaSlicer (785471be-…)   │
├──────────────────────┬──────────────────┬──────────────────┬────┤
│ Field                │ BambuStudio      │ OrcaSlicer       │ ▶  │
├──────────────────────┼──────────────────┼──────────────────┼────┤
│ inner_wall_speed     │ 400              │ 400              │ —  │  (match)
│ top_surface_pattern  │ zig-zag          │ rectilinear      │ ◀▶ │  (differ — pick)
│ sparse_infill_speed  │ 400              │ 400              │ —  │  (match)
│ enable_support       │ 1                │ 1                │ —  │  (match)
│ ...                                                            │
├──────────────────────┴──────────────────┴──────────────────┴────┤
│ Context (not selectable):                                      │
│   name: 0.2 base (both)   inherits: 0.20mm Standard @BBL A1    │
│   version: 1.10.0.32 (both)  base_id: GP079 (both)             │
└─────────────────────────────────────────────────────────────────┘
```

- **Match** fields collapsed by default; **differ** fields expanded and highlighted.
- **Only-source** / **only-dest** fields shown with the missing side blank.
- Per differing field: radio to pick BS value ◀, Orca value ▶, or a text input for a custom value.
- "Apply selected" writes the chosen values to the dest profile.
- Format differences (`["400"]` vs `"400"`) are **not** shown as diffs — the normalized value is displayed.

---

## 7. Sync interaction

1. User selects category + a profile pair (auto-suggested by name, override available).
2. Diff loads, showing per-field source vs dest values.
3. For each differing field, user picks which value wins (or enters custom).
4. User clicks **Apply selected**.
5. App:
   a. Builds the resulting dest profile (existing fields + selected overrides).
   b. Runs transform rules (version stamp, setting_id gen if new, base_id remap, format normalize).
   c. Stages the updated mirror files in the **vault**.
   d. Commits the vault (message: `sync: process "0.2 base" → orcaslicer, 2 fields`).
   e. Copies vault mirror files → dest slicer user dir (atomic write: temp + rename).
   f. Returns success + commit hash.
6. If vault commit fails: abort, don't write slicer dir, surface error.
7. If slicer write fails after vault commit: vault is in good state; surface error, allow retry of step (e) without re-doing the diff.

### New profile creation

If no dest profile exists (or user marks "new"): the sync creates a new profile. User provides a name; `setting_id` is generated; `inherits`/`base_id` come from the source (remapped). All selected fields written.

---

## 8. Vault (git tracking)

### Structure

```
vault/                          # git repo
├── bambustudio/
│   └── user/<uid>/
│       ├── machine/*.json + *.info
│       ├── process/*.json + *.info
│       └── filament/*.json + *.info
└── orcaslicer/
    └── user/<uid>/
        ├── machine/...
        ├── process/...
        └── filament/...
```

The vault mirrors the slicer user dirs. On startup, the app syncs the mirror from the live slicer dirs (detecting out-of-band edits the slicer made).

### Commit granularity

One commit per **Apply selected** action (one profile, one direction, possibly multiple fields). Commit message format:

```
sync: <category> "<profile name>" → <dest slicer>
applied 2 fields: top_surface_pattern, inner_wall_speed
```

### History

The History tab shows the vault's git log with commit message + timestamp. Each entry can be expanded to show the diff of that commit.

### Rollback (post-MVP)

Selecting a history entry and rolling back: checkout that commit's mirror files and copy them back to the slicer user dirs. This reverts all syncs after that commit.

### Vault initialization

On first run (or if vault path is new): `git init`, do an initial commit of the current mirror state (`initial: snapshot of current profiles`).

---

## 9. Config & path detection

### OS-specific paths (macOS first)

| Slicer | macOS |
|---|---|
| BambuStudio | `~/Library/Application Support/BambuStudio/user/<uid>/` |
| OrcaSlicer | `~/Library/Application Support/OrcaSlicer/user/<uid>/` |

Linux (`~/.config/<Slicer>/user/`) and Windows (`%APPDATA%\<Slicer>\user\`) are post-MVP.

### User_id selection

When multiple `user/<uid>` dirs exist:
1. Exclude empty dirs (e.g. `default/` with 0 profiles).
2. Auto-pick the dir with the most profiles.
3. Expose in Settings for manual override (handles the UUID-vs-numeric ambiguity).

### Config file: `~/.bborcaprofsync.yaml`

```yaml
slicers:
  bambustudio:
    user_dir: ""        # override auto-detected path
    user_id: ""         # override auto-selected uid
  orcaslicer:
    user_dir: ""
    user_id: ""
vault:
  path: ""              # override default app-data-dir vault
server:
  port: 8484
```

Empty fields = use auto-detected defaults. Settings tab edits this file.

---

## 10. Web UI

Single-page, tabbed. No framework — vanilla HTML/JS/CSS embedded via `go:embed`.

### Tabs

- **Browse**: category selector (machine/process/fillet) + slicer pair. Lists profile pairs auto-matched by name. Unpaired profiles shown separately. Search/filter (needed for ~40 filament profiles). Clicking a pair opens the diff panel inline.
- **History**: vault git log. Commit message, timestamp, expandable field changes.
- **Settings**: slicer paths, user_id selection, vault path, slicer version overrides. Edits persist to config file.

### Diff panel (inline in Browse)

As described in §6. Per-field value selection, "Apply selected" button. Shows transform context (version, setting_id, base_id) as read-only context.

---

## 11. API

```
GET  /api/health
     → { paths, user_ids, slicer_versions, vault_status }

GET  /api/profiles?slicer=bs&category=process
     → [{ name, setting_id, inherits, updated_time, paired: bool }]

GET  /api/profile?slicer=bs&category=process&name=0.2 base
     → { profile (full JSON), info }

GET  /api/diff?category=process&bsName=0.2 base&orcaName=0.2 base
     → { fields: [{ name, bsValue, orcaValue, status, selectable }], context: {...} }

POST /api/sync
     body: {
       category,
       destSlicer, destName, isNew: bool,
       fields: { "top_surface_pattern": "rectilinear", ... }
     }
     → { commit, writtenFiles }

GET  /api/history
     → [{ hash, message, timestamp }]

GET  /api/settings
PUT  /api/settings
```

---

## 12. File structure

```
BBOrcaProfsync/
├── AGENTS.md                  # agent operating rules only
├── plan.md                    # this design spec
├── GLOSSARY.md                # domain terms
├── go.mod
├── cmd/
│   └── bborcaprofsync/
│       └── main.go
├── internal/
│   ├── slicer/
│   │   ├── adapter.go         # SlicerAdapter interface
│   │   ├── bambustudio.go
│   │   └── orcaslicer.go
│   ├── profile/
│   │   ├── types.go           # Profile, Info, Category, Value
│   │   ├── read.go            # parse + normalize on read
│   │   ├── write.go           # denormalize + atomic write
│   │   └── fieldtype.go       # field-type registry (array/scalar per slicer)
│   ├── diff/
│   │   └── diff.go            # field-level diff, format-normalized comparison
│   ├── sync/
│   │   ├── engine.go          # apply selected fields + run transforms
│   │   └── transform.go       # version stamp, setting_id gen, base_id remap
│   ├── vault/
│   │   └── vault.go           # git: stage, commit, history, rollback
│   ├── api/
│   │   └── handlers.go
│   ├── paths/
│   │   └── detect.go          # OS path detection, user_id selection
│   └── config/
│       └── config.go          # load/save ~/.bborcaprofsync.yaml
├── web/
│   ├── embed.go               # //go:embed
│   ├── index.html
│   ├── app.js
│   └── style.css
├── testdata/
│   └── fixtures/
│       ├── bs_0.2base.json
│       ├── bs_0.2base.info
│       ├── orca_0.2base.json
│       └── orca_0.2base.info
└── README.md
```

---

## 13. Test plan

### Unit tests

- **`profile`**: parse JSON + `.info`, normalize values (unwrap arrays), round-trip parse → normalize → denormalize → equals original.
- **`diff`**: value comparison with format normalization (`["400"]` == `"400"`), status classification (match/differ/only-source/only-dest), field categorization (setting/identity/lineage/version).
- **`sync/transform`**: version stamping, setting_id generation (format correctness), base_id remapping (found/not-found/flag).
- **`paths`**: user_id selection logic (multiple dirs, empty dirs, UUID vs numeric).
- **`config`**: load defaults, merge overrides, save.

### Integration tests

- **Round-trip**: read fixture profile → normalize → transform for dest → write to temp dir → re-read → assert expected fields match.
- **Sync flow**: mock slicer dirs in temp, diff two profiles, apply selected fields, assert dest file written correctly + vault committed.
- **Vault**: init temp git repo, stage + commit, assert history, rollback restores.
- **API handlers**: mock profile dirs, assert `/api/profiles`, `/api/diff`, `/api/sync` responses.

### Fixtures

- The real `0.2 base` pair (captures array/scalar + value divergence) — copy into `testdata/fixtures/`.
- Synthetic edge cases: new profile creation, missing dest, multi-element array fields, fields unique to one slicer.

---

## 14. MVP scope

1. Path detection + config (macOS).
2. Read + list profiles from both slicers (all 3 categories).
3. Field-level diff with auto-pair by name.
4. Per-field selective sync (write to existing or new dest profile).
5. Vault git tracking (stage, commit, history).
6. Web UI: Browse + diff panel + Settings.
7. Unit + integration tests with fixtures.

### Post-MVP

- Rollback via vault history.
- Bulk operations (sync multiple profiles).
- Linux + Windows path detection.
- Field-type registry auto-sampling tool.
- Base_id remapping research (full system profile cross-reference).
- Vault history diff view (show what a past commit changed).
