This project uses SQLC and Goose in order to manage database schemas and queries

# Guidelines
- Make sure that the tables are always normalized
- One column per line

# Creating Migrations
To make a migration

1. Create a migration using `goose`. Always use `sql` as the language. 
    ```
    goose create new_migration sql
    ```

    Never make the sql file directly. Always generate it via `goose`

2. Add necessary schemas to the migration file. 

    ```sql
    -- +goose Up
    CREATE TABLE users (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        username text NOT NULL
    );
    -- +goose Down
    DROP TABLE users;
    ```
    
    Always add a way to rollback migration changes
    
    Migration changes should be atomic. Migratations should not affect other migrations. Only changes stated in it's own sql file
    
    Keep migrations as small as possible. This is so that we can incrementally rollback changes to the database to see what went wrong

3. Apply the migration using `goose up`

# Creating Queries
To make a function that queries a database

1. Make a file in `/internal/database/queries`. 

    Always respect the boundaries of each sql file. All user related queries should only be found in `users.sql`

2. Create the query. Example:

    ```sql
    -- name: ListUsers :many
    SELECT 
        name, 
        profilePicture,
        password 
    FROM users 
    ORDER BY username;
    ```

    Always list out the variables you want to return. Never use `*` to make it clear to the reviewer

    Prefer to use `@variable_name` or `sqlc.arg(variable_name)` over `$1`. This makes it clearer to the reader what the variable is 
    ```sql
    -- BAD
    SELECT name FROM users WHERE id = $1;
    -- GOOD
    SELECT name FROM users WHERE id = sqlc.arg(userId);
    SELECT name FROM users WHERE id = @userId;              -- Alternative
    ```

    For deletions, prefer to return the deleted resource instead of quietly deleting them.

    ```sql
    DELETE FROM users WHERE id = sqlc.arg(userId) RETURNING;
    ```

3. Generate the Go code using `sqlc generate`

# Editing Queries
- Always edit the .sql files and let sqlc generate the .go files. Do not directly modify the .go files
