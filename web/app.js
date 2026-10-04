// BBOrcaProfsync UI — Browse / History / Settings tabs.
// Plan §10 (UI), §6 (diff model), §11 (API), §14 (MVP scope).
(function () {
  'use strict';

  // ---------- Utilities ----------
  function $(sel) { return document.querySelector(sel); }
  function $$(sel) { return document.querySelectorAll(sel); }

  function el(tag, attrs, children) {
    var node = document.createElement(tag);
    if (attrs) {
      Object.keys(attrs).forEach(function (k) {
        if (k === 'class') node.className = attrs[k];
        else if (k === 'html') node.innerHTML = attrs[k];
        else if (k.indexOf('on') === 0 && typeof attrs[k] === 'function') {
          node.addEventListener(k.substring(2).toLowerCase(), attrs[k]);
        } else if (attrs[k] === true) node.setAttribute(k, '');
        else if (attrs[k] !== false && attrs[k] != null) node.setAttribute(k, attrs[k]);
      });
    }
    if (children) {
      children.forEach(function (c) {
        if (c == null) return;
        node.appendChild(typeof c === 'string' ? document.createTextNode(c) : c);
      });
    }
    return node;
  }

  function escapeHtml(s) {
    return String(s).replace(/[&<>"']/g, function (c) {
      return ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[c];
    });
  }

  function formatTime(ts) {
    if (!ts || ts <= 0) return '—';
    try {
      var d = new Date(ts * 1000);
      return d.toLocaleString();
    } catch (e) { return String(ts); }
  }

  function formatValue(v) {
    if (v === null || v === undefined) return '(none)';
    if (Array.isArray(v)) {
      if (v.length === 0) return '[]';
      return v.map(formatValue).join(', ');
    }
    if (typeof v === 'object') {
      try { return JSON.stringify(v); } catch (e) { return String(v); }
    }
    return String(v);
  }

  function showError(msg) {
    var banner = $('#error-banner');
    banner.textContent = msg;
    banner.hidden = false;
    setTimeout(function () { banner.hidden = true; }, 6000);
  }

  function showToast(msg) {
    var t = $('#toast');
    t.textContent = msg;
    t.hidden = false;
    setTimeout(function () { t.hidden = true; }, 4000);
  }

  async function api(path, opts) {
    opts = opts || {};
    var res;
    try {
      res = await fetch(path, opts);
    } catch (e) {
      throw new Error('Network error: ' + e.message);
    }
    var text = await res.text();
    if (!res.ok) throw new Error('HTTP ' + res.status + ': ' + (text || res.statusText));
    if (!text) return null;
    try {
      return JSON.parse(text);
    } catch (e) {
      return text;
    }
  }

  // ---------- State ----------
  var state = {
    category: 'process',
    bsProfiles: [],
    orcaProfiles: [],
    pairedMap: {},
    currentDiff: null,
    diffContext: null,
    diffCategory: null,
    direction: 'bs-to-orca', // 'bs-to-orca' | 'orca-to-bs'
    selectedFields: {},
    showMatches: false,
  };

  // ---------- Tab management ----------
  function showTab(name) {
    var btns = $$('.tab-btn');
    btns.forEach(function (b) {
      if (b.dataset.tab === name) b.classList.add('active');
      else b.classList.remove('active');
    });
    $$('.tab-section').forEach(function (s) {
      s.hidden = s.id !== ('tab-' + name);
    });
    if (name === 'history') loadHistory();
    if (name === 'settings') loadSettings();
  }

  // ---------- Health ----------
  async function checkHealth() {
    var badge = $('#health-badge');
    var text = $('#health-text');
    try {
      var h = await api('/api/health');
      badge.classList.remove('unknown', 'err');
      badge.classList.add('ok');
      badge.title = 'Server is up';
      if (h && h.vault_status) {
        var vs = h.vault_status;
        text.textContent = vs.initialized
          ? 'vault: ' + (vs.path || 'ready')
          : 'vault: off' + (vs.path ? ' — ' + vs.path : '');
      } else {
        text.textContent = 'ok';
      }
    } catch (e) {
      badge.classList.remove('unknown', 'ok');
      badge.classList.add('err');
      badge.title = 'Server unreachable: ' + e.message;
      text.textContent = 'offline';
    }
  }

  // ---------- Browse: load profiles ----------
  async function loadProfiles() {
    var container = $('#profile-list-container');
    container.innerHTML = '';
    container.appendChild(el('p', { class: 'placeholder' }, ['Loading profiles…']));

    var cat = state.category;
    try {
      var results = await Promise.all([
        api('/api/profiles?slicer=bs&category=' + encodeURIComponent(cat)),
        api('/api/profiles?slicer=orca&category=' + encodeURIComponent(cat)),
      ]);
      state.bsProfiles = Array.isArray(results[0]) ? results[0] : [];
      state.orcaProfiles = Array.isArray(results[1]) ? results[1] : [];
      buildPairedMap();
      renderProfileList();
    } catch (e) {
      container.innerHTML = '';
      container.appendChild(el('p', { class: 'placeholder' }, ['Failed to load profiles: ' + e.message]));
    }
  }

  function buildPairedMap() {
    state.pairedMap = {};
    state.bsProfiles.forEach(function (p) {
      var name = p.name;
      if (!state.pairedMap[name]) state.pairedMap[name] = { bs: null, orca: null };
      state.pairedMap[name].bs = p;
    });
    state.orcaProfiles.forEach(function (p) {
      var name = p.name;
      if (!state.pairedMap[name]) state.pairedMap[name] = { bs: null, orca: null };
      state.pairedMap[name].orca = p;
    });
  }

  function renderProfileList() {
    var container = $('#profile-list-container');
    container.innerHTML = '';
    var names = Object.keys(state.pairedMap).sort();
    if (names.length === 0) {
      container.appendChild(el('p', { class: 'placeholder' }, ['No profiles found for category "' + state.category + '".']));
      return;
    }

    var table = el('table', { class: 'profile-table' });
    var thead = el('thead', null, [
      el('tr', null, [
        el('th', null, ['Name']),
        el('th', null, ['BambuStudio ID']),
        el('th', null, ['OrcaSlicer ID']),
        el('th', null, ['Status']),
        el('th', null, ['']),
      ])
    ]);
    table.appendChild(thead);
    var tbody = el('tbody');
    names.forEach(function (name) {
      var pair = state.pairedMap[name];
      var paired = !!(pair.bs && pair.orca);
      var tr = el('tr');
      tr.appendChild(el('td', { class: 'name' }, [name]));
      tr.appendChild(el('td', null, [
        pair.bs ? el('span', { class: 'setting-id' }, [pair.bs.setting_id || '(no id)']) : el('span', { class: 'setting-id' }, ['—'])
      ]));
      tr.appendChild(el('td', null, [
        pair.orca ? el('span', { class: 'setting-id' }, [pair.orca.setting_id || '(no id)']) : el('span', { class: 'setting-id' }, ['—'])
      ]));
      var badge = el('span', { class: 'badge ' + (paired ? 'badge-paired' : '') }, [paired ? 'paired' : (pair.bs ? 'BS only' : 'Orca only')]);
      tr.appendChild(el('td', null, [badge]));
      var btn = el('button', { class: 'btn btn-small', onclick: function () { showDiff(name); } }, ['Diff']);
      if (!paired) btn.disabled = true;
      tr.appendChild(el('td', null, [btn]));
      tbody.appendChild(tr);
    });
    table.appendChild(tbody);
    container.appendChild(table);
  }

  // ---------- Diff panel ----------
  async function showDiff(name) {
    var panel = $('#diff-panel');
    panel.hidden = false;
    $('#diff-title').textContent = 'Diff: ' + name;
    $('#diff-context').innerHTML = '<span class="placeholder">Loading diff…</span>';
    $('#diff-table-container').innerHTML = '';
    $('#diff-actions').innerHTML = '';

    var cat = state.category;
    try {
      var url = '/api/diff?category=' + encodeURIComponent(cat)
        + '&bsName=' + encodeURIComponent(name)
        + '&orcaName=' + encodeURIComponent(name);
      var diff = await api(url);
      state.currentDiff = diff;
      state.diffContext = diff && diff.context ? diff.context : {};
      state.diffCategory = cat;
      state.selectedFields = {};
      renderDiff();
    } catch (e) {
      $('#diff-context').innerHTML = '';
      $('#diff-context').appendChild(el('p', { class: 'placeholder' }, ['Failed to load diff: ' + e.message]));
    }
  }

  function closeDiff() {
    $('#diff-panel').hidden = true;
    state.currentDiff = null;
  }

  function renderDiff() {
    var diff = state.currentDiff;
    var ctx = state.diffContext || {};
    // Context block
    var ctxEl = $('#diff-context');
    ctxEl.innerHTML = '';
    var ctxItems = [
      { label: 'Category', value: state.diffCategory || '' },
      { label: 'BambuStudio name', value: ctx.source_name || '' },
      { label: 'BambuStudio setting_id', value: ctx.source_setting_id || '(none)' },
      { label: 'BambuStudio version', value: ctx.source_version || '(none)' },
      { label: 'BambuStudio inherits', value: ctx.source_inherits || '(none)' },
      { label: 'BambuStudio base_id', value: ctx.source_base_id || '(none)' },
      { label: 'OrcaSlicer name', value: ctx.dest_name || '' },
      { label: 'OrcaSlicer setting_id', value: ctx.dest_setting_id || '(none)' },
      { label: 'OrcaSlicer version', value: ctx.dest_version || '(none)' },
      { label: 'OrcaSlicer inherits', value: ctx.dest_inherits || '(none)' },
      { label: 'OrcaSlicer base_id', value: ctx.dest_base_id || '(none)' },
    ];
    ctxItems.forEach(function (item) {
      ctxEl.appendChild(el('div', { class: 'ctx-item' }, [
        el('span', { class: 'ctx-label' }, [item.label]),
        el('span', { class: 'ctx-value' }, [item.value || '—']),
      ]));
    });

    // Diff table
    var tableContainer = $('#diff-table-container');
    tableContainer.innerHTML = '';
    var fields = (diff && diff.fields) || [];
    var matchFields = fields.filter(function (f) { return f.status === 'match'; });
    var activeFields = fields.filter(function (f) { return f.status !== 'match'; });

    if (fields.length === 0) {
      tableContainer.appendChild(el('p', { class: 'placeholder' }, ['No fields in diff (empty profile or API returned no data).']));
    } else {
      var table = el('table', { class: 'diff-table' });
      table.appendChild(el('thead', null, [
        el('tr', null, [
          el('th', null, ['Field']),
          el('th', null, ['BambuStudio']),
          el('th', null, ['OrcaSlicer']),
          el('th', { style: 'width:140px' }, ['Action']),
        ])
      ]));
      var tbody = el('tbody');

      activeFields.forEach(function (f) {
        tbody.appendChild(renderDiffRow(f, false));
      });
      if (matchFields.length > 0) {
        if (state.showMatches) {
          matchFields.forEach(function (f) {
            tbody.appendChild(renderDiffRow(f, true));
          });
        }
        var toggle = el('div', { class: 'diff-toggle-row', onclick: function () {
          state.showMatches = !state.showMatches;
          renderDiff();
        } }, [
          state.showMatches
              ? '▾ Hide ' + matchFields.length + ' matching field' + (matchFields.length === 1 ? '' : 's')
              : '▸ Show ' + matchFields.length + ' matching field' + (matchFields.length === 1 ? '' : 's')
        ]);
        tableContainer.appendChild(table);
        table.appendChild(tbody);
        tableContainer.appendChild(toggle);
      } else {
        table.appendChild(tbody);
        tableContainer.appendChild(table);
      }
    }

    renderDiffActions();
  }

  function renderDiffRow(f, isMatch) {
    var rowClass = 'row-' + (f.status || 'unknown');
    if (isMatch) rowClass = 'row-match';
    var tr = el('tr', { class: rowClass });

    var kindTag = f.kind ? el('span', { class: 'field-kind-tag', title: 'Field kind: ' + f.kind }, [f.kind]) : null;

    var statusBadge = null;
    if (f.status === 'differ') statusBadge = el('span', { class: 'badge', style: 'background:var(--amber-bg);color:var(--amber)' }, ['differ']);
    else if (f.status === 'only-source') statusBadge = el('span', { class: 'badge', style: 'background:var(--amber-bg);color:var(--amber)' }, ['only BS']);
    else if (f.status === 'only-dest') statusBadge = el('span', { class: 'badge', style: 'background:var(--amber-bg);color:var(--amber)' }, ['only Orca']);
    else if (f.status === 'match') statusBadge = el('span', { class: 'badge', style: 'background:var(--green-bg);color:var(--green)' }, ['match']);

    tr.appendChild(el('td', null, [
      document.createTextNode(f.name + ' '),
      kindTag,
    ]));
    tr.appendChild(el('td', { class: 'val-cell' }, [formatValue(f.source_value)]));
    tr.appendChild(el('td', { class: 'val-cell' }, [formatValue(f.dest_value)]));
    var actionCell = el('td');
    if (statusBadge) actionCell.appendChild(statusBadge);
    if (!isMatch && f.selectable) {
      var checkbox = el('input', {
        type: 'checkbox',
        id: 'sel-' + f.name,
        onchange: function (e) {
          if (e.target.checked) state.selectedFields[f.name] = true;
          else delete state.selectedFields[f.name];
          renderDiffActions();
        }
      });
      actionCell.appendChild(document.createTextNode(' '));
      actionCell.appendChild(el('label', { for: 'sel-' + f.name, style: 'display:inline;flex-direction:row;font-size:0.8rem' }, [
        checkbox,
        document.createTextNode(' include'),
      ]));
    }
    tr.appendChild(actionCell);
    return tr;
  }

  function renderDiffActions() {
    var actions = $('#diff-actions');
    actions.innerHTML = '';
    var diff = state.currentDiff;
    if (!diff || !diff.fields || diff.fields.length === 0) return;

    var selectableCount = diff.fields.filter(function (f) { return f.selectable; }).length;
    var selectedCount = Object.keys(state.selectedFields).length;

    var dirLabel = el('label', null, [
      document.createTextNode(' Direction: '),
      (function () {
        var sel = el('select', { onchange: function (e) { state.direction = e.target.value; } });
        sel.appendChild(el('option', { value: 'bs-to-orca' }, ['BambuStudio → OrcaSlicer']));
        sel.appendChild(el('option', { value: 'orca-to-bs' }, ['OrcaSlicer → BambuStudio']));
        sel.value = state.direction;
        return sel;
      })()
    ]);
    actions.appendChild(dirLabel);

    var summary = el('span', { style: 'font-size:0.85rem;color:var(--text-muted)' }, [
      selectedCount + ' of ' + selectableCount + ' selectable fields marked',
    ]);
    actions.appendChild(summary);

    var applyBtn = el('button', {
      class: 'btn btn-primary',
      onclick: applySync,
    }, ['Apply selected (' + selectedCount + ')']);
    if (selectedCount === 0) applyBtn.disabled = true;
    actions.appendChild(applyBtn);
  }

  async function applySync() {
    var diff = state.currentDiff;
    if (!diff) return;
    var ctx = state.diffContext || {};
    var selected = Object.keys(state.selectedFields);
    if (selected.length === 0) {
      showError('No fields selected.');
      return;
    }
    var fieldsMap = {};
    var skipped = [];
    selected.forEach(function (name) {
      var f = diff.fields.find(function (x) { return x.name === name; });
      if (!f) return;
      var value;
      if (state.direction === 'bs-to-orca') {
        if (f.status === 'only-dest') {
          skipped.push(name + ' (only in Orca)');
          return;
        }
        value = f.source_value;
      } else {
        if (f.status === 'only-source') {
          skipped.push(name + ' (only in BS)');
          return;
        }
        value = f.dest_value;
      }
      fieldsMap[name] = value;
    });
    if (Object.keys(fieldsMap).length === 0) {
      showError('None of the selected fields can be synced in this direction. Skipped: ' + skipped.join(', '));
      return;
    }

    var destSlicer = state.direction === 'bs-to-orca' ? 'orcaslicer' : 'bambustudio';
    var destName = state.direction === 'bs-to-orca' ? (ctx.dest_name || '') : (ctx.source_name || '');

    var body = {
      category: state.diffCategory,
      dest_slicer: destSlicer,
      dest_name: destName,
      is_new: false,
      fields: fieldsMap,
    };

    try {
      var res = await api('/api/sync', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      });
      var commit = res && res.commit ? res.commit : '(no commit)';
      var files = res && res.written_files ? res.written_files.join(', ') : '';
      var msg = 'Synced ' + Object.keys(fieldsMap).length + ' field(s). Commit: ' + commit + (files ? ' Files: ' + files : '');
      showToast(msg);
      if (skipped.length > 0) {
        showToast('Skipped: ' + skipped.join(', '));
      }
      // Reload profiles and diff after sync
      await loadProfiles();
      showDiff(destName || ctx.source_name);
    } catch (e) {
      showError('Sync failed: ' + e.message);
    }
  }

  // ---------- History ----------
  async function loadHistory() {
    var container = $('#history-container');
    container.innerHTML = '';
    container.appendChild(el('p', { class: 'placeholder' }, ['Loading history…']));
    try {
      var entries = await api('/api/history');
      if (!Array.isArray(entries)) {
        container.innerHTML = '';
        container.appendChild(el('p', { class: 'placeholder' }, ['No history available.']));
        return;
      }
      if (entries.length === 0) {
        container.innerHTML = '';
        container.appendChild(el('p', { class: 'placeholder' }, ['No commits yet.']));
        return;
      }
      container.innerHTML = '';
      entries.forEach(function (e) {
        var hashShort = (e.hash || '').substring(0, 7);
        var details = el('details', { class: 'history-item' }, [
          el('summary', null, [
            el('span', { class: 'history-hash' }, [hashShort || '(no hash)']),
            el('span', { class: 'history-msg' }, [e.message || '(no message)']),
            el('span', { class: 'history-time' }, [formatTime(e.timestamp)]),
          ]),
          el('div', { class: 'history-full' }, [
            el('div', null, [document.createTextNode('Hash: ' + (e.hash || ''))]),
            el('div', null, [document.createTextNode('Timestamp: ' + formatTime(e.timestamp))]),
            el('div', null, [document.createTextNode('Message: ' + (e.message || ''))]),
          ]),
        ]);
        container.appendChild(details);
      });
    } catch (e) {
      container.innerHTML = '';
      container.appendChild(el('p', { class: 'placeholder' }, ['Failed to load history: ' + e.message]));
    }
  }

  // ---------- Settings ----------
  async function loadSettings() {
    var feedback = $('#settings-feedback');
    feedback.textContent = '';
    feedback.className = 'feedback';
    try {
      var s = await api('/api/settings');
      fillSettings(s || {});
    } catch (e) {
      fillSettings({});
      feedback.textContent = 'Could not load settings (using defaults): ' + e.message;
      feedback.className = 'feedback err';
    }
  }

  function fillSettings(s) {
    var get = function (obj, path) {
      var parts = path.split('.');
      var cur = obj;
      for (var i = 0; i < parts.length; i++) {
        if (cur && typeof cur === 'object' && parts[i] in cur) cur = cur[parts[i]];
        else return undefined;
      }
      return cur;
    };
    var setVal = function (id, v) {
      var node = document.getElementById(id);
      if (node) node.value = v == null ? '' : v;
    };
    setVal('set-bs-user-dir', get(s, 'slicers.bambustudio.user_dir'));
    setVal('set-bs-user-id', get(s, 'slicers.bambustudio.user_id'));
    setVal('set-orca-user-dir', get(s, 'slicers.orcaslicer.user_dir'));
    setVal('set-orca-user-id', get(s, 'slicers.orcaslicer.user_id'));
    setVal('set-vault-path', get(s, 'vault.path'));
    setVal('set-server-port', get(s, 'server.port') || 9876);
    setVal('set-bs-version', get(s, 'slicer_versions.bambustudio'));
    setVal('set-orca-version', get(s, 'slicer_versions.orcaslicer'));
  }

  function buildSettingsPayload() {
    var getVal = function (id) {
      var node = document.getElementById(id);
      return node ? node.value : '';
    };
    var portStr = getVal('set-server-port');
    var port = parseInt(portStr, 10);
    if (!isFinite(port) || port <= 0) port = 9876;
    return {
      slicers: {
        bambustudio: {
          user_dir: getVal('set-bs-user-dir'),
          user_id: getVal('set-bs-user-id'),
        },
        orcaslicer: {
          user_dir: getVal('set-orca-user-dir'),
          user_id: getVal('set-orca-user-id'),
        },
      },
      vault: { path: getVal('set-vault-path') },
      server: { port: port },
      slicer_versions: {
        bambustudio: getVal('set-bs-version'),
        orcaslicer: getVal('set-orca-version'),
      },
    };
  }

  async function saveSettings(e) {
    e.preventDefault();
    var feedback = $('#settings-feedback');
    feedback.textContent = 'Saving…';
    feedback.className = 'feedback';
    var payload = buildSettingsPayload();
    try {
      await api('/api/settings', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      });
      feedback.textContent = 'Settings saved.';
      feedback.className = 'feedback ok';
      checkHealth();
    } catch (err) {
      feedback.textContent = 'Save failed: ' + err.message;
      feedback.className = 'feedback err';
    }
  }

  // ---------- Init ----------
  function init() {
    // Tabs
    $$('.tab-btn').forEach(function (btn) {
      btn.addEventListener('click', function () { showTab(btn.dataset.tab); });
    });

    // Category selector
    $('#category-select').addEventListener('change', function (e) {
      state.category = e.target.value;
      closeDiff();
      loadProfiles();
    });

    // Refresh buttons
    $('#refresh-profiles').addEventListener('click', function () {
      closeDiff();
      loadProfiles();
    });
    $('#refresh-history').addEventListener('click', loadHistory);
    $('#refresh-settings').addEventListener('click', loadSettings);

    // Diff close
    $('#diff-close').addEventListener('click', closeDiff);

    // Settings form
    $('#settings-form').addEventListener('submit', saveSettings);

    // Prevent number inputs from changing value on wheel scroll.
    $$('input[type="number"]').forEach(function (input) {
      input.addEventListener('wheel', function (e) { e.preventDefault(); }, { passive: false });
    });

    showTab('browse');
    checkHealth();
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
