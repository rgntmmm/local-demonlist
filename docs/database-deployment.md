# Local Demonlist PostgreSQL setup

GitHub hosts the source code and migrations. The Go application and PostgreSQL run on a VPS or cloud platform. GitHub Pages cannot host PostgreSQL.

## Active migrations

- `migrations/000001_initial_schema.up.sql` creates the current tables, indexes, constraints, sequences, functions, triggers, and leaderboard view.
- Apply `000002_level_list_revision.up.sql` before deploying the list cache update. It tracks changes to levels and position points in the same transaction as each write.
- The initial migration populates `position_points` with the scoring values required by the application.
- `migrations/000001_initial_schema.down.sql` drops the entire application schema and its data. Never run it as part of a routine upgrade on a live database.
- `seeds/development.sql` contains optional demo levels with synthetic game IDs. Use it for development after applying the migrations.
- `migrations_archive/pre_baseline_20261117/` contains the previous migration history for reference. Keep it outside the active migration path.

Number new migrations `000002`, `000003`, and so on. Leave published migrations unchanged. Use a single migration runner that tracks applied versions in the database.

## New database

Set up PostgreSQL, create the application database, and set `DATABASE_URL` in the server environment. With the golang-migrate CLI installed, run this command from the repository root.

```sh
migrate -path migrations -database "$DATABASE_URL" up
```

Then start the application. To load optional demo data into a test database, run the following command.

```sh
psql "$DATABASE_URL" -X -v ON_ERROR_STOP=1 --single-transaction -f seeds/development.sql
```

## Existing project database

Before consolidating the migrations on November 17, 2026, the project stored these files locally.

- `backups/pre_baseline_20261117.dump`, a full backup in PostgreSQL's custom dump format
- `backups/pre_baseline_20261117_schema.sql`, the original schema
- The previous SQL migration files in `migrations_archive/pre_baseline_20261117/`

The schema was verified by comparing normalized schema-only dumps, excluding the migration metadata table, test schema names, and comments. The migration was tested with an up/down/up cycle, and integration tests ran against separate schemas.

The existing database reported version 2 in `schema_migrations` with `dirty=false`, even though it contained later schema changes. After verification, the migration metadata was set to baseline version 1 with `dirty=false`. The initial migration was left unapplied on that database to preserve its existing schema and data.

For any other existing database, make a backup and compare its schema with the baseline before changing the migration version. Use `migrate ... force 1` only if the schema matches. This command changes the migration metadata only. If the database has an older schema, upgrade it using the archived migrations first. Populate the actual in-game level IDs before applying the old migration `000006`.

## Moving a database with existing data

Restore the full dump into an empty target database. Apply the baseline only when initializing a fresh database without a full restore.

```sh
pg_dump --format=custom --no-owner --no-acl --file=database.dump "$SOURCE_DATABASE_URL"
pg_restore --exit-on-error --single-transaction --no-owner --no-acl \
  --dbname="$TARGET_DATABASE_URL" database.dump
```

A fresh full dump includes the current migration metadata. The `pre_baseline_20261117.dump` backup predates the consolidation and still contains version 2. After restoring it, verify the schema again before setting the migration metadata to baseline version 1.

Keep `.env`, database dumps, session data, and real user data out of GitHub. Git ignores `backups/`. Set database credentials on the server or through your CI secret store.
