# Course referral sources and expressions of interest

Implementation files:

- Startup/auth/security: `courses_module.go`, `main.go`, `request_security.go`.
- Course API: `internal/courses/handler.go`, `referrals.go`,
  `referral_tracking.go`, `referrals_test.go`, `referrals_postgres_test.go`.
- Academy: `internal/academy/analytics_domain.go`, `analytics_handler.go`,
  `analytics_repository.go`, `analytics_sources.go`, `analytics_sources_test.go`.
- Frontend: `frontend/src/pages/courses/CoursePricingStep.jsx`,
  `CourseReferralLinks.jsx`, `CourseView.jsx`, `shared.jsx`, and
  `frontend/src/pages/academy/AcademyAnalytics.jsx`, `AnalyticsSources.jsx`.
- Styles/schema/QA: `static/courses.css`,
  `migrations/091_course_referrals.sql`,
  `scripts/course-referrals-browser-check.cjs`.
- Documentation: this file and `docs/CODEX_CONTEXT.md`.

Course payments, orders, YooKassa, author settlements and refunds do not exist
in this repository yet. This feature records acquisition and **interest**, not
sales. No payment, enrollment, notification, or money movement is created by
clicking “Выбрать курс”. The intended 12% / 20% rates are not currently charged.
When course checkout is implemented, its server must resolve the permanent
attribution and freeze the rate and rounded amounts in the order; it must not
trust a client-supplied source or commission.

## Data and lifecycle

Migration `091_course_referrals.sql` is embedded in `prepareCoursesDatabase`.
It is additive and repeatable. Existing courses, enrollments, files and test
results are unchanged.

- `course_referral_links`: immutable name/code, lower-case ASCII code, unique
  `(course_id, code)` including soft-deleted rows. Administrative creation is
  marked explicitly rather than attributed to an invented user.
- `course_referral_guests`: hashed 256-bit random cookie token, consumed account
  binding. The raw token is only an HttpOnly SameSite=Lax browser cookie, Secure
  on HTTPS. Browser retention is at most 400 days; database attribution has no
  expiry. Deleting cookies or changing devices before login loses anonymous
  identity. Two simultaneous first requests without any cookie can be separate
  visitors; absolute anonymous uniqueness cannot be guaranteed.
- `course_referral_guest_claims`: first valid link per browser/course.
- `course_referral_attributions`: first valid link per account/course, no TTL.
  Composite foreign keys prevent cross-course sources. Existing account
  attribution always wins on login. Deleted links cannot recruit new accounts;
  a pending guest claim on a deleted link is not bound at login. Account
  attributions already bound before deletion survive unchanged.
- `course_referral_visitors`: compact per-link/browser-or-account visit counts.
  Visits less than two seconds apart are coalesced. Login merges the visitor
  counts with the account and consumes the guest identity. No IP, email,
  user-agent, cookie token or signed URL is stored in acquisition statistics.
- `course_interests`: one first choice per account/course or anonymous
  browser/course. Login merges duplicates. The source comes from the database,
  never from the interest request. A tariff is validated against the course;
  its numeric ID is historical metadata, not an order or price snapshot.
  A later first valid partner attribution can fill an interest's previously
  empty source; an already assigned partner source is never overwritten, and
  the date of first interest is retained.

Visit capture occurs through same-origin JavaScript POST after a published
landing is loaded. Server-side link previews do not create visits, and known
preview bot user agents are ignored. This is practical filtering, not bot-proof
fraud detection. The ref query parameter does not replace or erase UTM values.
Hidden/draft courses cannot receive visits or expressions of interest.

Login/registration bind sources after credential validation and before issuing
the login cookie. Binding failures are visible and do not silently discard the
source. After successful login the consumed guest cookie is cleared, so another
account on a shared browser cannot inherit it. Database transactions, row locks
and unique constraints handle concurrent requests; an existing source is never
overwritten by later links or ordinary visits. Guest and link lock ordering is
consistent with login binding. Soft deletion preserves names and statistics.

## API

- `GET/POST /api/courses/my/{id}/referrals`: owner list/create, even before
  landing approval. Creation accepts only `name` and `code`.
- `DELETE /api/courses/my/{id}/referrals/{link}`: owner soft-delete.
- Equivalent `/api/admin/courses/{id}/referrals[/{link}]` routes use the existing
  platform admin session; normal user identity is not impersonated.
- `POST /api/courses/referrals/visit`: `{slug,code}`, no personal-data response.
- `POST /api/courses/interest`: `{slug,tariff_id?}`, no user/source/commission
  input is trusted. Both public writes retain the existing Courses access gate,
  so beta access restrictions remain in force.
- Existing `.../academy/.../analytics/students?source=platform|{link_id}` adds
  sources, interest statistics and a separately paginated interest list. The
  source filter is applied on the server before student pagination, together
  with search and the existing learning filters. It also survives report filter
  serialization. Admin analytics reuses the same service.

Source summary counts are explicitly **all time**, independent of learning
filters. “Visitors” are pseudonymous browsers or accounts after merging, not
an absolute count of humans. Interest detail uses period, search and source;
cohort/version/progress filters only apply to the enrollment table, because a
lead need not have an enrollment. Whole-course acquisition statistics and leads
are owner/admin-only. Curators see source names for their permitted enrollments,
without course-wide acquisition totals or the lead list. Payments and revenue
are unavailable, not fake zero sales.

Existing CSRF middleware and same-origin checks protect writes. Separate IP
rate limits allow 60 referral visit and 30 interest requests per minute.

## Verification and rollout

Run `go test ./...`, `go vet ./...`, `go build ./...`, `npm run build`.
Unit tests cover code normalization, cookies, early CSRF/bot denials and source
filters. SQL tests require an explicitly supplied **separate cloud test DB**:

```text
COURSE_REFERRALS_TEST_DSN=<private DSN; database name must contain test>
go test ./internal/courses -run TestReferralIsolatedPostgres -count=1 -v
```

These tests never load `.env` or use `DATABASE_URL`, refuse the known production
host, and create/drop only a uniquely named test schema. They apply 091 twice
and exercise handler validation, guest binding, first-source wins, deletion,
cross-course integrity, interest deduplication and concurrent requests.

`node scripts/course-referrals-browser-check.cjs` checks the actual built React
app against loopback API fixtures in headless Edge (`EDGE_PATH` can override
the executable). It verifies link creation/copy/confirmation, referral query
navigation, desktop/mobile selection, no preview events, source filtering and
360/900/1440 layouts. This is frontend verification, not an end-to-end database
test. Screenshots are written to a temporary directory, outside Git.

For deployment, first verify 091 against a dedicated test database, back up the
working database, then deploy the backend and rebuilt frontend together through
the normal release process. No provider credentials are required. No data
backfill or historical commission recalculation is performed. Rollback may use
the previous application while retaining these additive tables; do not drop
attributions or reuse deleted codes.
