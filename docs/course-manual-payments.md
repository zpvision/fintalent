# Course applications and external payment confirmation

The author receives payment outside FinTalent and explicitly confirms it in
Academy → Analytics → Students. This is a manual payment declaration, not a
YooKassa verification, bank statement, invoice generator, fee collection or payout.
Migration 092 is additive and is embedded in `prepareAcademyDatabase`, after
the course/referral, enrollment and notification-outbox schemas.

## Rates

All percentages use integer basis points: 2000 = 20%, 1200 = 12%, 975 = 9.75%.
`course_commission_defaults` holds defaults for future courses. An AFTER INSERT
trigger copies them into `course_commission_terms` for each course. Existing
courses get their own terms once during migration, with ON CONFLICT preserving
any existing customization. Changing defaults never changes existing courses.

Authorized database administration can change a particular course's two rates
or set `course_interests.commission_override_bps` for one unpaid application.
There is no author-facing rate editing API: an author cannot choose their own
platform fee by changing a request. Values are constrained to 0–10000 inclusive.

Examples for a database administrator (review IDs first; these are not deployment
commands and are never run automatically):

```sql
-- New courses only:
UPDATE course_commission_defaults
SET standard_bps=2000, partner_bps=1200 WHERE id=true;
-- A specific course, unpaid applications only (paid snapshots do not change):
UPDATE course_commission_terms
SET standard_bps=1800, partner_bps=1000 WHERE course_id=4;
-- An individual unpaid application; NULL restores the course's rates:
UPDATE course_interests SET commission_override_bps=900
WHERE course_id=4 AND user_id=123 AND paid_at IS NULL;
```

At confirmation the backend reads the account's permanent attribution for the
same course, including a historically deleted link, and selects that course's
standard/partner rate, then the individual override if present. It freezes
`paid_commission_bps`, `paid_referral_link_id`, `paid_at`, `paid_by`,
`paid_enrollment_id`, and `invitation_outbox_id`. A trigger prevents ordinary
updates to an already confirmed payment snapshot. No percentage is taken from
the client. Changing course terms or later acquiring a source never recalculates
a confirmed payment. The UI explicitly distinguishes preview and fixed rates.
No historical order, result, grade, or enrollment progress is recalculated.

## Atomic confirmation

`POST /api/academy/courses/{course}/analytics/interests/{interest}/paid`
accepts `{ "confirm": true, "version_id": <published release ID> }`.
Only the course owner can confirm through the author workspace. Curators with analytics access can neither read all-course leads
nor mark them paid. Existing CSRF, feature gate and course moderation checks
remain in force. The existing administrative analytics namespace remains read-only.

The transaction locks the course and application, checks the student and
published release, reuses an active enrollment in that version (including a
cohort) or calls the same `ensureIndependentEnrollment` service as ordinary
manual enrollment. New access lasts `courses.access_months` months. Suspended,
revoked or expired access is not bypassed: the author must restore it through
the existing CRM access workflow. No program version is changed in place.
Group-only programs require assigning the learner to the appropriate cohort
first; the confirmation will reuse that enrollment rather than creating an
empty independent room.

Payment confirmation, enrollment/event creation and invitation queue admission
commit together. Queue errors roll everything back. Repeated/concurrent
confirmation returns the existing result without another enrollment or email.
An `external_payment_confirmed` learning event records the actor and snapshot.
The UI asks for confirmation and a published version before submitting. There
is no ordinary uncheck/refund operation: reversing a declaration requires a
separate audited support workflow rather than silently revoking access.

## Invitations

`enqueueAcademyPaidInvitation` adapts to the existing notification outbox and
existing event-email template; it does not introduce a new SMTP worker. The
unique key is `academy-paid-interest:{interest_id}`. The email links directly to
the protected enrollment room and to the student's course list. Forwarding a
link does not grant access; the recipient must log into the correct account.

The existing worker retries failed SMTP attempts. Queue status is shown next to
the payment: pending/sending/sent/failed. Sent means SMTP submission, not verified
delivery. As with the pre-existing outbox, SMTP acceptance followed by a worker
crash can cause at-least-once delivery; normal duplicate confirmations do not
enqueue duplicates. Real recipients are not contacted by automated tests.

## Analytics and profile privacy

The overview shows all-time unpaid authenticated applications and navigates to
the unpaid application filter with period/search/source reset. The audience
panel starts collapsed. Acquisition and applications use two desktop columns,
stack on tablet, and compact cards on phones. Acquisition counts are all time;
application detail uses period, source, search and `interest_payment=paid|unpaid`.
Learning cohort/version/progress filters remain specific to enrolled students.

Profile links use the user's actual public, published resume ID. A user without
a public profile is shown without a fabricated URL or exposure of private
resume data. Guest applications remain excluded.

## Tests and rollout

Unit tests cover rate selection, payment validation and email escaping/links.
The browser fixture script covers collapsed filters, desktop columns, profiles,
commissions, confirmation failures/success, the disabled paid checkbox, unpaid
navigation and 360/900/1440 layouts. Those browser checks use isolated API mocks.

`TestCourseManualPaymentsIsolatedPostgres` requires explicit
`COURSE_REFERRALS_TEST_DSN` for a cloud database whose name contains `test`;
the known production host and localhost are refused. It uses the existing
isolated-schema fixture, never starts SMTP, and verifies concurrent confirmation,
outbox deduplication, rollback on queue error, migration repeatability, custom
course rates and immutable snapshots. Without this environment it is skipped.

Deploy the migration/backend and rebuilt frontend together using the usual
backup/release process. Existing global SMTP/outbox configuration must work.
Ensure a published Academy release exists before confirming payment. Existing
programs, user files and course payments are not automatically changed. Previous
application versions can ignore these additive tables/columns on rollback;
never drop the payment snapshots or automatically reverse confirmations.
