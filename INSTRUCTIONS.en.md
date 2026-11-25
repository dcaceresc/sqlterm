# SQLTerm instructions

[Español (predeterminado)](INSTRUCTIONS.md) · English

This guide explains how to install, configure, and use SQLTerm. The client is built around a personal workflow, primarily targets Linux and Windows, and supports SQL Server, MySQL, and PostgreSQL.

## Contents

- [Recommended installation](#recommended-installation)
- [Local automation](#local-automation)
- [First workflow](#first-workflow)
- [Editor and SQL files](#editor-and-sql-files)
- [Connections, profiles, and databases](#connections-profiles-and-databases)
- [Catalog, tables, and routines](#catalog-tables-and-routines)
- [Execution and confirmations](#execution-and-confirmations)
- [Results, copying, and export](#results-copying-and-export)
- [IntelliSense](#intellisense)
- [Interface, themes, and folders](#interface-themes-and-folders)
- [Shortcuts](#shortcuts)
- [Configuration and credentials](#configuration-and-credentials)
- [Error logging and history](#error-logging-and-history)
- [Scripts and engine-specific behavior](#scripts-and-engine-specific-behavior)
- [Testing](#testing)
- [Known limitations](#known-limitations)
- [Repository structure](#repository-structure)

## Recommended installation

Install SQLTerm once from the repository root, then run it from any directory. Go 1.24 or later is only needed to build or install from source. The installed binary does not require Go, but it does require **Neovim 0.11 or later**, available as `nvim` on `PATH`. Run `nvim --version` to check before starting SQLTerm.

```sh
go install ./cmd/sqlterm
sqlterm --version
```

`go install` writes the executable to `GOBIN` when set. Otherwise, it uses the `bin` subdirectory of the first `GOPATH` entry. Check the actual values in your environment with:

```sh
go env GOBIN
go env GOPATH
```

With the default user-level destination, installation does not require `sudo` or administrator permissions. If you set a different `GOBIN`, you need write access to that directory. Installation may replace an earlier copy of `sqlterm` at the same destination. The command does not add that directory to `PATH` or persist it automatically.

### Linux

The executable is named `sqlterm`. If the installation directory is not yet on `PATH`, this example locates `GOBIN` or the first `GOPATH` entry and adds the install directory for the current session only, without replacing your configuration or the rest of `PATH`:

```sh
gobin=$(go env GOBIN)
if [ -n "$gobin" ]; then
  install_dir=$gobin
else
  gopath=$(go env GOPATH)
  install_dir=${gopath%%:*}/bin
fi
case ":$PATH:" in
  *":$install_dir:"*) ;;
  *) export PATH="$install_dir:$PATH" ;;
esac
sqlterm --version
```

To keep it in future terminals, add a line with the actual directory, for example `export PATH="/actual/path/bin:$PATH"`, to your user shell startup file. Do not replace that file or modify the global system configuration.

In X11 graphical sessions, clipboard copying may require `xclip` or `xsel`. Storing credentials may require Secret Service and a session that exposes it.

### Windows

The installed executable is named `sqlterm.exe`, but you run it as `sqlterm`. Find the directory with `go env GOBIN` or, if it is empty, use `bin` under the first entry shown by `go env GOPATH`. Add that directory to your user `Path` in the Windows environment variable settings and reopen the terminal. You do not need to change the system `Path` or use administrator privileges.

SQLTerm uses the available Windows services for the clipboard and credential storage. Availability also depends on the session in which it runs.

### Use an existing binary without Go

You can also place a SQLTerm binary that matches your system and architecture exactly in a directory on `PATH`. You do not need Go to run it. The repository can produce those binaries, but this guide does not assume a public download is available.

To update a source installation, update your local copy of the repository however you prefer, then run `go install ./cmd/sqlterm` again from its root. SQLTerm does not download changes or update the repository automatically.

### Development alternatives

To run once from the repository root:

```sh
go run ./cmd/sqlterm
```

To build a local development binary:

```sh
go build ./cmd/sqlterm
```

These alternatives are useful during development, but not needed to start an installed copy. SQLTerm needs an interactive terminal; 120×35 or larger is recommended. Dialogs adapt to the available space and do not require an editor extension.

Go can build for different systems and architectures, but that alone does not guarantee all features work at runtime. Linux and Windows are the primary targets; macOS remains an additional build target. Practical compatibility also depends on the terminal, clipboard, credential store, session dependencies, drivers, network, and available database servers.

## Local automation

The repository's portable automation runs with Go and does not require PowerShell or Bash:

```sh
go run ./scripts build
go run ./scripts check
go run ./scripts check -integration
```

`build` generates binaries for Windows, Linux, and macOS, in that order, for both `amd64` and `arm64`, with CGO disabled. Outputs go to `dist/sqlterm-<system>-<architecture>`; only Windows binaries use the `.exe` suffix.

`check` runs `go test ./...` followed by `go vet ./...`. The `-integration` option adds tests for `internal/db`; use it only with disposable profiles and databases, never with real or production data.

These scripts set local Go caches under `.cache/build`, `.cache/go`, and `.cache/mod` only for their child processes. You can still run individual Go build and check commands manually.

## First workflow

1. Press **F2**, select a folder, and use **a** or **Add profile**.
2. Enter the database engine, server, database, and user. **Folder** is the profile's logical folder.
3. Enter the password in the masked field. **Test connection** checks the settings without saving the profile or password. During the test it changes to **Cancel test**, and the temporary connection closes when the test ends.
4. Save and select the profile. If you change the engine, server, effective port, or user of a profile with a stored credential, reenter the password before testing or saving.
5. Press **F6** to find a procedure, function, table, or view by matching text in its name, schema, or type. Separate terms with spaces (for example, `dbo procedure`). The picker also supports filtering by type.
6. **Enter** opens the object's definition. For routines, **F9** opens the parameter form and lets you supply a value, `NULL`, or the default when the catalog or definition allows it.
7. Run the routine from the form. `[` and `]` switch between result sets, including output parameters and the return value when the engine provides them.
8. To edit a definition, press **F7**, enter INSERT mode with `i`, make your edits, and return to NORMAL with **Esc**. **Space c A** shows the diff and exact SQL before applying it.

**Space c t** generates a procedure template. Review it and press **Space c A** to apply it. A successful application refreshes the catalog. If you save the file before applying it, the comparison still uses the definition read from the server as its baseline.

In the top bar, click a tab name to select it or its **×** to close it. The connection context stays on the right of the same row. When space is limited, the bar prioritizes the active tab and shortens text or hides other tabs without closing them.

For ad hoc queries, **Ctrl+N** or **Space f n** creates another tab with an independent connection. **F5** runs the selection or the entire document. Each tab runs only one statement at a time, while other tabs can work in parallel.

## Editor and SQL files

### Modes and selection

The SQL pane uses a **real Neovim process per tab**, started with `--embed --clean -i NONE -n`. It does not load or modify your personal LazyVim configuration or persist swap files, ShaDa, or undo history to disk. Neovim handles modes, movements, registers, and undo; SQLTerm handles connections, results, file actions, and execution.

Main controls:

- `i a I A o`: insert.
- **Esc**: return to NORMAL.
- `v`: start characterwise VISUAL selection.
- `V` (uppercase): select the entire logical line, including its line break if present; `j` and `k` extend the selection by lines.
- `gg`: go to the start of the file; `G`: go to the end. `ggVG` selects the whole file.
- `hjkl`, `w b`, `0 $`: move the cursor.
- `yy` and `dd`: yank or cut a line.
- `y` and `d`: yank or cut the selection and return to NORMAL.
- `x`: delete; `p`: paste; `u`: undo.
- **Ctrl+R** in NORMAL: redo.
- `/`: search with wraparound.

Selection follows native Neovim behavior: `v` includes the current character and `V` selects whole lines. `y` uses Neovim registers and returns to NORMAL; Ctrl+C copies to the system clipboard without changing the mode or selection. Use Ctrl+S and Ctrl+Shift+S to save through SQLTerm: the Neovim buffer is temporary and has no path of its own.

In Windows Terminal, the updated backend uses bracketed paste. When the terminal intercepts Ctrl+V, the paste arrives as literal text rather than Vim commands and does not enter INSERT from NORMAL. When Ctrl+V reaches Neovim as a regular keystroke, it retains its native visual block selection behavior. SQL execution does not support block selections: it displays an error rather than running the entire document.

SQLTerm's global shortcuts (F1–F10, Ctrl+S, Ctrl+Shift+S, Ctrl+O, Ctrl+C, Ctrl+N, Alt+Z) take precedence. With the editor in NORMAL mode, Space opens the hierarchical menu. Press the group and action keys, or Esc to close it. In INSERT, Space still types a space. F11 and Ctrl+P no longer open actions; Ctrl+P keeps its native and IntelliSense navigation behavior. Neovim interprets other editor commands. The session is isolated from your configuration, but the embedded editor is not a sandbox.

If `nvim` is missing, startup fails with an error rather than silently falling back to the emulated editor. If the child process exits, SQLTerm keeps the last confirmed text and blocks saving or running stale data. Text-loading errors do not change the file path or saved state.

**Alt+Z** toggles visual line wrapping without changing the contents. The bottom bar shows the editor mode and, after a query starts, the execution time. Notices appear beside the editor mode without replacing the shortcut hints. The command palette can show or hide line numbers. These preferences are saved and apply to all tabs.

Highlighting distinguishes reserved words, strings, and numbers. Identifier delimiters are not highlighted as strings. Rendering respects the selection and line wrapping without changing SQL or undo history.

The embedded editor does not open the Ex command line with `:`. In INSERT mode, `:` is typed normally; searching with `/` or `?` still works. Use **F1** for help, **Space b d** to close the tab, and **Space q q** to quit.

### Selection and execution

**F5** runs the selected text, or the entire document if there is no selection. Commenting and copying do not run SQL.

Neovim toggles SQL line comments with `--`: in NORMAL mode, `gcc` acts on the current line and `gc` followed by a motion acts on a range (for example, `gcj` includes the next line). In VISUAL mode, select text and press `gc` to toggle comments on the selected lines. Press `u` in NORMAL to undo.

To toggle comments across the entire document, select all lines with `ggVG` and press `gc`.

Commented lines are dimmed, except for markers inside strings or dollar-quoted bodies. The color is visual only.

### Opening, saving, and confirming changes

When you open a SQL file, Windows CRLF line breaks appear as normal line breaks without `^M`. Saving preserves CRLF if the file used it consistently; files with mixed LF and CRLF are saved with LF.

**Ctrl+O** or **Space f e** opens the built-in browser. **Enter** opens a folder or file, `[..]` goes up one level, Tab and Shift+Tab move between fields, and **Esc** cancels. All files are shown to support scripts without a `.sql` extension. You can open files up to 8 MiB; if opening fails, the current document is preserved.

**Ctrl+Shift+S** opens **Save SQL as / Guardar SQL como** if the terminal can distinguish the shortcut. Some terminals may report it as **Ctrl+S** instead, in which case SQLTerm saves normally. **Ctrl+S** saves directly when the file already has a path and opens the save dialog otherwise.

The dialog asks before overwriting another file. Each successful save remembers the folder for the session: the next Save As starts there and keeps the tab's file name, or suggests `query.sql` for a new tab. The folder is not retained between sessions. If it no longer exists, SQLTerm uses the current file's folder or the working directory. The dialog does not create folders.

Closing a tab or replacing SQL asks for confirmation when there are unsaved changes. When you exit normally with **Space q q**, SQLTerm preserves the SQL tabs and which one was active without saving `.sql` files or restoring connections. Drafts remain marked as unsaved. The `<config>.workspace.json` cache sits next to the configuration file and contains SQL text, which may include sensitive data. The cache is not updated if the process terminates abruptly. Template actions also ask before replacing modified SQL.

### Copying SQL

With the editor focused, select text and press **Ctrl+C** to copy it literally to the system clipboard, preserving Unicode and line breaks. This adds no headers, TSV formatting, or spreadsheet protection. It does not modify the document, selection, history, or saved state, or run any SQL.

With no selection, SQLTerm shows a notice and does not overwrite the clipboard. A NUL character is represented as `\0` only in the copied text and triggers a notice. Errors appear in the status bar. Copying runs outside the UI loop, and another SQL copy cannot start while one is active.

Clipboard access requires a backend compatible with your system and graphical or terminal session. X11-based sessions may require `xclip` or `xsel`.

## Connections, profiles, and databases

### Profiles and connection testing

Profiles contain the engine, server, port, database, user, environment, logical folder, and encryption settings. The form shows the engine's default port. When you change engines, it updates the port if the field was empty or still held the previous default; it retains a custom port. A port of `0` uses 1433, 3306, or 5432, depending on the engine.

**Test connection** does not save changes. You can cancel the test, and it always closes its temporary connection. When editing the name, folder, or database, leaving the password empty keeps the stored credential. Changing the effective endpoint requires reentering the password so it cannot be reused with a different destination.

### Optional database

A profile can leave **Database** empty. On connection, the browser shows the databases available to that user. **Enter** selects a database and loads its objects; **F4** or **Space d b** lets you change it later.

The selection belongs to the tab and does not change the profile. Each switch opens a new connection, retains the SQL, and clears the previous database's results and metadata. If the new connection fails, the previous session remains active. Transaction state is not carried between connections.

You cannot run SQL, apply definitions, or invoke routines until you select a database. SQL Server uses `master` to list databases, PostgreSQL needs an initial connection to `postgres`, and MySQL can connect without a database. The user needs access to the initial database and permission to list available databases. Otherwise, configure an explicit database.

### Folders and favorites

**F2** organizes profiles into logical folders, initially DEV, QA, and PROD. They support subfolders such as `DEV/Ventas`; they are not filesystem directories. Profiles without a folder stay at the root.

With focus on the tree:

| Key | Action |
| --- | --- |
| Enter | Connect or expand/collapse a folder (no ←/→ or Space shortcuts) |
| a | Add a profile to the selected folder |
| n | Create a folder or subfolder |
| e | Edit the selected profile or folder |
| r | Open the same folder editor |
| d | Delete a folder without connections and its empty subfolders |
| / | Search by name, folder, engine, server, or database |
| Tab / Shift+Tab | Move between search, tree, and buttons |

Use **Name**, **Parent group**, and **Color** to create or edit a folder. Choose **(Root)** for a top-level folder. Renaming or moving a folder keeps its descendants and open connections. **Default** keeps the theme accent in the tree and the orange-red workspace indicator for PROD unless a folder has an explicit color. In the workspace, the nearest configured folder color wins. Color does not affect execution confirmations: the production policy still uses the top-level folder name, so moving a connection into or out of PROD can change whether it prompts.

To move a profile, change **Folder**; clear it to move the profile back to the root. Profiles can have the same name in different folders. Moving or renaming preserves credential references, and folder placement never changes the actual database of a session.

Press `f` or use **★ Favorite** to toggle a profile's favorite status. Favorites appear in a section at the top, stay in their original folders, and persist between sessions.

## Catalog, tables, and routines

The catalog groups objects by schema and type. Search matches name, schema, and type and is retained per tab. The command palette can switch to a flat list, insert a name with engine-specific quoting, generate a `SELECT` for a table or view, or invoke a routine. A generated `SELECT` does not run automatically.

### Tables

**Space c T** asks for a name and prepares a `CREATE TABLE` without running it. It includes an autogenerated `id` primary key and a `name` column, adapted to the engine. It uses `dbo` in SQL Server, `public` in PostgreSQL, and the selected database in MySQL. It requires a connection and, for MySQL, an active database.

Click a table or press **Enter** to inspect a `CREATE TABLE` definition. MySQL uses `SHOW CREATE TABLE`; SQL Server and PostgreSQL reconstruct columns, constraints, and indexes from visible metadata.

These definitions are not backups or complete migrations: they omit permissions, triggers, and some physical options. PostgreSQL does not reconstruct partitioned, inherited, or foreign tables. SQL Server does not reconstruct special options such as system-versioned temporal tables, columnstore indexes, or partitioning. **Apply definition** does not modify tables; do not run the `CREATE TABLE` against an existing table.

Generated `SELECT` statements do not add a fixed limit. They use incremental fetching and respect the engine's schema and quoting rules.

### Procedures and functions

**F9** invokes the selected object. Values are sent through the driver:

- SQL Server declares variables with the length, precision, and scale from the catalog and returns output parameters and `RETURN_STATUS` as an extra result set.
- PostgreSQL uses explicit types to resolve overloads and distinguishes `CALL` from function calls.
- MySQL uses session variables for `OUT` and `INOUT` and queries them after `CALL` finishes.

**Space c A** lets you review the diff and exact SQL before applying a routine. SQL Server uses `ALTER` for opened objects. PostgreSQL uses `CREATE OR REPLACE`; changing input types can create another overload and leave the original in place. MySQL requires `DROP` and `CREATE`; the review warns that this is not atomic and that permissions may need to be restored. If `CREATE` fails, the editor keeps the code so you can fix it.

Refresh the catalog with **Space d r** to find newly created objects and clear the column cache when the schema changes.

## Execution and confirmations

### Commands and status

**F5** runs SQL, **F9** invokes a routine, and **F10** cancels the active operation. The destination stays in the top bar. The Results heading indicates when an error or cancellation leaves partial rows, or when the results belong to earlier SQL.

The command palette offers **Execution confirmations**:

- Disabled (the default).
- Confirm likely writes in PROD.
- Confirm every F5 execution.

Write detection is heuristic. It does not guarantee safety, read-only execution, or detection of every side effect. Always review the SQL and the account's permissions.

### Incremental fetching

Rows are fetched continuously without scrolling to request more. While the query is running, the grid stays on the first rows and shows the received-row count and elapsed time. When it finishes, all received rows can be browsed. **F10** cancels the read and keeps received rows; copying, sorting, and exporting require the execution to end.

## Results, copying, and export

### Navigation and selection

In the results pane:

- Click a data cell to focus it. Press **Enter** to mark the focused cell, column, or row; clicking a column header or row number marks it too. **Ctrl+C** or **y** copies without headers or row numbers.
- `/` searches.
- `s` and `S` sort the column locally in ascending or descending order.
- `m` shows messages.
- `[` and `]` switch result sets.
- `e` opens export.

Sorting is lexicographic, puts `NULL` first in ascending order, and only affects rows already fetched. Export retains the original server order.

In SQL Server, `uniqueidentifier` values appear as readable GUIDs, including in routine results. Other binary columns are not treated as GUIDs just because they are 16 bytes long. An invalid binary length produces a read error rather than unreadable text.

### Safe and literal copying

| Key | Action |
| --- | --- |
| Ctrl+C / y | Copy the selected cell, row, or column |
| Y | Copy the selection or focused cell, row, or column, with headers |
| r / R | Copy the row without / with headers |
| c | Copy the cell |
| v | Start or clear a row selection |
| + / - | Widen or narrow the active column |

Press **Space** in the focused grid to open Results directly, or **Space r** from the NORMAL editor. In either menu, **Y** copies with headers, **h** copies only headers, and **v** toggles a full-row range. Use the grid arrow keys to extend the range.

Safe mode is the default. It prefixes `'` to text whose first significant character is `=`, `+`, `-`, or `@`, in both headers and values, to reduce formula execution when pasting into a spreadsheet. This also protects values such as `-42`. Use **Literal** only when you need the exact text and trust the data.

The transformation affects only the output. `NULL` is represented as `\N` and NUL as `\0`. Clipboard failures are reported without closing the application.

### Export and storage

Use **Shift+arrow keys** or drag to mark a rectangle. Press **`e`** to export: the default scope is **Selection** when a selection is active, or **Entire result** otherwise. When a selection is active, you can switch to the entire result before saving. Export follows the displayed order. Choose **Spreadsheet-safe CSV**, **Literal CSV**, **JSON**, or **Excel (.xlsx)**, regardless of the file extension. It then opens the same file browser used by Open and Save so you can choose a folder and file name. It remembers the last successful export folder even after a restart, without changing the folder used to save SQL. Export requires a new file, never overwrites an existing one, and removes partial output if canceled.

Both CSV modes exported from Results start with a UTF-8 byte order mark (BOM) so Excel can display accents and other special characters when opening the file directly. Safe CSV applies spreadsheet protection to headers and cells. Literal CSV preserves text and warns about untrusted data. Both use `\N` for SQL NULL and escape text beginning with `\` to distinguish NULL, an empty string, and literal `\N`.

JSON uses `{"columns":[...],"rows":[[...]]}`, preserves duplicate names, represents SQL NULL as `null`, and encodes other values as text.

Each result set is stored in a private temporary directory with an offset index. The grid loads pages of up to 128 rows. Changing queries, closing a tab, or exiting normally removes these files. A forced exit can leave `sqlterm-results-*` directories in the system temporary directory. Sorting keeps only indexes in memory, and export traverses the result set without loading it all at once.

## IntelliSense

On startup, SQLTerm shows its own welcome message in the status bar and leaves the SQL editor empty rather than showing the Neovim introduction. It currently does not restore tabs or files from a previous run. Open your files with **Ctrl+O**.

In INSERT mode, suggestions appear after typing two characters or a period. **Ctrl+Space** opens them manually. The arrow keys or **Ctrl+N/P** move through the list. **Tab** or **Enter** accepts a suggestion, and **Esc** closes the list and returns to NORMAL.

Suggestions include dialect keywords, schemas, tables, views, and routines with their signatures. Without a connection, only SQL keywords are available. Objects and columns require a catalog. SQLTerm recognizes simple references and aliases in `FROM`, `JOIN`, `UPDATE`, and `INTO`, and fetches columns as needed.

In SQL Server, typing `EXEC procedimiento ` with a trailing space suggests named parameters. They are offered again after a comma. Typing `@` or a prefix filters the list, and accepting a suggestion inserts `@parametro = `. The list does not open automatically at an empty value after `=`. The procedure must be in the active catalog; refresh it with **Space d r**.

Suggestions do not run SQL. IntelliSense is not a language server or a full semantic analyzer: it does not infer columns from CTEs or complex subqueries, or provide diagnostics equivalent to those of a specialized tool.

## Interface, themes, and folders

The top bar shows the environment, connection, database, and status. PROD stands out in text and color. **Space w** changes the layout, adjusts proportions, hides the catalog, or maximizes panes. The layout is saved. Small terminals show one pane at a time, and **F6**, **F7**, and **F8** switch between objects, the editor, and results.

**F1** opens searchable, sectioned contextual help. Close the connection, theme, and database pickers by clicking outside them, pressing **Esc** or **Ctrl+C**, or switching panes. Canceling the theme picker reverts the preview.

### Themes

**F3** or **Space u C** opens the theme picker. **Space c t** is still reserved for the procedure template. The arrow keys preview themes and **Enter** saves. The picker shows a sample query in the highlighted theme's colors without running it or changing the editor. Closing without saving restores the previous theme.

**F2**, **F3**, and **F4** open with the list focused. Press **/** to filter; **Enter** picks the highlighted item. **Esc** from search clears the filter and returns to the list; from the list it closes the picker.

Themes include Tokyo Night, Rosé Pine, Nord, Gruvbox, Light, Classic, Dracula, Solarized Light, Catppuccin Mocha, and Everforest. A theme changes panes, the editor, results, forms, and dialogs without disconnecting or replacing SQL. The selection persists. The JSON field `theme` accepts `tokyo-night`, `rose-pine`, `nord`, `gruvbox`, `light`, `classic`, `dracula`, `solarized-light`, `catppuccin-mocha`, or `everforest`; an unknown value falls back to Tokyo Night.

The cursor shape tries to switch from a block in NORMAL and VISUAL to a bar in INSERT, depending on terminal support. The mode indicator and border remain visible. No icons or special font are required.

### Language

**Space u i** opens **Language / Idioma**. Technical messages from the database engine, SQL, identifiers, and values are not translated.

## Shortcuts

| Key | Action |
| --- | --- |
| F1 / F2 | Help / connections |
| F3 / Space u C | Themes with preview |
| F4 | Select or change the database |
| F5 / F9 / F10 | Run / invoke / cancel |
| F6 / F7 / F8 | Objects / editor / results |
| Space (editor in NORMAL) | Hierarchical action menu |
| Ctrl+Space | Open IntelliSense |
| Alt+Z | Toggle line wrapping |
| `gcc` / `gc` with a motion / `gc` in VISUAL | Toggle comments on the current line / range / selection |
| Ctrl+O / Ctrl+S / Ctrl+N | Open / save / new tab |
| Ctrl+Shift+S | Save as |
| Ctrl+C | Copy selected text or the highlighted result |
| Space b d | Close the current tab (confirm if there are unsaved changes) |
| Space q q | Quit and preserve drafts |
| `Shift+H` / `Shift+L` in NORMAL | Previous / next tab (wraps at either end) |

### Space menu

With focus on the editor in NORMAL, press **Space**, the group key, and the action key. Each prefix displays the next available keys in a pane on the right rather than opening a modal dialog or taking focus from the editor. The pane does not change the SQL or cursor and closes when you run an action or cancel. Keys are case-sensitive. **Esc** cancels and **Backspace** returns to the previous group. In INSERT, Space still types spaces. F11 and Ctrl+P do not open menus; Ctrl+P still navigates suggestions.

| Group | Actions after Space |
| --- | --- |
| `f` Files | `e` explore files · `n` new session · `s` save · `S` save as |
| `b` Tabs | `d` close tab |
| `c` Code | `x` run · `A` review/apply definition · `p` invoke routine · `t` routine template · `T` table template · `i` insert object name · `q` generate SELECT |
| `d` Connections | `c` profiles · `b` switch database · `d` disconnect · `r` refresh catalog · `e` confirmations |
| `s` Explore | `o` find objects · `d` view definition · `g` grouped/flat catalog · `h` history |
| `r` Results | `y` copy · `Y` copy with headers · `h` copy only headers · `v` toggle row range · `e` export · `m` technical messages |
| `u` Preferences | `w` line wrap · `l` line numbers · `C` theme · `i` language |
| `w` Layout | `b` balanced · `e` editing · `d` browsing · `m` maximize editor · `r` maximize results · `c` show/hide catalog · `>` / `<` adjust catalog width by ±4 columns · `+` / `-` give the editor/results ±10 percentage points of space |
| General | `?` help · `x` cancel operation · `q q` quit |

**Space w b** restores the balanced layout after maximizing a pane.

`keys` supports global aliases using tcell special key names, for example `"run": "Ctrl-E"` or `"apply": "F12"`. Supported actions are `run`, `connect`, `invoke`, `cancel`, `help`, `save`, `open`, `new`, `apply`, `refresh`, `theme`, `database`, and `complete`. F4 is reserved for database selection. F11 and Ctrl-P cannot be used as action aliases. Aliases take precedence and the remaining default shortcuts stay active.

## Configuration and credentials

The default configuration is located at:

```text
os.UserConfigDir()/sqlterm/config.json
```

To use a different file with the recommended installation:

```sh
sqlterm --config examples/config.json
```

The relative path `examples/config.json` works when running the command from the local repository. From another directory, provide an absolute path or a valid relative path to the file you want to use. The example contains local profiles; engines are identified as `sqlserver`, `mysql`, and `postgres`.

### Encryption and certificates

The form contains **Encrypt connection** and **Trust server certificate**. Encryption is enabled by default; trusting the certificate is a separate decision. JSON stores these settings as `encrypt` and `trust_server_certificate`.

If you see `x509: certificate signed by unknown authority`, the server presents a certificate issued by an authority that the system does not recognize:

- If you have verified that it is the expected server, you can enable **Trust server certificate** while keeping **Encrypt connection** enabled. The connection remains encrypted, but the certificate is not validated.
- To validate the certificate without enabling that option, install the appropriate certificate authority in the system trust store.

SQLTerm never enables certificate trust automatically. Edit the profile, check the endpoint, and reconnect. Trusting a certificate without validation reduces protection against impersonation.

### Credential store

Passwords are saved through the system credential store. The JSON contains a `credential_id`, not the password. Deleting a profile also removes its credential. If the store is unavailable, the application reports the error and does not save the password in plain text.

Credential store availability and configuration depend on the system and session. Some environments require a compatible service such as Secret Service. Older profiles remain supported, and existing files can still use `password_env`.

Connection errors redact the known password, even when it appears encoded in a URL or DSN. Changing the engine, server, effective port, or user invalidates the old association and requires reentering the password.

## Error logging and history

### Error log

Full technical errors are appended to `sqlterm-errors.log` next to the selected configuration file, including when you use `--config`. The file is created only when an error occurs, and SQLTerm does not rotate it. Remove it manually when you no longer need it.

SQLTerm redacts known passwords and their encoded forms. It does not add SQL, results, configuration, or DSNs separately, but the text returned by a server may contain fragments of SQL or data. Review the file before sharing it and remove it if it contains sensitive information.

### Optional history

History is disabled by default. Enable it with **Space s h**. It is stored as `history.json` alongside profile data and retains at most 200 queries.

History can include sensitive literals entered in SQL. Values from the routine invocation form are excluded. Updates replace the file atomically and are coordinated among processes that share the same path. Disabling history preserves existing entries; **Clear history** removes them. Editor contents are not automatically saved to SQL files on exit.

## Scripts and engine-specific behavior

- SQL Server preserves the batch and recognizes `GO` and `GO n` (up to 1000) on standalone lines outside strings and comments.
- MySQL recognizes `DELIMITER` when creating routines. Applying a definition sends the full body without requiring the directive.
- PostgreSQL recognizes dollar-quoted bodies, nested comments, and `E''` strings.
- The statement splitter does not use a simple `strings.Split(";")`.

Transactions follow server rules. Use `BEGIN`, `COMMIT`, and `ROLLBACK` explicitly. Switching databases opens another connection and loses temporary state from the previous one. Writes are never retried automatically.

Form parameters are text or `NULL`, and the server handles conversion. SQL Server table-valued parameters and composite types that require special binding must be passed through SQL. PostgreSQL arrays are entered in textual form, and returned cursors require an explicit `FETCH`.

## Testing

Neovim tests use isolated processes and are skipped if `nvim` is not installed. Skipping them does not establish that the real integration works. With Neovim installed, `go test ./internal/nvim ./internal/editor ./internal/ui -count=1` covers the engine and its integration.

To check native transport in Windows Terminal, run `./scripts/verify-windows-terminal.ps1` from PowerShell and follow the prompts for pasting, keys, the mouse, and resizing. This is an optional interactive check. Automated parser tests do not replace that evidence or test actual console restoration.

For the usual checks:

```sh
go test ./...
go vet ./...
```

If your environment and dependencies support it, you can also use:

```sh
go test -race ./...
```

Integration tests are skipped unless disposable databases are configured. They create and remove procedures and tables with unique names: **do not use production servers or databases**.

Each engine is enabled through its own environment variables:

- `SQLTERM_TEST_POSTGRES_PROFILE` and `SQLTERM_TEST_POSTGRES_PASSWORD`.
- `SQLTERM_TEST_SQLSERVER_PROFILE` and `SQLTERM_TEST_SQLSERVER_PASSWORD`.
- `SQLTERM_TEST_MYSQL_PROFILE` and `SQLTERM_TEST_MYSQL_PASSWORD`.

Set `*_PROFILE` to a JSON object with the appropriate host, port, disposable database, user, and connection options. Set `*_PASSWORD` separately. Use your terminal's environment variable facilities and avoid storing credentials in the repository.

Repository automation runs Go tests in several environments and provides disposable services for all three engines. This verifies the build targets and automated workflow. It does not by itself establish compatibility with every terminal, credential store, native dependency, network, or existing server.

## Known limitations

- SQLTerm targets SQL Server, MySQL 8.0+, and PostgreSQL 14+.
- Automated integration uses specific versions. Other versions need real-world validation.
- Authentication uses a username and password.
- The connected user can see and work with only the objects their privileges allow. Encrypted definitions and definitions the user cannot access are shown as unavailable.
- The editor offers basic Vim modes, not a complete Neovim implementation.
- IntelliSense depends on the catalog and does not perform advanced semantic analysis or search inside all definitions.
- SQL compatibility depends on the dialect and server modes. The lexer does not adapt to `NO_BACKSLASH_ESCAPES` or `standard_conforming_strings=off`.
- PostgreSQL routine bodies must use dollar quoting or strings. Unquoted SQL `BEGIN ATOMIC` bodies must be applied as a complete definition with **Space c A**, not run as a document split into statements with F5.
- PostgreSQL materialized views have no dedicated editing workflow.
- Named SQL Server instances must be configured with their effective host and TCP port.
- Full compatibility with older SQL Server versions needs real-world validation, although signature generation does not depend on `STRING_AGG`.
- Grid data editing, designers, schema comparison, profiling, backup, and restore are outside the current scope.
- Clipboard, cursor, color, credential store, and other service availability depends on the environment.

## Repository structure

- `cmd/sqlterm`: entry point and flags.
- `internal/db`: adapters, metadata, scripts, routine calls, and execution events.
- `internal/editor`: Vim controls over a tview buffer.
- `internal/ui`: panes, forms, and session coordination.
- `internal/result`: paged storage and export.
- `internal/config` and `internal/history`: local persistence.
- `examples`: example configuration files.
- `scripts`: build and check automation.
