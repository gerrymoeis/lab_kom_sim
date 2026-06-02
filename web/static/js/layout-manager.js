var PCLayoutManager = (function() {
  var layoutModal, gridData, selectedSlot, mode, onSelectCallback;
  var grid = [];
  var cadangan = [];
  var special = [];
  var maxRow = 5;
  var COLUMNS = 8;
  var busy = false;

  function init() {
    layoutModal = new bootstrap.Modal(document.getElementById('pcLayoutModal'));
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
    var html = '<div class="layout-grid-scroll">';
    for (var r = 0; r < Math.max(maxRow, grid.length); r++) {
      html += '<div class="d-flex align-items-center gap-2 mb-2 layout-row" data-row="' + (r + 1) + '">';
      if (mode === 'manager') {
        html += '<button type="button" class="btn btn-sm btn-outline-danger flex-shrink-0 btn-remove-row" style="padding:2px 6px;font-size:0.75rem" title="Pindahkan semua PC di baris ini ke cadangan"><i class="bi bi-trash"></i></button>';
      }
      html += '<span class="flex-shrink-0 small text-muted" style="width:48px">Baris ' + (r + 1) + '</span>';
      for (var c = 0; c < COLUMNS; c++) {
        var pc = (grid[r] && grid[r][c]) ? grid[r][c] : null;
        var selected = selectedSlot && selectedSlot.row === r && selectedSlot.col === c;
        var label = pc && pc.label ? pc.label : '-';
        var status = pc && pc.status ? pc.status : '';
        var filled = pc && pc.label;
        var cls = 'border rounded text-center flex-shrink-0 small';
        cls += ' ' + (selected ? 'border-primary border-2 shadow-sm' : 'border-secondary');
        cls += ' ' + (filled ? 'text-white fw-semibold' : 'text-muted bg-light');
        cls += ' ' + (filled ? (status === 'warning' ? 'bg-warning' : status === 'broken' ? 'bg-danger' : 'bg-success') : '');
        cls += ' cursor-pointer';
        html += '<div class="' + cls + '" style="width:80px;height:56px;display:flex;align-items:center;justify-content:center" data-row="' + r + '" data-col="' + c + '" data-label="' + (pc ? (pc.label || '') : '') + '" data-status="' + status + '" onclick="PCLayoutManager.onSlotClick(' + r + ',' + c + ')">' + label + '</div>';
      }
      html += '</div>';
    }
    html += '</div>';
    container.innerHTML = html;

    document.querySelectorAll('.layout-row').forEach(function(rowDiv) {
      var row = parseInt(rowDiv.dataset.row);
      var btn = rowDiv.querySelector('.btn-remove-row');
      if (btn) {
        btn.addEventListener('click', function(e) {
          e.stopPropagation();
          if (confirm('Pindahkan semua PC di baris ' + row + ' ke cadangan?')) {
            moveRowToCadangan(row);
          }
        });
      }
    });
  }

  function renderCadangan() {
    var container = document.getElementById('layoutCadanganBody');
    var html = '';
    cadangan.forEach(function(pc) {
      if (!pc.label) return;
      var selected = selectedSlot && selectedSlot.label === pc.label && selectedSlot.type === 'cadangan';
      var cls = 'border rounded small text-center flex-shrink-0 px-2 py-1';
      cls += ' ' + (selected ? 'border-primary border-2 shadow-sm' : 'border-secondary');
      cls += ' text-white fw-semibold';
      cls += ' ' + (pc.status === 'warning' ? 'bg-warning' : pc.status === 'broken' ? 'bg-danger' : 'bg-success');
      html += '<div class="' + cls + '" style="min-width:90px;cursor:pointer" data-label="' + pc.label + '" onclick="PCLayoutManager.onCadanganClick(\'' + pc.label + '\')">' + pc.label + '<br><small>' + (pc.status || '') + '</small></div>';
    });
    container.innerHTML = html || '<div class="text-muted small p-2">Tidak ada PC cadangan</div>';
  }

  function renderSpecial() {
    var container = document.getElementById('layoutSpecialBody');
    var html = '';
    special.forEach(function(pc) {
      if (!pc.label) return;
      var cls = 'border rounded small text-center flex-shrink-0 px-2 py-1 border-secondary bg-light';
      html += '<div class="' + cls + '" style="min-width:90px">' + pc.label + '<br><small>' + (pc.status || '') + '</small></div>';
    });
    container.innerHTML = html || '';
  }

  function renderActions() {
    var container = document.getElementById('layoutActions');
    var html = '<button type="button" class="btn btn-sm btn-outline-primary" onclick="PCLayoutManager.addRow()"><i class="bi bi-plus-lg"></i> Tambah Baris</button>';
    if (selectedSlot && selectedSlot.type === 'grid') {
      html += ' <button type="button" class="btn btn-sm btn-outline-secondary" onclick="PCLayoutManager.moveSelectedToCadangan()"><i class="bi bi-box-arrow-in-down-right"></i> Pindahkan ke Cadangan</button>';
    }
    container.innerHTML = html;
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

    var confirmMsg, op, body;

    if (selectedSlot.type === 'grid') {
      if (!label) {
        op = 'move';
        body = { label: selectedSlot.label, row: row + 1, col: col + 1 };
        confirmMsg = 'Pindahkan ' + selectedSlot.label + ' ke Baris ' + (row + 1) + ', Kolom ' + (col + 1) + '?';
      } else {
        op = 'swap';
        body = { a: selectedSlot.label, b: label };
        confirmMsg = 'Tukar ' + selectedSlot.label + ' dengan ' + label + '?';
      }
    } else if (selectedSlot.type === 'cadangan') {
      if (!label) {
        op = 'place';
        body = { label: selectedSlot.label, row: row + 1, col: col + 1 };
        confirmMsg = 'Tempatkan ' + selectedSlot.label + ' di Baris ' + (row + 1) + ', Kolom ' + (col + 1) + '?';
      } else {
        op = 'replace';
        body = { target: label, spare: selectedSlot.label };
        confirmMsg = 'Ganti ' + label + ' dengan ' + selectedSlot.label + '?';
      }
    }

    if (!confirm(confirmMsg)) return;
    selectedSlot = null;
    render();
    executeOperation(op, body);
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

    var gridLabel = selectedSlot.label;
    if (!confirm('Ganti ' + gridLabel + ' dengan ' + label + '?')) return;
    selectedSlot = null;
    render();
    executeOperation('replace', { target: gridLabel, spare: label });
  }

  function executeOperation(op, body) {
    if (busy) return;
    busy = true;

    fetch('/api/pc/' + op, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body)
    })
    .then(function(r) { return r.json(); })
    .then(function(data) {
      busy = false;
      if (data.success) {
        onDone(data.pcs || null);
      } else {
        alert('Gagal: ' + (data.error || 'unknown error'));
      }
    })
    .catch(function() {
      busy = false;
      alert('Gagal menghubungi server');
    });
  }

  function onDone(pcs) {
    selectedSlot = null;
    fetchLayout();
    refreshOrUpdateDashboard(pcs);
  }

  function refreshOrUpdateDashboard(pcs) {
    if (pcs && window.updateDashboardFromData) {
      window.updateDashboardFromData(pcs);
    } else if (window.refreshDashboardGrid) {
      window.refreshDashboardGrid(function(success) {
        if (!success) {
          console.warn('Dashboard refresh gagal, reload...');
          setTimeout(function() { location.reload(); }, 500);
        }
      });
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
        refreshOrUpdateDashboard(data.pcs || null);
      } else {
        alert('Gagal memindahkan baris');
      }
    }).catch(function(err) {
      console.error('moveRowToCadangan error:', err);
      alert('Gagal menghubungi server');
    });
  }

  function addRow() {
    maxRow++;
    render();
  }

  function moveSelectedToCadangan() {
    if (!selectedSlot || selectedSlot.type !== 'grid') return;
    var label = selectedSlot.label;
    if (!confirm('Pindahkan ' + label + ' ke cadangan?')) return;
    selectedSlot = null;
    render();
    executeOperation('move-to-cadangan', { label: label });
  }

  return {
    openPicker: openPicker,
    openManager: openManager,
    onSlotClick: onSlotClick,
    onCadanganClick: onCadanganClick,
    addRow: addRow,
    moveSelectedToCadangan: moveSelectedToCadangan,
    init: init
  };
})();

document.addEventListener('DOMContentLoaded', PCLayoutManager.init);
