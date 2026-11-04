# Storage release provenance and deployment precautions

The S3 implementation was developed on `feature/academy-object-storage`, based on
`b0832972673845b6ef627b0a198f6cb831797345`. The production package identifier is
`s3-20261104-db171738299b`; it is a package identifier, not a Git commit.

During the Git audit on 2026-11-04, the locally retained release manifest was
compared against rebuilt artifacts. All eleven compared payload files matched:
the Linux backend, migrations 088–090, built React assets, and changed legacy
assets. The rebuilt backend SHA-256 also matched the executable retained in the
post-deployment backup. No production connection was made for this comparison.
This proves correspondence to those local artifacts, not the current live server.
Future builds after a commit can differ because Go embeds VCS revision metadata.

The subsequent private-file administrator verification mode is an intentional
addition to the original package. It was deployed as a separate migration utility,
without replacing or restarting the backend. Its source and tests are retained
under `cmd/fintalent-storage-migrate` and `internal/filemigration`. Verification
of anonymous denial and source/S3 SHA-256 does not prove successful access under
a real owner's account. See `fintalent-storage-migration.md` for the explicit flag.

## Recorded migration outcome

The operator reported 123 public files migrated with stable URLs, full SHA-256
checks, and retained local originals. A separate administrator run transferred
eight private certificates/test materials while preserving disk originals/BYTEA.
The reviewed total was 131 files / 80,512,346 bytes. Subsequent uploads were not
silently included in that reviewed set. Sixty-one unreferenced old images and
four missing/unsafe legacy references were excluded and retained as applicable.
These counts are historical observations, not hard-coded application rules.

The production reverse proxy was separately configured to proxy only
`/static/uploads/` to the existing backend resolver. Other static assets kept
their previous handling. Nginx configuration is not application source: review
the actual site configuration before deployment; do not overwrite other sites.

Backups were retained off-server and their checksums verified. PostgreSQL restore
was explicitly waived by the operator and remains unverified. A DB dump does not
back up the contents of the S3 bucket. Preserve manifests/journals outside Git.
Do not roll the whole storage driver back to local: new direct-S3 uploads can lack
local originals. Only perform an explicitly reviewed, per-file rollback.

## Checks at Git audit

- `go test ./...`, `go vet ./...`, native and Linux backend builds.
- `npm run build` and syntax checks for changed browser scripts.
- Isolated browser fixture checks at 360/900/1440 px, including video playback,
  Range-style seeking behavior in the fixture, URL renewal, retry and cancellation.
- Storage and migration unit tests, including anonymous-denial/private-mode tests.
- No production DB/S3 tests or file migrations were run during the Git audit.
- DB integration re-execution was refused because the available saved test DSN
  could not be safely distinguished from production/loopback. The opt-in DB tests
  remain in source for a separately verified test database.

## Before the next deployment

Review and merge the storage branch through the normal process. Build from the
approved commit rather than an unidentified worktree. Keep generated React output
out of Git and build it during release packaging. Verify private S3 configuration,
the permanent local fallback directory, migrations' embed/prepare chain, and the
reverse-proxy resolver route. Do not restore an old `.env` or binary inadvertently.
Retain fresh backups and journals; never automatically rerun the completed file
migration or delete originals. Rotate previously exposed credentials separately;
never include real credentials, cookies, private manifests or user files in Git.
