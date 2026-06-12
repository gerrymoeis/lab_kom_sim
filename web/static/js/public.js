(function () {
  'use strict'

  var SORT_ASC = 1
  var SORT_DESC = -1
  var DAY_NAMES = ['Minggu', 'Senin', 'Selasa', 'Rabu', 'Kamis', 'Jumat', 'Sabtu']

  function initPublicTable(config) {
    var container = document.getElementById(config.containerId)
    if (!container) return

    var data = config.data || []
    var columns = config.columns || []
    var searchFields = config.searchFields || []
    var pageSize = config.pageSize || 25
    var groupBy = config.groupBy || null
    var groupConfig = config.groupConfig || {}
    var highlightToday = config.highlightToday || false
    var todayName = highlightToday ? DAY_NAMES[new Date().getDay()] : null

    var state = {
      all: data.slice(),
      filtered: data.slice(),
      sortField: null,
      sortDir: SORT_ASC,
      searchTerm: '',
      filters: {},
      page: 1,
    }

    function render() {
      applyFilters(state)

      var total = state.filtered.length

      ensureSearchBar(total)

      var html = ''
      html += buildFilters()

      if (groupBy) {
        html += buildInfoText(total > 0 ? 1 : 0, total, total)
        html += buildGroupedTable(state.filtered)
      } else {
        var pages = Math.ceil(total / pageSize) || 1
        if (state.page > pages) state.page = pages
        var start = (state.page - 1) * pageSize
        var pageData = state.filtered.slice(start, start + pageSize)
        var infoStart = total > 0 ? start + 1 : 0
        var infoEnd = Math.min(start + pageSize, total)
        html += buildInfoText(infoStart, infoEnd, total)
        html += buildPagination(total, pages)
        html += buildTable(pageData)
        html += buildPagination(total, pages)
      }

      var contentEl = container.querySelector('.ps-content')
      if (!contentEl) {
        contentEl = document.createElement('div')
        contentEl.className = 'ps-content'
        container.appendChild(contentEl)
      }
      contentEl.innerHTML = html
      bindEvents()
    }

    function applyFilters(s) {
      s.filtered = s.all.slice()

      if (s.searchTerm) {
        var q = s.searchTerm.toLowerCase()
        s.filtered = s.filtered.filter(function (row) {
          return searchFields.some(function (field) {
            var val = row[field]
            return val != null && String(val).toLowerCase().indexOf(q) !== -1
          })
        })
      }

      Object.keys(s.filters).forEach(function (field) {
        var fv = s.filters[field]
        if (!fv) return
        s.filtered = s.filtered.filter(function (row) {
          return String(row[field] || '') === fv
        })
      })

      if (s.sortField) {
        s.filtered.sort(function (a, b) {
          var va = a[s.sortField]
          var vb = b[s.sortField]
          if (va == null) va = ''
          if (vb == null) vb = ''
          if (typeof va === 'number' && typeof vb === 'number') {
            return (va - vb) * s.sortDir
          }
          return String(va).localeCompare(String(vb)) * s.sortDir
        })
      }
    }

    function ensureSearchBar(total) {
      var wrap = container.querySelector('.ps-search-bar-wrap')
      if (!wrap) {
        wrap = document.createElement('div')
        wrap.className = 'ps-search-bar-wrap'
        container.insertBefore(wrap, container.firstChild)
      }
      if (!wrap.hasChildNodes()) {
        wrap.innerHTML = buildSearchBar(total)
        var input = document.getElementById('ps-search')
        if (input) {
          input.addEventListener('input', function (e) {
            state.searchTerm = e.target.value
            state.page = 1
            render()
          })
        }
      } else {
        var countEl = wrap.querySelector('.text-muted')
        if (countEl) countEl.textContent = total + ' total data'
      }
    }

    function buildSearchBar(total) {
      var h = '<div class="row mb-3 align-items-center">'
      h += '<div class="col-md-6">'
      h += '<input type="text" id="ps-search" class="form-control" placeholder="Cari..." value="' + esc(state.searchTerm) + '">'
      h += '</div>'
      h += '<div class="col-md-6 text-md-end">'
      h += '<small class="text-muted">' + total + ' total data</small>'
      h += '</div></div>'
      return h
    }

    function buildInfoText(start, end, total) {
      if (total === 0) return ''
      return '<p class="text-muted my-2">Menampilkan ' + start + ' - ' + end + ' dari ' + total + ' data</p>'
    }

    function buildFilters() {
      var filterCols = columns.filter(function (c) { return c.filterable })

      var h = '<div class="row mb-3 g-2">'
      filterCols.forEach(function (col) {
        var vals = {}
        state.all.forEach(function (row) {
          var v = row[col.field]
          if (v != null && v !== '') vals[v] = (vals[v] || 0) + 1
        })
        var keys = Object.keys(vals).sort()
        h += '<div class="col-md-3">'
        h += '<select class="form-select form-select-sm ps-filter" data-field="' + col.field + '">'
        h += '<option value="">' + col.label + ' (Semua)</option>'
        keys.forEach(function (k) {
          var sel = state.filters[col.field] === k ? ' selected' : ''
          h += '<option value="' + esc(k) + '"' + sel + '>' + esc(k) + ' (' + vals[k] + ')</option>'
        })
        h += '</select></div>'
      })
      h += '<div class="col-md-auto d-flex align-items-end ms-auto">'
      h += '<button type="button" class="btn btn-sm btn-outline-secondary ps-reset"><i class="bi bi-x-circle"></i> Reset</button>'
      h += '</div></div>'
      return h
    }

    function buildTable(pageData) {
      var h = '<div class="table-responsive"><table class="table table-hover table-bordered table-striped">'
      h += '<thead class="table-dark"><tr>'
      columns.forEach(function (col) {
        var sortable = col.sortable !== false
        var cls = sortable ? 'ps-sortable' : ''
        var field = col.field
        if (sortFieldClass(field)) cls += ' ' + sortFieldClass(field)
        h += '<th class="' + cls + '" data-field="' + field + '" data-sortable="' + sortable + '">'
        h += col.label
        h += '</th>'
      })
      h += '</tr></thead><tbody>'

      if (!pageData.length) {
        h += '<tr><td colspan="' + columns.length + '" class="text-center py-4 text-muted fst-italic">Tidak ada data ditemukan</td></tr>'
      } else {
        pageData.forEach(function (row) {
          h += '<tr>'
          columns.forEach(function (col) {
            h += '<td>'
            if (col.render) {
              h += col.render(row[col.field], row)
            } else {
              var v = row[col.field]
              h += v != null ? esc(String(v)) : ''
            }
            h += '</td>'
          })
          h += '</tr>'
        })
      }

      h += '</tbody></table></div>'
      return h
    }

    function buildGroupedTable(allData) {
      var h = '<div class="table-responsive"><table class="table table-hover table-bordered table-striped">'
      h += '<thead class="table-dark"><tr>'
      columns.forEach(function (col) {
        var sortable = col.sortable !== false
        var cls = sortable ? 'ps-sortable' : ''
        var field = col.field
        if (sortFieldClass(field)) cls += ' ' + sortFieldClass(field)
        h += '<th class="' + cls + '" data-field="' + field + '" data-sortable="' + sortable + '">'
        h += col.label
        h += '</th>'
      })
      h += '</tr></thead><tbody>'

      if (!allData.length) {
        h += '<tr><td colspan="' + columns.length + '" class="text-center py-4 text-muted fst-italic">Tidak ada data ditemukan</td></tr>'
      } else {
        var groups = {}
        allData.forEach(function (row) {
          var key = row[groupBy[0]] || ''
          if (!groups[key]) groups[key] = []
          groups[key].push(row)
        })

        var order = groupConfig[groupBy[0]] && groupConfig[groupBy[0]].order
        var keys = order ? order.filter(function (k) { return groups[k] }) : Object.keys(groups).sort()

        keys.forEach(function (key) {
          var rows = groups[key]
          var isToday = todayName && key === todayName
          var dayExpanded = !!isToday

          h += '<tr class="ps-group-header' + (isToday ? ' table-primary' : '') + '" data-group="' + esc(key) + '">'
          h += '<td colspan="' + columns.length + '">'
          h += '<span class="ps-group-toggle me-1"><i class="bi ' + (dayExpanded ? 'bi-chevron-down' : 'bi-chevron-right') + '"></i></span> '
          h += '<strong>' + esc(key) + '</strong>'
          h += ' <span class="text-muted small">(' + rows.length + ')</span>'
          h += '</td></tr>'

          if (groupBy.length > 1) {
            var subGroups = {}
            rows.forEach(function (row) {
              var subKey = row[groupBy[1]] || ''
              if (!subGroups[subKey]) subGroups[subKey] = []
              subGroups[subKey].push(row)
            })

            var subOrder = groupConfig[groupBy[1]] && groupConfig[groupBy[1]].order
            var subKeys = subOrder ? subOrder.filter(function (k) { return subGroups[k] }) : Object.keys(subGroups).sort()

            subKeys.forEach(function (subKey) {
              var subRows = subGroups[subKey]
              var multiRow = subRows.length > 1
              var courseExpanded = dayExpanded && multiRow

              h += '<tr class="ps-subgroup-header' + (dayExpanded ? '' : ' d-none') + '" data-subgroup-day="' + esc(key) + '" data-subgroup="' + esc(subKey) + '"' + (multiRow ? '' : ' style="cursor:default"') + '>'
              h += '<td colspan="' + columns.length + '">'
              if (multiRow) {
                h += '<span class="ps-group-toggle ms-3 me-1"><i class="bi ' + (courseExpanded ? 'bi-chevron-down' : 'bi-chevron-right') + '"></i></span> '
              } else {
                h += '<span class="ms-3 me-1"><i class="bi bi-dot"></i></span> '
              }
              h += esc(subKey)
              if (!dayExpanded) {
                h += ' <span class="text-muted small">(' + subRows.length + ')</span>'
              }
              h += '</td></tr>'

              subRows.forEach(function (row) {
                var visible = dayExpanded && (multiRow ? courseExpanded : true)
                h += '<tr class="ps-group-data' + (visible ? '' : ' d-none') + '" data-group-parent="' + esc(key) + '" data-subgroup-parent="' + esc(subKey) + '">'
                columns.forEach(function (col) {
                  h += '<td>'
                  if (col.render) {
                    h += col.render(row[col.field], row)
                  } else {
                    var v = row[col.field]
                    h += v != null ? esc(String(v)) : ''
                  }
                  h += '</td>'
                })
                h += '</tr>'
              })
            })
          }
        })
      }

      h += '</tbody></table></div>'
      return h
    }

    function buildPagination(total, pages) {
      if (pages <= 1) return ''

      var h = '<nav><ul class="pagination pagination-sm justify-content-center">'

      if (state.page > 1) {
        h += '<li class="page-item"><a class="page-link ps-page" href="#" data-page="' + (state.page - 1) + '">&laquo;</a></li>'
      }

      var startPage = Math.max(1, state.page - 2)
      var endPage = Math.min(pages, state.page + 2)

      if (startPage > 1) {
        h += '<li class="page-item"><a class="page-link ps-page" href="#" data-page="1">1</a></li>'
        if (startPage > 2) h += '<li class="page-item disabled"><span class="page-link">...</span></li>'
      }

      for (var i = startPage; i <= endPage; i++) {
        h += '<li class="page-item' + (i === state.page ? ' active' : '') + '">'
        h += '<a class="page-link ps-page' + (i === state.page ? ' disabled" tabindex="-1' : '') + '" href="#" data-page="' + i + '">' + i + '</a></li>'
      }

      if (endPage < pages) {
        if (endPage < pages - 1) h += '<li class="page-item disabled"><span class="page-link">...</span></li>'
        h += '<li class="page-item"><a class="page-link ps-page" href="#" data-page="' + pages + '">' + pages + '</a></li>'
      }

      if (state.page < pages) {
        h += '<li class="page-item"><a class="page-link ps-page" href="#" data-page="' + (state.page + 1) + '">&raquo;</a></li>'
      }

      h += '</ul></nav>'
      return h
    }

    function sortFieldClass(field) {
      if (state.sortField !== field) return ''
      return state.sortDir === SORT_ASC ? 'ps-sort-asc' : 'ps-sort-desc'
    }

    function bindEvents() {
      container.querySelectorAll('.ps-filter').forEach(function (sel) {
        sel.addEventListener('change', function (e) {
          state.filters[e.target.getAttribute('data-field')] = e.target.value
          state.page = 1
          render()
        })
      })

      container.querySelectorAll('.ps-sortable').forEach(function (th) {
        th.addEventListener('click', function (e) {
          var field = th.getAttribute('data-field')
          if (state.sortField === field) {
            state.sortDir = state.sortDir === SORT_ASC ? SORT_DESC : SORT_ASC
          } else {
            state.sortField = field
            state.sortDir = SORT_ASC
          }
          render()
        })
      })

      container.querySelectorAll('.ps-page').forEach(function (a) {
        a.addEventListener('click', function (e) {
          e.preventDefault()
          var p = parseInt(e.currentTarget.getAttribute('data-page'), 10)
          if (p && p !== state.page) {
            state.page = p
            render()
          }
        })
      })

      container.querySelectorAll('.ps-group-header').forEach(function (tr) {
        tr.addEventListener('click', function (e) {
          var key = tr.getAttribute('data-group')
          if (!key) return
          var dataRows = container.querySelectorAll('[data-group-parent="' + key + '"]')
          var subHeaders = container.querySelectorAll('[data-subgroup-day="' + key + '"]')
          var icon = tr.querySelector('.ps-group-toggle i')
          var allHidden = true
          dataRows.forEach(function (el) {
            el.classList.toggle('d-none')
            if (!el.classList.contains('d-none')) allHidden = false
          })
          subHeaders.forEach(function (el) {
            el.classList.toggle('d-none')
          })
          if (icon) icon.className = 'bi ' + (allHidden ? 'bi-chevron-right' : 'bi-chevron-down')
        })
      })

      container.querySelectorAll('.ps-subgroup-header').forEach(function (tr) {
        var subKey = tr.getAttribute('data-subgroup')
        if (!subKey) return
        var dataRows = container.querySelectorAll('[data-subgroup-parent="' + subKey + '"]')
        if (dataRows.length <= 1) return
        tr.addEventListener('click', function (e) {
          e.stopPropagation()
          var icon = tr.querySelector('.ps-group-toggle i')
          var allHidden = true
          dataRows.forEach(function (el) {
            el.classList.toggle('d-none')
            if (!el.classList.contains('d-none')) allHidden = false
          })
          if (icon) icon.className = 'bi ' + (allHidden ? 'bi-chevron-right' : 'bi-chevron-down')
        })
      })

      var resetBtn = container.querySelector('.ps-reset')
      if (resetBtn) {
        resetBtn.addEventListener('click', function (e) {
          state.searchTerm = ''
          state.filters = {}
          state.sortField = null
          state.sortDir = SORT_ASC
          state.page = 1
          var input = document.getElementById('ps-search')
          if (input) { input.value = ''; input.focus() }
          render()
        })
      }
    }

    render()
  }

  function esc(str) {
    var d = document.createElement('div')
    d.appendChild(document.createTextNode(str))
    return d.innerHTML
  }

  window.initPublicTable = initPublicTable
})()
