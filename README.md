# Local Demonlist

Local Demonlist ranks Geometry Dash Demon levels and tracks player standings for the Obninsk community. The application uses Go and Gin, stores data in PostgreSQL, and serves HTML, CSS, and JavaScript.

## Local setup

You need Go 1.25.2 or later, PostgreSQL, and the `migrate` CLI with PostgreSQL support to set up a new database.

1. Start PostgreSQL. For a new installation, create an empty database and a database user with permission to create objects in its schema.
2. If you don't already have a `.env` file, copy the template. Set `DATABASE_URL` in `.env` to your PostgreSQL connection string.

   ```sh
   test -e .env || cp .env.example .env
   ```

3. For a new, empty database, run the migrations from the project root. This command loads the settings from your local `.env` in a subshell.

   ```sh
   (
     set -a
     . ./.env
     migrate -path migrations -database "$DATABASE_URL" up
   )
   ```

   For an existing database, restoring from a backup, or loading optional demo levels, see the [PostgreSQL setup guide](docs/database-deployment.md).

4. Start the application from the project root so it can find `templates/` and `static/`.

   ```sh
   go run ./cmd
   ```

   Open the [demonlist](http://localhost:8080/demonlist) in your browser.

## Tests

```sh
go test ./...
```

## Source code and data

The repository includes the source code, active migrations, archived migrations for reference, and sample data for development. Player data is stored in PostgreSQL. Keep your `.env` file and database backups in local or private storage.

[Demonlist rules](docs/rules.md)
