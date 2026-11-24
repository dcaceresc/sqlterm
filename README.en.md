# SQLTerm

[Español (predeterminado)](README.md) · English

SQLTerm is a terminal SQL client written in Go for **SQL Server, MySQL, and PostgreSQL**. I built it for my own workflow: I work mainly on Linux and need the same tool on Windows to find objects, review and edit routines, run queries, and explore results without leaving the terminal.

It is not meant to be a universal client or to replace every database administration tool. It focuses on the features I use and can maintain.

## Main features

- Connection profiles organized by folder, favorites, and environment.
- Catalog of tables, views, procedures, and functions.
- Real Neovim editor embedded in the SQL pane, isolated from your personal configuration, with SQL tabs and files.
- Parameterized calls and editing for procedures and functions, depending on the database engine.
- IntelliSense based on the dialect, catalog, and simple SQL references.
- Cancelable execution with multiple result sets and incremental fetching.
- Safe copying, cell inspection, and CSV or JSON exports.
- Themes, configurable shortcuts, and optional local history.

## Platforms

Linux and Windows are the primary platforms. Go builds the project for both, and automation also produces macOS binaries as an additional build target. This does not mean every feature has been tested live on every system: the terminal, clipboard, credential store, session dependencies, network, and available servers also affect runtime behavior.

## Installation and usage

Requires **Neovim 0.11 or later** (`nvim` on `PATH`). Building from source requires **Go 1.24 or later**. SQLTerm keeps its connections and results panes; each SQL tab runs an isolated Neovim session.

Follow the [recommended installation](INSTRUCTIONS.en.md#recommended-installation) to install once from the repository and then run `sqlterm` from any directory. The guide also covers configuration, shortcuts, automation, testing, and current limitations.
