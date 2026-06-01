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
    renderActions();
    updateMessage();
    document.getElementById('layoutSelectedInfo').textContent = '';
  }

  function renderGrid() {
    var container = document.getElementById('layoutGridBody');
    var html = '';
    for (var r = 0; r < Math.max(maxRow, grid.length); r++) {
      html += '<div class="layout-row" data-row="' + (r + 1) + '">';
      if (mode === 'manager') {
        html += '<button type="button" class="btn btn-sm btn-outline-danger layout-remove-row" data-row="' + (r + 1) + '" title="Pindahkan semua PC di baris ini ke cadangan"><i class="bi bi-trash"></i></button>';
      }
      html += '<span class="layout-row-label">Baris ' + (r + 1) + '</span>';
      for (var c = 0; c < COLUMNS; c++) {
        var pc = (grid[r] && grid[r][c]) ? grid[r][c] : null;
        var slotId = 'slot-' + r + '-' + c;
        var cls = 'layout-slot';
        var label = '-';
        var status = '';
        if (pc && pc.label) {
          cls += ' layout-filled layout-status-' + (pc.status || 'normal');
          label = pc.label;
          status = pc.status;
        } else {
          cls += ' layout-empty';
        }
        if (selectedSlot && selectedSlot.row === r && selectedSlot.col === c) {
          cls += ' layout-selected';
        }
        html += '<div class="' + cls + '" data-row="' + r + '" data-col="' + c + '" data-label="' + (pc ? (pc.label || '') : '') + '" data-status="' + status + '" onclick="PCLayoutManager.onSlotClick(' + r + ',' + c + ')">' + label + '</div>';
      }
      html += '</div>';
    }
    container.innerHTML = html;

    // Remove row handlers
    document.querySelectorAll('.layout-remove-row').forEach(function(btn) {
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
    var allSpare = cadangan.concat(special);
    allSpare.forEach(function(pc) {
      if (!pc.label) return;
      var cls = 'layout-cadangan-item layout-status-' + (pc.status || 'normal');
      if (selectedSlot && selectedSlot.label === pc.label && selectedSlot.type === 'cadangan') {
        cls += ' layout-selected';
      }
      html += '<div class="' + cls + '" data-label="' + pc.label + '" onclick="PCLayoutManager.onCadanganClick(\'' + pc.label + '\')">' + pc.label + '<br><small>' + (pc.status || '') + '</small></div>';
    });
    container.innerHTML = html || '<div class="text-muted small p-2">Tidak ada PC cadangan</div>';
  }

  function renderActions() {
    var container = document.getElementById('layoutActions');
    container.innerHTML = '<button type="button" class="btn btn-sm btn-outline-primary" onclick="PCLayoutManager.addRow()"><i class="bi bi-plus-lg"></i> Tambah Baris</button>';
  }

  function updateMessage() {
    var el = document.getElementById('layoutModeMessage');
    if (mode === 'manager') {
      el.className = 'alert alert-info small py-1 mb-2';
      el.textContent = 'Klik dua slot PC untuk menukar posisi, atau klik PC cadangan lalu slot tujuan untuk mengganti.';
    }
  }

  function onSlotClick(row, col) {
    var slot = document.querySelector('.layout-slot[data-row="' + row + '"][data-col="' + col + '"]');
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
      if (!label) return; // ignore empty slot as first selection in manager mode
      selectedSlot = { type: 'grid', row: row, col: col, label: label };
      render();
      document.getElementById('layoutSelectedInfo').textContent = 'Terpilih: ' + label;
      return;
    }

    // Second click
    var secondLabel = label;
    var isSecondCadangan = false;

    if (selectedSlot.type === 'grid') {
      if (selectedSlot.row === row && selectedSlot.col === col) {
        selectedSlot = null;
        render();
        document.getElementById('layoutSelectedInfo').textContent = '';
        return;
      }
      document.getElementById('layoutConfirmBtn').dataset.a = selectedSlot.label;
      document.getElementById('layoutConfirmBtn').dataset.b = secondLabel;
      document.getElementById('layoutConfirmBtn').dataset.op = 'swap';
      document.getElementById('layoutConfirmText').textContent = 'Tukar ' + selectedSlot.label + ' dengan ' + secondLabel + '?';
    } else if (selectedSlot.type === 'cadangan') {
      if (!secondLabel) {
        // user clicked empty slot → deselect
        selectedSlot = null;
        render();
        document.getElementById('layoutSelectedInfo').textContent = '';
        return;
      }
      document.getElementById('layoutConfirmBtn').dataset.target = secondLabel;
      document.getElementById('layoutConfirmBtn').dataset.spare = selectedSlot.label;
      document.getElementById('layoutConfirmBtn').dataset.op = 'replace';
      document.getElementById('layoutConfirmText').textContent = 'Ganti ' + secondLabel + ' dengan ' + selectedSlot.label + '?';
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
        // same item → deselect
        selectedSlot = null;
        render();
        document.getElementById('layoutSelectedInfo').textContent = '';
        return;
      }
      // different cadangan → switch selection
      selectedSlot = { type: 'cadangan', label: label };
      render();
      document.getElementById('layoutSelectedInfo').textContent = 'Terpilih: ' + label;
      return;
    }

    // Selected a grid slot first, now a cadangan → the user wants to replace
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
    var a, b, target, spare, row;

    switch (op) {
      case 'swap':
        a = btn.dataset.a;
        b = btn.dataset.b;
        if (!a || !b) return;
        fetch('/api/pc/swap', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ a: a, b: b })
        }).then(function(r) { return r.json(); }).then(function(data) {
          if (data.success) {
            selectedSlot = null;
            fetchLayout();
          } else {
            alert('Gagal menukar: ' + (data.error || 'unknown error'));
          }
        }).catch(function() { alert('Gagal menghubungi server'); });
        break;

      case 'replace':
        target = btn.dataset.target;
        spare = btn.dataset.spare;
        if (!target || !spare) return;
        fetch('/api/pc/replace', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ target: target, spare: spare })
        }).then(function(r) { return r.json(); }).then(function(data) {
          if (data.success) {
            selectedSlot = null;
            fetchLayout();
          } else {
            alert('Gagal mengganti: ' + (data.error || 'unknown error'));
          }
        }).catch(function() { alert('Gagal menghubungi server'); });
        break;
    }

    var confirmModalEl = document.getElementById('layoutConfirmDialog');
    var confirmModal = bootstrap.Modal.getInstance(confirmModalEl);
    if (confirmModal) confirmModal.hide();
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
