# Glossary

Canonical terms for BBOrcaProfsync. Use these in code, issues, docs, and conversation. Don't use synonyms the glossary explicitly avoids.

## Profile

A single user-created configuration, stored as a file pair (`.json` + `.info`) in one of three categories. **Avoid** "preset" — the two slicers and the old spec used it interchangeably with "profile," causing ambiguity. One word: **profile**.

- **Machine profile** — printer settings per model + nozzle. Lives in `machine/`.
- **Process profile** — print settings: layer height, speeds, infill, support. Lives in `process/`.
- **Filament profile** — filament temperature / cooling per material. Lives in `filament/`.

## Setting field

A key→value pair in the profile `.json` that is a tunable value (e.g. `inner_wall_speed`, `top_surface_pattern`). Selectable for per-field sync. Contrast with identity, lineage, and version fields, which are not selectable.

## Identity fields

`name`, `print_settings_id` (process) / `machine_id` (machine) / `filament_id` (filament), and `from`. These establish a profile's identity. Shown in the diff as context; never selectable for sync (changing them would break identity).

## Lineage fields

- **`inherits`** (in `.json`) — names the system profile this user profile derives from.
- **`base_id`** (in `.info`) — the `setting_id` of that system profile.

Auto-remapped on write to the dest slicer's equivalent. Not selectable.

## setting_id

An account-scoped identifier for a user profile, stored in `.info`. Format differs by slicer: BambuStudio uses `PPUS` + hex; OrcaSlicer native accounts use UUID. Used to flag "already synced" pairs but is **not** the primary matching key (formats differ across slicers).

## Version

The `version` field in `.json`, reflecting the slicer version that wrote the profile. On sync, stamped to the dest slicer's version. Not selectable.

## Vault

The app's local git repo that mirrors profile files from both slicers. Every sync stages + commits to the vault, creating a timeline of changes. Rollback restores from the vault.

## Sync

A **manual, per-field** write operation driven by the web UI. The user picks which value wins for each differing field (or enters a custom value) and applies. **Never** automatic or background.
