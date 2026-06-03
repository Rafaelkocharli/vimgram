# vimgram — agent context

Terminal Telegram client written in Go with vim-style modal UX.

## Stack

- **UI**: Bubble Tea (Elm/MVU) + lipgloss
- **Telegram**: gotd/td (native MTProto, no bot API)
- **Go version**: 1.26.3

## Architecture

```
cmd/vimgram/         entry point
internal/
  app/               domain types only — no framework imports
  config/            env-based config (TG_APP_ID, TG_APP_HASH)
  storage/           session.json persistence
  telegram/          MTProto client, event emission
  ui/                all Bubble Tea code: model, update, view, render
```

### Key files

| File | Purpose |
|---|---|
| `internal/ui/model.go` | Model struct — all state lives here |
| `internal/ui/update.go` | All key handling and event processing |
| `internal/ui/view.go` | Rendering: window compositor, chat list, overlays |
| `internal/ui/chat_lines.go` | Flattened chat lines, viewport, cursor rendering |
| `internal/ui/message_render.go` | Per-message block rendering (header, body, forward/reply hints) |
| `internal/ui/buffer.go` | Buffer and window structs |
| `internal/ui/scroll.go` | Height calculations, list offset |
| `internal/ui/styles.go` | All lipgloss styles in one place |
| `internal/ui/help.go` | Static content for :help buffer |
| `internal/telegram/client.go` | Request queue, event emission, public API |
| `internal/telegram/history.go` | buildMessage, fetchHistory |
| `internal/telegram/dialogs.go` | fetchDialogs, buildDialogEntry |
| `internal/telegram/updates.go` | Incoming update dispatcher |
| `internal/app/command.go` | ParseCommand — all : commands defined here |

## Data model

### Buffers

Every open chat is a `buffer`. The chat list is buffer id=1 (always present).
`:help` is a `bufHelp` buffer with static `helpLines []string`.
Windows hold a `bufferID` reference — two windows can show the same buffer independently.

```
bufferKind: bufChatList | bufChat | bufHelp
```

### Windows

```go
type window struct {
    bufferID   int
    altBuffer  int    // for :b#
    lineOffset int    // chat scroll from the bottom (0 = newest)
    cursor     int    // chat-list row cursor
    listOffset int    // chat-list scroll
    msgCursor  int    // absolute index into chatLines (chat cursor)
    colCursor  int    // horizontal column within cursor line
    width      int    // cached from last render
}
```

### Chat lines

`chatLines(b, width)` flattens a buffer's messages into visual lines (memoized by `msgVersion`).
For `bufHelp` it returns `b.helpLines` directly.
The viewport is **bottom-anchored**: `lineOffset=0` shows the newest messages.

### Events (telegram → UI)

All events implement `telegram.Event`. The client emits them via a callback; the UI wraps them in `telegramEventMsg` and routes through `handleTelegramEvent`.

```
EventConnected
EventDialogsLoaded
EventMessagesLoaded
EventMessagesPrepended
EventMessageSent
EventMessageReceived
EventMessageEdited
EventMessageDeleted
EventUserStatus
EventUserTyping
EventError
```

## UI patterns

### Modal state

```
vimMode: ModeNormal | ModeEdit | ModeVisual | ModeCommand
```

Normal mode key chords use boolean flags on Model:
- `pendingCtrlW bool` — after `<C-w>`, waits for h/l/w
- `pendingDelete bool` — after `d`, waits for m/d/a
- `pendingYank bool` — after `y`, waits for second `y`

Confirmation prompts use bool flags:
- `discardPrompt bool` — "No write since last change. Discard draft? [y/N]"
- `deletePrompt bool` + `deleteRevoke bool`

### Overlays

Rendered in `View()` before the normal window compositor:
- `forwardActive bool` — full-screen chat picker for `f` (forward)
- `len(m.overlay) > 0` — :ls buffer list (static string slice)

### Reply / Edit / Forward state (per buffer)

```go
replyToID      int
replyToPreview string
editMsgID      int
editOrigText   string
```

Forward state is on Model (not buffer) since only one forward can be active at a time.

## Adding a new : command

1. Add `CmdFoo` constant to `internal/app/command.go`
2. Add the verb string to `ParseCommand`'s switch
3. Handle `app.CmdFoo` in `executeCommand` in `internal/ui/update.go`

## Adding a new Normal-mode key action

- Single key: add a `case "x":` in `updateChatNormal`
- Two-key chord: add a `pendingFoo bool` to Model, set it on the first key,
  handle the second key at the top of `updateChatNormal` (before the switch)

## Telegram client

`client.go` has a single goroutine processing `c.requests` channel.
Add new actions by:
1. Defining a `fooReq` struct
2. Adding a public method that sends to `c.requests`
3. Handling the req in the select loop
4. Emitting a result event

## Commit conventions

`type(scope): description` — no Co-Authored-By lines, no emoji.
Examples: `feat(chat): ...`, `fix(chat-list): ...`, `docs: ...`

## Build

```bash
make build         # current platform
# cross-compile:
GOOS=darwin  GOARCH=arm64 go build -o dist/vimgram-darwin-arm64  ./cmd/vimgram
GOOS=linux   GOARCH=amd64 go build -o dist/vimgram-linux-amd64   ./cmd/vimgram
GOOS=linux   GOARCH=arm64 go build -o dist/vimgram-linux-arm64   ./cmd/vimgram
GOOS=windows GOARCH=amd64 go build -o dist/vimgram-windows-amd64.exe ./cmd/vimgram
```

gotd/td is large — set `GOGC=20 GOMEMLIMIT=2GiB -p=1` to avoid OOM during build.
