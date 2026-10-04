package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/Jurrer/BBOrcaPresetManager/internal/config"
	"github.com/Jurrer/BBOrcaPresetManager/internal/paths"
	"github.com/Jurrer/BBOrcaPresetManager/internal/profile"
	"github.com/Jurrer/BBOrcaPresetManager/internal/slicer"
	"github.com/Jurrer/BBOrcaPresetManager/internal/sync"
	"github.com/Jurrer/BBOrcaPresetManager/internal/vault"
)

const (
	testBSUID   = "2240525152"
	testOrcaUID = "11b99db1-3c45-4482-a69b-7f430a909e28"
	testBSVer   = "1.10.0.32"
	testOrcaVer = "2.0.0"
)

// setupServer creates a fresh test environment: temp BS + Orca user dirs
// (each pre-populated with the testdata fixture profile under its
// respective slicer uid/category), a vault in a third temp dir, and an API
// server wired to all of them. Returns the server plus the resolved
// paths so tests can assert file-level effects.
func setupServer(t *testing.T) (*Server, string, string, string) {
	t.Helper()

	bsRoot := t.TempDir()
	orcaRoot := t.TempDir()
	vaultDir := t.TempDir()

	bsUIDDir := filepath.Join(bsRoot, testBSUID)
	orcaUIDDir := filepath.Join(orcaRoot, testOrcaUID)
	if err := os.MkdirAll(filepath.Join(bsUIDDir, "process"), 0o755); err != nil {
		t.Fatalf("mkdir bs process: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(bsUIDDir, "machine"), 0o755); err != nil {
		t.Fatalf("mkdir bs machine: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(orcaUIDDir, "process"), 0o755); err != nil {
		t.Fatalf("mkdir orca process: %v", err)
	}

	fixtures := "../../testdata/fixtures"
	for _, ext := range []string{".json", ".info"} {
		data, err := os.ReadFile(filepath.Join(fixtures, "bs_0.2base"+ext))
		if err != nil {
			t.Fatalf("read bs fixture %s: %v", ext, err)
		}
		if err := os.WriteFile(filepath.Join(bsUIDDir, "process", "0.2 base"+ext), data, 0o644); err != nil {
			t.Fatalf("write bs %s: %v", ext, err)
		}
		data, err = os.ReadFile(filepath.Join(fixtures, "orca_0.2base"+ext))
		if err != nil {
			t.Fatalf("read orca fixture %s: %v", ext, err)
		}
		if err := os.WriteFile(filepath.Join(orcaUIDDir, "process", "0.2 base"+ext), data, 0o644); err != nil {
			t.Fatalf("write orca %s: %v", ext, err)
		}
	}

	bs := slicer.NewBambuStudioAdapter(bsUIDDir, testBSUID, testBSVer)
	orca := slicer.NewOrcaSlicerAdapter(orcaUIDDir, testOrcaUID, testOrcaVer)

	bsPaths := paths.SlicerPaths{
		UserDir:       bsUIDDir,
		CandidateUIDs: []string{testBSUID},
		SelectedUID:   testBSUID,
	}
	orPaths := paths.SlicerPaths{
		UserDir:       orcaUIDDir,
		CandidateUIDs: []string{testOrcaUID},
		SelectedUID:   testOrcaUID,
	}

	v, err := vault.Open(vaultDir)
	if err != nil {
		t.Fatalf("vault.Open: %v", err)
	}

	cfg := config.Config{}
	cfg.Slicers.BambuStudio.UserID = testBSUID
	cfg.Slicers.OrcaSlicer.UserID = testOrcaUID
	cfg.Server.Port = 9876

	srv := NewServer(bs, orca, bsPaths, orPaths, sync.NewEngine(bs, orca, v), v, cfg, "")
	return srv, bsRoot, orcaRoot, vaultDir
}

// --- HTTP helpers ---

func getJSON(t *testing.T, h http.Handler, path string) (int, []byte) {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	h.ServeHTTP(rec, req)
	return rec.Code, rec.Body.Bytes()
}

func postJSON(t *testing.T, h http.Handler, path string, body any) (int, []byte) {
	t.Helper()
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		t.Fatalf("encode body: %v", err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(rec, req)
	return rec.Code, rec.Body.Bytes()
}

func putJSON(t *testing.T, h http.Handler, path string, body any) (int, []byte) {
	t.Helper()
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		t.Fatalf("encode body: %v", err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(rec, req)
	return rec.Code, rec.Body.Bytes()
}

// --- /api/health ---

func TestHealth(t *testing.T) {
	srv, _, _, _ := setupServer(t)
	code, body := getJSON(t, srv.Handler(), "/api/health")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", code, body)
	}
	var got HealthResponse
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal: %v; body=%s", err, body)
	}
	if got.VaultStatus.Path == "" {
		t.Error("vault_status.path is empty")
	}
	if !got.VaultStatus.Initialized {
		t.Error("vault_status.initialized = false, want true")
	}
	if got.SlicerVersions["bambustudio"] != testBSVer {
		t.Errorf("bs version = %q, want %q", got.SlicerVersions["bambustudio"], testBSVer)
	}
	if got.SlicerVersions["orcaslicer"] != testOrcaVer {
		t.Errorf("orca version = %q, want %q", got.SlicerVersions["orcaslicer"], testOrcaVer)
	}
	if _, ok := got.Paths["bambustudio"]; !ok {
		t.Error("paths.bambustudio missing")
	}
	if _, ok := got.UserIDs["bambustudio"]; !ok {
		t.Error("user_ids.bambustudio missing")
	}
}

// --- /api/profiles ---

func TestListProfiles(t *testing.T) {
	srv, _, _, _ := setupServer(t)

	code, body := getJSON(t, srv.Handler(), "/api/profiles?slicer=bs&category=process")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", code, body)
	}
	var got []ProfileSummary
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal: %v; body=%s", err, body)
	}
	if len(got) != 1 {
		t.Fatalf("got %d profiles, want 1; body=%s", len(got), body)
	}
	p := got[0]
	if p.Name != "0.2 base" {
		t.Errorf("name = %q, want %q", p.Name, "0.2 base")
	}
	if p.SettingID != "PPUS249810809cffcb" {
		t.Errorf("setting_id = %q, want PPUS...", p.SettingID)
	}
	if !p.Paired {
		t.Error("paired = false, want true (same name exists in Orca)")
	}
	if p.Inherits == "" {
		t.Error("inherits empty")
	}
	if p.UpdatedTime == 0 {
		t.Error("updated_time = 0")
	}
}

func TestListProfilesUnknownSlicer(t *testing.T) {
	srv, _, _, _ := setupServer(t)
	code, body := getJSON(t, srv.Handler(), "/api/profiles?slicer=unknown&category=process")
	if code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400; body=%s", code, body)
	}
}

func TestListProfilesUnknownCategory(t *testing.T) {
	srv, _, _, _ := setupServer(t)
	code, _ := getJSON(t, srv.Handler(), "/api/profiles?slicer=bs&category=zzz")
	if code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", code)
	}
}

// --- /api/profile ---

func TestGetProfile(t *testing.T) {
	srv, _, _, _ := setupServer(t)
	code, body := getJSON(t, srv.Handler(), "/api/profile?slicer=bs&category=process&name="+urlQ("0.2 base"))
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", code, body)
	}
	var got ProfileResponse
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Profile.Name != "0.2 base" {
		t.Errorf("profile.name = %q, want %q", got.Profile.Name, "0.2 base")
	}
	if got.Info.SettingID != "PPUS249810809cffcb" {
		t.Errorf("info.setting_id = %q", got.Info.SettingID)
	}
	if len(got.Profile.JSON) == 0 {
		t.Error("profile.json is empty")
	}
}

func TestGetProfileMissingName(t *testing.T) {
	srv, _, _, _ := setupServer(t)
	code, _ := getJSON(t, srv.Handler(), "/api/profile?slicer=bs&category=process")
	if code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", code)
	}
}

func TestGetProfileNotFound(t *testing.T) {
	srv, _, _, _ := setupServer(t)
	code, _ := getJSON(t, srv.Handler(), "/api/profile?slicer=bs&category=process&name=nope")
	if code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", code)
	}
}

// --- /api/diff ---

func TestDiff(t *testing.T) {
	srv, _, _, _ := setupServer(t)
	path := "/api/diff?category=process&bsName=" + urlQ("0.2 base") + "&orcaName=" + urlQ("0.2 base")
	code, body := getJSON(t, srv.Handler(), path)
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", code, body)
	}
	var got DiffResponse
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got.Fields) == 0 {
		t.Error("fields empty")
	}
	// The fixtures diverge on top_surface_pattern (BS="zig-zag", Orca="rectilinear")
	// so it must appear as a "differ" field.
	var found bool
	for _, f := range got.Fields {
		if f.Name == "top_surface_pattern" {
			found = true
			if f.Status != "differ" {
				t.Errorf("top_surface_pattern status = %q, want differ", f.Status)
			}
			if !f.Selectable {
				t.Error("top_surface_pattern.selectable = false, want true")
			}
		}
	}
	if !found {
		t.Error("top_surface_pattern not found in diff fields")
	}
	if got.Context.SourceName != "0.2 base" || got.Context.DestName != "0.2 base" {
		t.Errorf("context names = (%q,%q)", got.Context.SourceName, got.Context.DestName)
	}
	if got.Context.SourceSettingID == "" || got.Context.DestSettingID == "" {
		t.Error("context setting ids empty")
	}
}

// --- /api/sync ---

func TestSyncExisting(t *testing.T) {
	srv, bsRoot, _, _ := setupServer(t)
	body := SyncRequest{
		Category:   profile.CategoryProcess,
		DestSlicer: profile.SlicerBambuStudio,
		DestName:   "0.2 base",
		IsNew:      false,
		Fields:     map[string]any{"top_surface_pattern": "rectilinear"},
	}
	code, resp := postJSON(t, srv.Handler(), "/api/sync", body)
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", code, resp)
	}
	var got SyncResponse
	if err := json.Unmarshal(resp, &got); err != nil {
		t.Fatalf("unmarshal: %v; body=%s", err, resp)
	}
	if got.Commit == "" {
		t.Error("commit is empty")
	}
	if len(got.WrittenFiles) != 2 {
		t.Errorf("writtenFiles = %d, want 2", len(got.WrittenFiles))
	}
	// Verify file actually written under the BS user dir.
	bsFile := filepath.Join(bsRoot, testBSUID, "process", "0.2 base.json")
	if _, err := os.Stat(bsFile); err != nil {
		t.Errorf("dest file not written: %v", err)
	}
	rr, err := profile.Read(filepath.Join(bsRoot, testBSUID, "process"), "0.2 base", profile.CategoryProcess, profile.SlicerBambuStudio)
	if err != nil {
		t.Fatalf("re-read: %v", err)
	}
	if v := rr.Profile.JSON["top_surface_pattern"].Canonical; v != "rectilinear" {
		t.Errorf("top_surface_pattern = %v, want rectilinear", v)
	}
}

func TestSyncNewProfile(t *testing.T) {
	srv, _, orcaRoot, _ := setupServer(t)
	body := SyncRequest{
		Category:   profile.CategoryProcess,
		DestSlicer: profile.SlicerOrcaSlicer,
		DestName:   "fresh profile",
		IsNew:      true,
		Fields:     map[string]any{"name": "fresh profile", "top_surface_pattern": "rectilinear"},
	}
	code, resp := postJSON(t, srv.Handler(), "/api/sync", body)
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", code, resp)
	}
	var got SyncResponse
	if err := json.Unmarshal(resp, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Commit == "" {
		t.Error("commit empty")
	}
	orcaFile := filepath.Join(orcaRoot, testOrcaUID, "process", "fresh profile.json")
	if _, err := os.Stat(orcaFile); err != nil {
		t.Errorf("new dest file not written: %v", err)
	}
	rr, err := profile.Read(filepath.Join(orcaRoot, testOrcaUID, "process"), "fresh profile", profile.CategoryProcess, profile.SlicerOrcaSlicer)
	if err != nil {
		t.Fatalf("re-read new: %v", err)
	}
	if rr.Profile.Info.SettingID == "" {
		t.Error("new profile setting_id empty")
	}
}

func TestSyncBadSlicer(t *testing.T) {
	srv, _, _, _ := setupServer(t)
	code, _ := postJSON(t, srv.Handler(), "/api/sync", SyncRequest{
		Category:   profile.CategoryProcess,
		DestSlicer: "nope",
		DestName:   "x",
	})
	if code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", code)
	}
}

// --- /api/history ---

func TestHistory(t *testing.T) {
	srv, _, _, _ := setupServer(t)
	// Run a sync to seed the history.
	_, _ = postJSON(t, srv.Handler(), "/api/sync", SyncRequest{
		Category:   profile.CategoryProcess,
		DestSlicer: profile.SlicerBambuStudio,
		DestName:   "0.2 base",
		Fields:     map[string]any{"top_surface_pattern": "rectilinear"},
	})

	code, body := getJSON(t, srv.Handler(), "/api/history")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", code, body)
	}
	var got []HistoryEntry
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("history empty after sync")
	}
	// Initial commit + at least one sync commit.
	if len(got) < 2 {
		t.Errorf("history len = %d, want >= 2 (initial + sync)", len(got))
	}
	// Newest first ordering.
	if !sort.SliceIsSorted(got, func(i, j int) bool { return got[i].Timestamp > got[j].Timestamp }) {
		t.Error("history not sorted newest-first by timestamp")
	}
	if got[0].Hash == "" {
		t.Error("first commit hash missing")
	}
}

// --- /api/settings ---

func TestSettingsRoundTrip(t *testing.T) {
	srv, _, _, _ := setupServer(t)

	code, body := getJSON(t, srv.Handler(), "/api/settings")
	if code != http.StatusOK {
		t.Fatalf("GET status = %d, want 200; body=%s", code, body)
	}
	var got config.Config
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("GET unmarshal: %v", err)
	}
	if got.Server.Port != 9876 {
		t.Errorf("default port = %d, want 9876", got.Server.Port)
	}
	if got.Slicers.BambuStudio.UserID != testBSUID {
		t.Errorf("bs uid = %q, want %q", got.Slicers.BambuStudio.UserID, testBSUID)
	}

	// PUT a new value (in-memory only because cfgPath == "").
	newCfg := got
	newCfg.Server.Port = 12345
	newCfg.SlicerVersions = map[string]string{"bambustudio": "1.10.0.32", "orcaslicer": "2.0.0"}
	code, putBody := putJSON(t, srv.Handler(), "/api/settings", newCfg)
	if code != http.StatusOK {
		t.Fatalf("PUT status = %d, want 200; body=%s", code, putBody)
	}

	// GET should reflect the change.
	_, body = getJSON(t, srv.Handler(), "/api/settings")
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("GET-after-PUT unmarshal: %v", err)
	}
	if got.Server.Port != 12345 {
		t.Errorf("after PUT port = %d, want 12345", got.Server.Port)
	}
	if got.SlicerVersions["bambustudio"] != "1.10.0.32" {
		t.Errorf("after PUT bs version = %q, want 1.10.0.32", got.SlicerVersions["bambustudio"])
	}
}

func TestSettingsPutPersists(t *testing.T) {
	srv, _, _, _ := setupServer(t)
	// Wire a real path on the server.
	cfgPath := filepath.Join(t.TempDir(), "settings.yaml")
	srv.cfgPath = cfgPath

	_, _ = putJSON(t, srv.Handler(), "/api/settings", config.Config{
		Server: config.ServerConfig{Port: 5555},
	})

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("config not persisted: %v", err)
	}
	if !bytes.Contains(data, []byte("5555")) {
		t.Errorf("persisted config doesn't contain port 5555: %s", data)
	}
}

// --- Routing & method gating ---

func TestRoutesRejectBadMethod(t *testing.T) {
	srv, _, _, _ := setupServer(t)
	// POST to a GET endpoint.
	code, _ := postJSON(t, srv.Handler(), "/api/health", map[string]any{})
	if code != http.StatusMethodNotAllowed {
		t.Errorf("POST /api/health = %d, want 405", code)
	}
	// GET to a POST endpoint.
	code, _ = getJSON(t, srv.Handler(), "/api/sync")
	if code != http.StatusMethodNotAllowed {
		t.Errorf("GET /api/sync = %d, want 405", code)
	}
}

func TestDiffMissingNames(t *testing.T) {
	srv, _, _, _ := setupServer(t)
	code, _ := getJSON(t, srv.Handler(), "/api/diff?category=process")
	if code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", code)
	}
}

// --- Sanity: Server.health output is a top-level list for /api/profiles ---

func TestProfileSummariesShape(t *testing.T) {
	// Lock the wire shape so future changes don't silently break the UI.
	srv, _, _, _ := setupServer(t)
	_, body := getJSON(t, srv.Handler(), "/api/profiles?slicer=bs&category=process")
	var raw []map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(raw) == 0 {
		t.Fatal("empty")
	}
	want := []string{"name", "setting_id", "inherits", "updated_time", "paired"}
	for _, k := range want {
		if _, ok := raw[0][k]; !ok {
			t.Errorf("missing key %q in profile summary", k)
		}
	}
	// 'name' from JSON should be "0.2 base" (BS fixture stores it).
	if !reflect.DeepEqual(raw[0]["name"], "0.2 base") {
		t.Errorf("name = %v, want %q", raw[0]["name"], "0.2 base")
	}
}

// --- Invalid JSON bodies ---

func TestSyncInvalidJSON(t *testing.T) {
	srv, _, _, _ := setupServer(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/sync", bytes.NewReader([]byte("{not json")))
	req.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestSettingsPutInvalidJSON(t *testing.T) {
	srv, _, _, _ := setupServer(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/settings", bytes.NewReader([]byte("{not json")))
	req.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

// urlQ is url.QueryEscape for inline use in test path strings.
func urlQ(s string) string { return url.QueryEscape(s) }
