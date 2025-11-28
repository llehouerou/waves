# Fuzzy Search Design

## Overview

Add fzf-like fuzzy search with a generic, reusable search component. The search UI can be used for filesystem navigation, library search, playlist search, etc.

## Architecture

**New package:** `internal/search/` (generic, reusable)
- `search.go` - Generic Bubble Tea model for fuzzy search popup
- `item.go` - Item interface definition

**Interface:**
```go
type Item interface {
    FilterValue() string  // what fuzzy matches against
    DisplayText() string  // what's shown in results
}

type Model struct {
    items    []Item
    query    string
    cursor   int
    loading  bool
    // ...
}

func New() Model
func (m *Model) SetItems(items []Item)  // for async loading
func (m *Model) SetLoading(bool)        // show spinner
func (m Model) Selected() Item          // returns selected item
```

**Filesystem scanner:** `internal/navigator/scanner.go`
- Runs in goroutine, sends results via channel
- Converts paths to search.Item
- Managed by navigator/main, not by search package

**Integration:**
- Main model gets `searchMode bool` and `search search.Model` fields
- `/` key toggles search mode and starts filesystem scanner
- Search overlay renders on top of navigator
- On selection, navigator jumps to the item's location

**Navigator addition:**
- `NavigateTo(path string)` - navigates to directory containing path and selects item

## Dependencies

- `github.com/sahilm/fuzzy` - fuzzy matching with fzf-style scoring

## UI Design

```
╭─────────────────────────────────────╮
│ > query                             │
├─────────────────────────────────────┤
│   path/to/matching/file.mp3         │
│ > path/to/selected/item/            │
│   another/match.flac                │
╰─────────────────────────────────────╯
```

- Width: 60% of terminal
- Height: 50% of terminal (or fewer rows if less results)
- Centered overlay
- Input at top with `>` prompt
- Results sorted by fuzzy score
- Directories shown with trailing `/`
- Relative paths from search root

## Key Bindings

- Type → filter results
- Up/Down, Ctrl+P/Ctrl+N → move cursor
- Enter → select and close
- Escape → cancel and close
- Backspace → delete character

## Scanner

- Runs in goroutine, sends results via channel
- Uses `filepath.WalkDir` for recursive scanning
- Filters: directories + music files (.mp3, .flac)
- Skips hidden files/directories (starting with `.`)
- Partial results shown while scanning (with spinner)

## Performance

- All paths stored in memory (~10MB for 100k files)
- Fuzzy filter on each keystroke (<10ms for 100k items)
- Display limited to ~20 visible items (scrollable)

## Edge Cases

- Empty query → show all results (up to limit)
- No matches → show "No matches"
- Permission errors → skip silently
- Scanner running → show partial results + indicator
