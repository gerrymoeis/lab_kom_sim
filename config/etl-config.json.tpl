{
  "source_db": "{{E2E_ROOT}}/staging/inventaris_lab.db",
  "source_uploads": "{{E2E_ROOT}}/staging/uploads",
  "source_upload_dir": "lab-kom-mi",
  "global_db": "{{E2E_ROOT}}/out/global.db",
  "uploads_dest": "{{E2E_ROOT}}/out/uploads",
  "main_account_suffix": "123",
  "labs": [
    {
      "id": "lab-mi",
      "url": "lab-mi",
      "db": "{{E2E_ROOT}}/out/lab_mi_1.db",
      "title": "Laboratorium MI-1",
      "mode": "source",
      "cols": [8, 8, 8, 8, 8],
      "has_gap": false,
      "gap_pos": 0,
      "row_gaps": [[], [], [], [], []]
    },
    {
      "id": "lab-vokasi-1",
      "url": "lab-vokasi-1",
      "db": "{{E2E_ROOT}}/out/lab_vokasi_1.db",
      "title": "Laboratorium Vokasi-1",
      "mode": "seed",
      "cols": [11, 9, 11, 11],
      "has_gap": true,
      "gap_pos": 5,
      "row_gaps": [[5], [5], [5], [5]]
    }
  ]
}