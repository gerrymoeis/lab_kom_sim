# Future Enhancements

## Authentication & Security
- **Rate limiting** on login endpoint to prevent brute-force attacks.
- **2FA / TOTP** support for admin accounts.
- **Session revocation** — allow admins to invalidate all active sessions for a user.
- **Audit trail retention policy** — configurable auto-purge of old activity logs.

## Inventory Features
- **Bulk import/export via CSV** for PCs, devices, software (beyond the current Excel export).
- **QR/bar code labels** — generate printable labels for each PC/device with QR codes linking to detail pages.
- **Asset lifecycle tracking** — procurement date, warranty expiry, EOL reminders.
- **Location hierarchy** — rooms → racks → positions, with visual floor plan.

## UI/UX
- **Dark mode** toggle with persistent preference.
- **Responsive/mobile-friendly** dashboard grid for small screens (currently optimized for desktop).
- **Inline editing** on table views (click-to-edit fields without page reload).
- **Sortable/filterable table columns** across all list pages.

## Notifications & Alerts
- **Email/Push notifications** for low-stock supplies, expiring warranties, or overdue device loans.
- **Scheduled maintenance reminders** based on logbook entries.
- **Dashboard alerts** for hardware failures or disk space warnings.

## Performance & Scalability
- **Connection pooling** tuning for PostgreSQL under concurrent access.
- **Caching layer** (e.g., in-memory or Redis) for frequently accessed data like PC list and dashboard stats.
- **Pagination** on all long list pages (PCs, devices, logbook, activity logs).
- **Lazy loading** for the 40-PC dashboard grid images.

## OCR & Logbook
- **Batch OCR upload** — process multiple logbook photos at once.
- **OCR confidence scoring** and manual correction workflow.
- **Export to PDF** with logbook entries formatted as official reports.

## API
- **REST API** for third-party integrations (e.g., SIS akademik) with API key auth.
- **Webhook support** for inventory change events.

## Deployment & Operations
- **Docker Compose** setup for easy self-hosting.
- **Automated database backup** schedules via the web UI.
- **Health check endpoint** (`/health`) for monitoring.
- **Prometheus metrics** for request volume, error rates, OCR latency.

## Code Quality
- **Unit test coverage** for individual services/repositories (beyond the existing integration test).
- **API documentation** via OpenAPI/Swagger.
