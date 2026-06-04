# Vimgram

A vim-style terminal client for Telegram. Navigate chats, read history, and send
messages without ever leaving your keyboard — modal editing, `:` commands, and all.

## Why?

Telegram's desktop and web clients are mouse-driven and heavy. If you live in the
terminal and think in `hjkl`, switching to a GUI just to reply to a message breaks
your flow.

Vimgram brings the modal editing model — and the **buffer/window** data model —
you already know to Telegram:

- **Normal mode** to move around, **Insert mode** to type, **Command mode** (`:`)
- **Visual mode** to select multiple messages at once — yank, reply, delete, or forward them all in one go
- **Buffers**: each open chat is a buffer — `:b`, `:bnext`, `:bprev`, `:bdelete`, `:ls`
- **Windows**: split the screen vertically and view two chats side by side —
  `:vsplit`, `<C-w>` to move between windows

No mouse, no clutter, no context switch.

## Features

- **Vim buffer model** — every loaded chat is a buffer; reopen instantly, list
  with `:ls`, jump with `:b N` / `:bnext` / `:bprev` / `:b#`, drop with `:bdelete`
- **Vertical splits** — `:vsplit` / `:vs [buffer]` to view chats side by side,
  `<C-w>h` / `<C-w>l` / `<C-w>w` to move focus
- **Modal controls** — `NORMAL` · `INSERT` · `COMMAND` · `VISUAL`
- **Visual mode** — select a range of messages with `v` + `j`/`k`, then yank, reply, forward, or delete the whole selection
- **Vim marks** — `ma` to bookmark a message, `'a` to jump back to it
- **Message cursor** — vim-style caret in chat view; `j`/`k` move the cursor,
  `H`/`L` jump to the top/bottom of the viewport
- **Reply** — press `r` on any message to reply; banner shows above the input
- **Edit** — press `e` on your message to edit it in-place
- **Forward** — press `f` to forward one message or a visual selection to any chat
- **Delete** — `dd`/`dm` delete for yourself, `da` delete for everyone
- **Yank / paste** — `yy` yanks a message text into an internal register, `p` pastes
  it into the compose input
- **Archive filter** — archived chats are hidden by default;
  `:set showarchive` / `:set noshowarchive` to toggle
- **Full chat list** — all your dialogs, sorted by last message, fully paginated
- **Message history** — `[HH:MM] Sender` headers, reply chains, forward hints,
  word-wrap, infinite scroll-back
- **Presence** — `(online)` / `(offline)` / `(typing...)` next to a DM's name
- **Real-time updates** — incoming messages land instantly, dialogs re-sort live
- **Persistent session** — log in once; session cached in `session.json`
- **Built-in help** — `:help` opens a scrollable command reference

## Installation

### Option 1 — download a binary

Grab the build for your platform from the
[Releases](https://github.com/Rafaelkocharli/vimgram/releases) page:

| Platform              | File                        |
| --------------------- | --------------------------- |
| macOS (Apple Silicon) | `vimgram-darwin-arm64`      |
| Linux (x86-64)        | `vimgram-linux-amd64`       |
| Linux (ARM64)         | `vimgram-linux-arm64`       |
| Windows (x86-64)      | `vimgram-windows-amd64.exe` |

Make it executable and run it:

```bash
chmod +x vimgram-*
./vimgram-*
```

### Option 2 — build from source

Requires Go 1.21+.

```bash
git clone https://github.com/Rafaelkocharli/vimgram.git
cd vimgram
make build
./vimgram
```

> **Note:** the build links `gotd/td`, a large auto-generated package, and can use
> ~2 GB of RAM. The `Makefile` passes `GOGC=20 GOMEMLIMIT=2GiB -p=1` to keep
> your machine from freezing.

### Optional: use your own Telegram app credentials

Vimgram ships with working API credentials. To use your own, get them at
[my.telegram.org/apps](https://my.telegram.org/apps) and place a `.env` file next to
the binary:

```
TG_APP_ID=12345
TG_APP_HASH=your_hash_here
```

## Usage

On first launch you'll be asked for your phone number, the login code Telegram sends
you, and your 2FA password (if enabled). After that the session is cached.

For a full key reference see [docs/cheatsheet.md](docs/cheatsheet.md), or type
`:help` inside the app.

## License

MIT
