# Plan: 5C — Settings Page

## Overview
Add a settings page that lets users view and edit search filters (stored in
config.yaml) and the base resume (stored in resume.md). Changes are written
directly to those files and the in-memory config/resume is reloaded.

## Design Decisions
- Server needs access to the config file path and resume path at runtime, so
  we add `configPath` and a `*config.Config` pointer (for live reload) to
  Server. Because `Server` is created in `main.go` after loading config, this
  is a small addition.
- Search filter edits overwrite the YAML search_filters block using
  gopkg.in/yaml.v3 marshaling and writing back to file.
- Resume edits write the entire body to resume.md.
- Flash messages are implemented via a query-param redirect (?saved=1).
- Keep it simple: direct file writes, no database.

## Steps

### Step 1: Tests (server_test.go additions)
- GET /settings returns 200 text/html
- POST /settings/resume writes resume content and redirects
- POST /settings/filters parses form, writes updated filters, redirects

### Step 2: Extend Server struct
- Add `configPath string` and `cfg *config.Config` and `resumePath string`
  fields to Server.
- Add `NewServerWithConfig(st JobStore, cfg *config.Config, configPath,
  resumePath string) *Server` constructor (the simple `NewServer` can remain
  for tests, using nil cfg / empty paths).
- Parse `templates/settings.html` into a third template set `settingsTmpl`.

### Step 3: Create internal/web/templates/settings.html
- Extends layout.html.
- Displays current search filters in a table (read-only view).
- Form for adding a filter (keywords, location, min_salary fields).
- Remove buttons (POST /settings/filters/remove?index=N via hx-post).
- Textarea for resume content (POST /settings/resume).
- Flash message on ?saved=1.

### Step 4: Add routes to server.go
- GET /settings → handleSettings (render page)
- POST /settings/resume → handleSaveResume
- POST /settings/filters → handleAddFilter
- POST /settings/filters/remove → handleRemoveFilter (query param: index)

## Files Created/Modified
- internal/web/templates/settings.html  (new)
- internal/web/server.go                (extend Server struct + 4 handlers)
- internal/web/server_test.go           (add test cases)
