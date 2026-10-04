// Package api wires the REST endpoints described in plan.md §11 onto
// the embedded net/http server.
package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Jurrer/BBOrcaPresetManager/internal/config"
	"github.com/Jurrer/BBOrcaPresetManager/internal/diff"
	"github.com/Jurrer/BBOrcaPresetManager/internal/paths"
	"github.com/Jurrer/BBOrcaPresetManager/internal/profile"
	"github.com/Jurrer/BBOrcaPresetManager/internal/slicer"
	"github.com/Jurrer/BBOrcaPresetManager/internal/sync"
	"github.com/Jurrer/BBOrcaPresetManager/internal/vault"
)

// Server is the HTTP server. Concrete dependencies (sync engine, vault,
// slicer adapters) are attached at startup.
type Server struct {
	mux     *http.ServeMux
	bs      slicer.SlicerAdapter
	orca    slicer.SlicerAdapter
	engine  *sync.Engine
	vault   *vault.Vault
	bsPaths paths.SlicerPaths
	orPaths paths.SlicerPaths
	config  config.Config
	cfgPath string
}

// NewServer constructs the handler with all dependencies wired in.
//
// cfgPath is the on-disk path of the config file; PUT /api/settings writes
// back to it. Pass "" to disable persistence (handy for tests).
func NewServer(
	bs, orca slicer.SlicerAdapter,
	bsPaths, orPaths paths.SlicerPaths,
	engine *sync.Engine,
	v *vault.Vault,
	cfg config.Config,
	cfgPath string,
) *Server {
	s := &Server{
		mux:     http.NewServeMux(),
		bs:      bs,
		orca:    orca,
		engine:  engine,
		vault:   v,
		bsPaths: bsPaths,
		orPaths: orPaths,
		config:  cfg,
		cfgPath: cfgPath,
	}
	s.routes()
	return s
}

// Handler returns the http.Handler that should be served.
func (s *Server) Handler() http.Handler { return s.mux }

func (s *Server) routes() {
	s.mux.HandleFunc("/api/health", s.health)
	s.mux.HandleFunc("/api/profiles", s.listProfiles)
	s.mux.HandleFunc("/api/profile", s.getProfile)
	s.mux.HandleFunc("/api/diff", s.diffProfiles)
	s.mux.HandleFunc("/api/sync", s.syncProfiles)
	s.mux.HandleFunc("/api/history", s.history)
	s.mux.HandleFunc("/api/settings", s.settings)
}

// --- Response/request payload types (plan §11 wire shapes) ---

// HealthResponse mirrors GET /api/health -> { paths, user_ids, slicer_versions, vault_status }.
type HealthResponse struct {
	Paths          map[string]string   `json:"paths"`
	UserIDs        map[string][]string `json:"user_ids"`
	SlicerVersions map[string]string   `json:"slicer_versions"`
	VaultStatus    VaultStatus         `json:"vault_status"`
}

// VaultStatus reports the vault path + initialized flag for /api/health.
type VaultStatus struct {
	Initialized bool   `json:"initialized"`
	Path        string `json:"path"`
}

// ProfileSummary mirrors one element of GET /api/profiles -> [{ name, setting_id, inherits, updated_time, paired }].
type ProfileSummary struct {
	Name        string `json:"name"`
	SettingID   string `json:"setting_id"`
	Inherits    string `json:"inherits"`
	UpdatedTime int64  `json:"updated_time"`
	Paired      bool   `json:"paired"`
}

// ProfileResponse mirrors GET /api/profile -> { profile, info }.
type ProfileResponse struct {
	Profile profile.Profile `json:"profile"`
	Info    profile.Info    `json:"info"`
}

// DiffResponse mirrors GET /api/diff -> { fields, context }.
type DiffResponse struct {
	Fields  []diff.DiffField `json:"fields"`
	Context diff.DiffContext `json:"context"`
}

// SyncRequest mirrors POST /api/sync body.
type SyncRequest struct {
	Category   profile.Category `json:"category"`
	DestSlicer profile.SlicerID `json:"dest_slicer"`
	DestName   string           `json:"dest_name"`
	IsNew      bool             `json:"is_new"`
	Fields     map[string]any   `json:"fields"`
}

// SyncResponse mirrors POST /api/sync -> { commit, written_files }.
type SyncResponse struct {
	Commit       string   `json:"commit"`
	WrittenFiles []string `json:"written_files"`
}

// HistoryEntry mirrors one element of GET /api/history -> [{ hash, message, timestamp }].
type HistoryEntry struct {
	Hash      string `json:"hash"`
	Message   string `json:"message"`
	Timestamp int64  `json:"timestamp"`
}

// SettingsPayload is the JSON body of GET/PUT /api/settings.
type SettingsPayload map[string]any

// --- Handlers ---

// GET /api/health
func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	resp := HealthResponse{
		Paths: map[string]string{
			"bambustudio": s.bsPaths.UserDir,
			"orcaslicer":  s.orPaths.UserDir,
		},
		UserIDs: map[string][]string{
			"bambustudio": s.bsPaths.CandidateUIDs,
			"orcaslicer":  s.orPaths.CandidateUIDs,
		},
		SlicerVersions: map[string]string{
			"bambustudio": s.bs.Version(),
			"orcaslicer":  s.orca.Version(),
		},
		VaultStatus: VaultStatus{
			Initialized: s.vault != nil,
			Path:        vaultPath(s.vault),
		},
	}
	writeJSON(w, http.StatusOK, resp)
}

// GET /api/profiles?slicer=bs&category=process
func (s *Server) listProfiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	q := r.URL.Query()
	slicerID, err := parseSlicer(q.Get("slicer"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	category, err := parseCategory(q.Get("category"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	adapter, other := s.adapterFor(slicerID), s.otherAdapter(slicerID)
	names, err := adapter.ListProfiles(category)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list profiles: "+err.Error())
		return
	}
	otherNames := map[string]bool{}
	if other != nil {
		if onList, err := other.ListProfiles(category); err == nil {
			for _, n := range onList {
				otherNames[n] = true
			}
		}
	}

	out := make([]ProfileSummary, 0, len(names))
	for _, name := range names {
		rr, err := adapter.ReadProfile(category, name)
		if err != nil {
			// Skip unreadable profiles rather than failing the whole list.
			continue
		}
		out = append(out, ProfileSummary{
			Name:        rr.Profile.Name,
			SettingID:   rr.Profile.Info.SettingID,
			Inherits:    profile.JSONString(rr.Profile.JSON, "inherits"),
			UpdatedTime: rr.Profile.Info.UpdatedTime,
			Paired:      otherNames[rr.Profile.Name],
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// GET /api/profile?slicer=bs&category=process&name=0.2 base
func (s *Server) getProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	q := r.URL.Query()
	slicerID, err := parseSlicer(q.Get("slicer"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	category, err := parseCategory(q.Get("category"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	name := q.Get("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "missing name")
		return
	}
	adapter := s.adapterFor(slicerID)
	rr, err := adapter.ReadProfile(category, name)
	if err != nil {
		writeError(w, http.StatusNotFound, "read profile: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, ProfileResponse{Profile: rr.Profile, Info: rr.Profile.Info})
}

// GET /api/diff?category=process&bsName=0.2 base&orcaName=0.2 base
func (s *Server) diffProfiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	q := r.URL.Query()
	category, err := parseCategory(q.Get("category"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	bsName := q.Get("bsName")
	orcaName := q.Get("orcaName")
	if bsName == "" || orcaName == "" {
		writeError(w, http.StatusBadRequest, "bsName and orcaName required")
		return
	}

	bsRR, err := s.bs.ReadProfile(category, bsName)
	if err != nil {
		writeError(w, http.StatusNotFound, "read bs profile: "+err.Error())
		return
	}
	orcaRR, err := s.orca.ReadProfile(category, orcaName)
	if err != nil {
		writeError(w, http.StatusNotFound, "read orca profile: "+err.Error())
		return
	}

	d := diff.ComputeDiff(bsRR.Profile, orcaRR.Profile)
	writeJSON(w, http.StatusOK, DiffResponse{Fields: d.Fields, Context: d.Context})
}

// POST /api/sync
func (s *Server) syncProfiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req SyncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "decode body: "+err.Error())
		return
	}
	if req.Category == "" {
		writeError(w, http.StatusBadRequest, "category required")
		return
	}
	if req.DestSlicer == "" {
		writeError(w, http.StatusBadRequest, "dest_slicer required")
		return
	}
	if req.DestName == "" {
		writeError(w, http.StatusBadRequest, "dest_name required")
		return
	}
	adapter := s.adapterFor(req.DestSlicer)
	if adapter == nil {
		writeError(w, http.StatusBadRequest, "unknown dest_slicer")
		return
	}

	var dest profile.Profile
	if req.IsNew {
		dest = profile.Profile{
			Category: req.Category,
			Name:     req.DestName,
			JSON:     map[string]profile.Value{},
			Info:     profile.Info{UserID: s.userIDFor(req.DestSlicer)},
			Source:   req.DestSlicer,
		}
	} else {
		rr, err := adapter.ReadProfile(req.Category, req.DestName)
		if err != nil {
			writeError(w, http.StatusNotFound, "read dest profile: "+err.Error())
			return
		}
		dest = rr.Profile
	}

	overrides := sync.FieldOverrides{}
	for k, v := range req.Fields {
		overrides[k] = v
	}

	res, err := s.engine.Sync(dest, req.DestSlicer, overrides, req.IsNew, adapter.Version())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "sync: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, SyncResponse{Commit: res.Commit, WrittenFiles: res.WrittenFiles})
}

// GET /api/history
func (s *Server) history(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	commits, err := s.vault.History()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "history: "+err.Error())
		return
	}
	out := make([]HistoryEntry, 0, len(commits))
	for _, c := range commits {
		out = append(out, HistoryEntry{
			Hash:      c.Hash,
			Message:   c.Message,
			Timestamp: c.Timestamp,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// GET /api/settings  /  PUT /api/settings
func (s *Server) settings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, s.effectiveConfig())
	case http.MethodPut:
		var c config.Config
		if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
			writeError(w, http.StatusBadRequest, "decode body: "+err.Error())
			return
		}
		if s.cfgPath != "" {
			if err := config.Save(s.cfgPath, c); err != nil {
				writeError(w, http.StatusInternalServerError, "save: "+err.Error())
				return
			}
		}
		if c.SlicerVersions == nil {
			c.SlicerVersions = make(map[string]string)
		}
		s.config = c
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// --- Helpers ---
// effectiveConfig returns the stored config with auto-detected values
// filled in for empty override fields, so the Settings tab shows the
// values actually in use (plan §9).
func (s *Server) effectiveConfig() config.Config {
	c := s.config
	// Copy the map so we do not mutate the original.
	c.SlicerVersions = map[string]string{}
	for k, v := range s.config.SlicerVersions {
		c.SlicerVersions[k] = v
	}
	if c.Slicers.BambuStudio.UserDir == "" {
		c.Slicers.BambuStudio.UserDir = s.bsPaths.UserDir
	}
	if c.Slicers.BambuStudio.UserID == "" {
		c.Slicers.BambuStudio.UserID = s.bsPaths.SelectedUID
	}
	if c.Slicers.OrcaSlicer.UserDir == "" {
		c.Slicers.OrcaSlicer.UserDir = s.orPaths.UserDir
	}
	if c.Slicers.OrcaSlicer.UserID == "" {
		c.Slicers.OrcaSlicer.UserID = s.orPaths.SelectedUID
	}
	if c.Vault.Path == "" {
		c.Vault.Path = vaultPath(s.vault)
	}
	if c.SlicerVersions["bambustudio"] == "" && s.bs != nil {
		if v := s.bs.Version(); v != "" {
			c.SlicerVersions["bambustudio"] = v
		}
	}
	if c.SlicerVersions["orcaslicer"] == "" && s.orca != nil {
		if v := s.orca.Version(); v != "" {
			c.SlicerVersions["orcaslicer"] = v
		}
	}
	return c
}

// writeJSON marshals v as JSON and writes it with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError writes a JSON error body: { "error": "<msg>" }.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// adapterFor returns the adapter for the given slicer ID. Accepts both the
// canonical SlicerID ("bambustudio"/"orcaslicer") and the short alias
// ("bs"/"orca"). Unknown values return nil.
func (s *Server) adapterFor(id profile.SlicerID) slicer.SlicerAdapter {
	switch id {
	case profile.SlicerBambuStudio, "bs":
		return s.bs
	case profile.SlicerOrcaSlicer, "orca":
		return s.orca
	default:
		return nil
	}
}

// otherAdapter returns the adapter that isn't `id`.
func (s *Server) otherAdapter(id profile.SlicerID) slicer.SlicerAdapter {
	switch id {
	case profile.SlicerBambuStudio, "bs":
		return s.orca
	default:
		return s.bs
	}
}

// userIDFor returns the configured user_id for the given slicer.
func (s *Server) userIDFor(id profile.SlicerID) string {
	switch id {
	case profile.SlicerBambuStudio, "bs":
		return s.config.Slicers.BambuStudio.UserID
	default:
		return s.config.Slicers.OrcaSlicer.UserID
	}
}

// parseSlicer accepts "bs"/"bambustudio" and "orca"/"orcaslicer".
func parseSlicer(raw string) (profile.SlicerID, error) {
	switch raw {
	case "bs", "bambustudio":
		return profile.SlicerBambuStudio, nil
	case "orca", "orcaslicer":
		return profile.SlicerOrcaSlicer, nil
	default:
		return "", errors.New("slicer must be bs or orca")
	}
}

// parseCategory accepts "machine", "process", "filament".
func parseCategory(raw string) (profile.Category, error) {
	switch raw {
	case "machine":
		return profile.CategoryMachine, nil
	case "process":
		return profile.CategoryProcess, nil
	case "filament":
		return profile.CategoryFilament, nil
	default:
		return "", errors.New("category must be machine, process, or filament")
	}
}

// vaultPath returns v.Path() or "" if v is nil.
func vaultPath(v *vault.Vault) string {
	if v == nil {
		return ""
	}
	return v.Path()
}
