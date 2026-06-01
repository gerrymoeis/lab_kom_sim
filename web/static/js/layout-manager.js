var PCLayoutManager = (function() {
  var layoutModal, gridData, selectedSlot, mode, onSelectCallback;
  var grid = [];
  var cadangan = [];
  var special = [];
  var maxRow = 5;
  var COLUMNS = 8;

  function init() {
    layoutModal = new bootstrap.Modal(document.getElementById('pcLayoutModal'));
    document.getElementById('layoutConfirmBtn').addEventListener('click', onConfirm);
  }

  function openPicker(callback) {
    mode = 'picker';
    onSelectCallback = callback;
    selectedSlot = null;
    fetchLayout();
  }

  function openManager() {
    mode = 'manager';
    selectedSlot = null;
    document.getElementById('layoutModeMessage').className = 'd-none';
    fetchLayout();
  }

  function fetchLayout() {
    fetch('/api/pc/layout')
      .then(function(r) { return r.json(); })
      .then(function(data) {
        grid = data.grid || [];
        cadangan = data.cadangan || [];
        special = data.special || [];
        maxRow = data.maxRow || 5;
        render();
        layoutModal.show();
      })
      .catch(function() { alert('Gagal memuat data layout'); });
  }

  function render() {
    renderGrid();
    renderCadangan();
    renderSpecial();
    renderActions();
    updateMessage();
    document.getElementById('layoutSelectedInfo').textContent = '';
  }

  function renderGrid() {
    var container = document.getElementById('layoutGridBody');
    var html = '';
    for (var r = 0; r < Math.max(maxRow, grid.length); r++) {
      html += '<div class="d-flex align-items-center gap-1 mb-1">';
      if (mode === 'manager') {
        html += '<button type="button" class="btn btn-sm btn-outline-danger flex-shrink-0" style="padding:2px 4px;font-size:0.7rem" data-row="' + (r + 1) + '" title="Pindahkan semua PC di baris ini ke cadangan"><i class="bi bi-trash"></i></button>';
      }
      html += '<span class="flex-shrink-0 small text-muted" style="width:48px">Baris ' + (r + 1) + '</span>';
      for (var c = 0; c < COLUMNS; c++) {
        var pc = (grid[r] && grid[r][c]) ? grid[r][c] : null;
        var selected = selectedSlot && selectedSlot.row === r && selectedSlot.col === c;
        var label = pc && pc.label ? pc.label : '-';
        var status = pc && pc.status ? pc.status : '';
        var filled = pc && pc.label;
        var cls = 'border rounded text-center flex-shrink-0';
        cls += ' ' + (selected ? 'border-primary border-2 shadow-sm' : 'border-secondary');
        cls += ' ' + (filled ? 'text-white fw-semibold' : 'text-muted bg-light');
        cls += ' ' + (filled ? (status === 'warning' ? 'bg-warning' : status === 'broken' ? 'bg-danger' : 'bg-success') : '');
        cls += ' ' + (mode === 'picker' || mode === 'manager' ? 'cursor-pointer' : '');
        html += '<div class="' + cls + '" style="width:64px;height:48px;font-size:0.7rem;display:flex;align-items:center;justify-content:center;' + (mode === 'picker' || mode === 'manager' ? 'cursor:pointer' : '') + '" data-row="' + r + '" data-col="' + c + '" data-label="' + (pc ? (pc.label || '') : '') + '" data-status="' + status + '" onclick="PCLayoutManager.onSlotClick(' + r + ',' + c + ')">' + label + '</div>';
      }
      html += '</div>';
    }
    container.innerHTML = html;

    document.querySelectorAll('[data-row] > .btn-outline-danger').forEach(function(btn) {
      btn.addEventListener('click', function(e) {
        e.stopPropagation();
        var row = parseInt(this.dataset.row);
        if (confirm('Pindahkan semua PC di baris ' + row + ' ke cadangan?')) {
          moveRowToCadangan(row);
        }
      });
    });
  }

  function renderCadangan() {
    var container = document.getElementById('layoutCadanganBody');
    var html = '';
    cadangan.forEach(function(pc) {
      if (!pc.label) return;
      var selected = selectedSlot && selectedSlot.label === pc.label && selectedSlot.type === 'cadangan';
      var cls = 'border rounded p-1 small text-center flex-shrink-0';
      cls += ' ' + (selected ? 'border-primary border-2 shadow-sm' : 'border-secondary');
      cls += ' text-white fw-semibold';
      cls += ' ' + (pc.status === 'warning' ? 'bg-warning' : pc.status === 'broken' ? 'bg-danger' : 'bg-success');
      html += '<div class="' + cls + '" style="min-width:80px;cursor:pointer" data-label="' + pc.label + '" onclick="PCLayoutManager.onCadanganClick(\'' + pc.label + '\')">' + pc.label + '<br><small>' + (pc.status || '') + '</small></div>';
    });
    container.innerHTML = html || '<div class="text-muted small p-2">Tidak ada PC cadangan</div>';
  }

  function renderSpecial() {
    var container = document.getElementById('layoutSpecialBody');
    var html = '';
    special.forEach(function(pc) {
      if (!pc.label) return;
      var cls = 'border rounded p-1 small text-center flex-shrink-0 border-secondary bg-light';
      html += '<div class="' + cls + '" style="min-width:80px">' + pc.label + '<br><small>' + (pc.status || '') + '</small></div>';
    });
    container.innerHTML = html || '';
  }

  function renderActions() {
    var container = document.getElementById('layoutActions');
    container.innerHTML = '<button type="button" class="btn btn-sm btn-outline-primary" onclick="PCLayoutManager.addRow()"><i class="bi bi-plus-lg"></i> Tambah Baris</button>';
  }

  function updateMessage() {
    var el = document.getElementById('layoutModeMessage');
    if (mode === 'manager') {
      el.className = 'alert alert-info small py-1 mb-2';
      el.textContent = 'Klik PC untuk memilih, lalu klik tujuan (slot kosong/PC lain/cadangan).';
    }
  }

  function onSlotClick(row, col) {
    var slot = document.querySelector('[data-row="' + row + '"][data-col="' + col + '"]');
    var label = slot ? slot.dataset.label : '';
    var status = slot ? slot.dataset.status : '';

    if (mode === 'picker') {
      if (!label) {
        if (onSelectCallback) onSelectCallback(row + 1, col + 1);
        layoutModal.hide();
      }
      return;
    }

    if (!selectedSlot) {
      if (!label) return;
      selectedSlot = { type: 'grid', row: row, col: col, label: label };
      render();
      document.getElementById('layoutSelectedInfo').textContent = 'Terpilih: ' + label;
      return;
    }

    if (selectedSlot.row === row && selectedSlot.col === col && selectedSlot.type === 'grid') {
      selectedSlot = null;
      render();
      document.getElementById('layoutSelectedInfo').textContent = '';
      return;
    }

    var confirmBtn = document.getElementById('layoutConfirmBtn');
    var confirmText = document.getElementById('layoutConfirmText');

    if (selectedSlot.type === 'grid') {
      if (!label) {
        // Filled → Empty slot: Move
        confirmBtn.dataset.label = selectedSlot.label;
        confirmBtn.dataset.row = row + 1;
        confirmBtn.dataset.col = col + 1;
        confirmBtn.dataset.op = 'move';
        confirmText.textContent = 'Pindahkan ' + selectedSlot.label + ' ke Baris ' + (row + 1) + ', Kolom ' + (col + 1) + '?';
      } else {
        // Filled → Filled: Swap
        confirmBtn.dataset.a = selectedSlot.label;
        confirmBtn.dataset.b = label;
        confirmBtn.dataset.op = 'swap';
        confirmText.textContent = 'Tukar ' + selectedSlot.label + ' dengan ' + label + '?';
      }
    } else if (selectedSlot.type === 'cadangan') {
      if (!label) {
        // Cadangan → Empty slot: Place
        confirmBtn.dataset.label = selectedSlot.label;
        confirmBtn.dataset.row = row + 1;
        confirmBtn.dataset.col = col + 1;
        confirmBtn.dataset.op = 'place';
        confirmText.textContent = 'Tempatkan ' + selectedSlot.label + ' di Baris ' + (row + 1) + ', Kolom ' + (col + 1) + '?';
      } else {
        // Cadangan → Filled: Replace
        confirmBtn.dataset.target = label;
        confirmBtn.dataset.spare = selectedSlot.label;
        confirmBtn.dataset.op = 'replace';
        confirmText.textContent = 'Ganti ' + label + ' dengan ' + selectedSlot.label + '?';
      }
    }

    var confirmModal = new bootstrap.Modal(document.getElementById('layoutConfirmDialog'));
    confirmModal.show();
  }

  function onCadanganClick(label) {
    if (mode === 'picker') return;

    if (!selectedSlot) {
      selectedSlot = { type: 'cadangan', label: label };
      render();
      document.getElementById('layoutSelectedInfo').textContent = 'Terpilih: ' + label;
      return;
    }

    if (selectedSlot.type === 'cadangan') {
      if (selectedSlot.label === label) {
        selectedSlot = null;
        render();
        document.getElementById('layoutSelectedInfo').textContent = '';
        return;
      }
      selectedSlot = { type: 'cadangan', label: label };
      render();
      document.getElementById('layoutSelectedInfo').textContent = 'Terpilih: ' + label;
      return;
    }

    // Grid → Cadangan: Replace
    document.getElementById('layoutConfirmBtn').dataset.target = selectedSlot.label;
    document.getElementById('layoutConfirmBtn').dataset.spare = label;
    document.getElementById('layoutConfirmBtn').dataset.op = 'replace';
    document.getElementById('layoutConfirmText').textContent = 'Ganti ' + selectedSlot.label + ' dengan ' + label + '?';

    var confirmModal = new bootstrap.Modal(document.getElementById('layoutConfirmDialog'));
    confirmModal.show();
  }

  function onConfirm() {
    var btn = document.getElementById('layoutConfirmBtn');
    var op = btn.dataset.op;
    var a, b, target, spare, label, row, col;

    function onDone() {
      selectedSlot = null;
      var confirmModalEl = document.getElementById('layoutConfirmDialog');
      var confirmModal = bootstrap.Modal.getInstance(confirmModalEl);
      if (confirmModal) confirmModal.hide();
      fetchLayout();
      if (window.refreshDashboardGrid) window.refreshDashboardGrid();
    }

    function handleResponse(r) { return r.json(); }

    function handleSuccess(data) {
      if (data.success) { onDone(); }
      else { alert('Gagal: ' + (data.error || 'unknown error')); }
    }

    function handleError() { alert('Gagal menghubungi server'); }

    switch (op) {
      case 'swap':
        a = btn.dataset.a;
        b = btn.dataset.b;
        if (!a || !b) return;
        fetch('/api/pc/swap', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ a: a, b: b })
        }).then(handleResponse).then(handleSuccess).catch(handleError);
        break;

      case 'replace':
        target = btn.dataset.target;
        spare = btn.dataset.spare;
        if (!target || !spare) return;
        fetch('/api/pc/replace', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ target: target, spare: spare })
        }).then(handleResponse).then(handleSuccess).catch(handleError);
        break;

      case 'move':
        label = btn.dataset.label;
        row = parseInt(btn.dataset.row);
        col = parseInt(btn.dataset.col);
        if (!label || !row || !col) return;
        fetch('/api/pc/move', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ label: label, row: row, col: col })
        }).then(handleResponse).then(handleSuccess).catch(handleError);
        break;

      case 'place':
        label = btn.dataset.label;
        row = parseInt(btn.dataset.row);
        col = parseInt(btn.dataset.col);
        if (!label || !row || !col) return;
        fetch('/api/pc/place', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ label: label, row: row, col: col })
        }).then(handleResponse).then(handleSuccess).catch(handleError);
        break;
    }
  }

  function moveRowToCadangan(row) {
    fetch('/api/pc/move-row', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ row: row })
    }).then(function(r) { return r.json(); }).then(function(data) {
      if (data.success) {
        selectedSlot = null;
        fetchLayout();
        if (window.refreshDashboardGrid) window.refreshDashboardGrid();
      } else {
        alert('Gagal memindahkan baris');
      }
    }).catch(function() { alert('Gagal menghubungi server'); });
  }

  function addRow() {
    maxRow++;
    render();
  }

  return {
    openPicker: openPicker,
    openManager: openManager,
    onSlotClick: onSlotClick,
    onCadanganClick: onCadanganClick,
    addRow: addRow,
    onConfirm: onConfirm,
    init: init
  };
})();

document.addEventListener('DOMContentLoaded', PCLayoutManager.init);
