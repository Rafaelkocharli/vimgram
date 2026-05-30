# Vimgram

A vim-style terminal client for Telegram. Navigate chats, read history, and send
messages without ever leaving your keyboard — modal editing, `:` commands, and all.

## Why?

Telegram's desktop and web clients are mouse-driven and heavy. If you live in the
terminal and think in `hjkl`, switching to a GUI just to reply to a message breaks
your flow.

Vimgram brings the modal editing model — and the **buffer/window** data model —
you already know to Telegram:

- **Visual mode** to move around, **Insert mode** to type, **Command mode** (`:`)
- **Buffers**: each open chat (and the chat list) is a buffer you can switch
  between instantly — `:b`, `:bnext`, `:bprev`, `:bdelete`, `:ls`
- **Windows**: split the screen vertically and view two chats side by side —
  `:vsplit`, `<C-w>` to move between windows, `:close`

No mouse, no clutter, no context switch.

## Features

- **Vim buffer model** — every loaded chat is a buffer; reopen instantly, list
  with `:ls`, jump with `:b N` / `:bnext` / `:bprev` / `:b#`, drop with `:bdelete`
- **Vertical splits** — `:vsplit` / `:vs [buffer]` to view chats side by side,
  `<C-w>h` / `<C-w>l` / `<C-w>w` to move focus, `:close` to close a window
- **Modal controls** — `VISUAL` (navigate) · `INSERT` (type) · `COMMAND` (`:`)
- **Full chat list** — all your dialogs, sorted by last message, fully paginated
- **Message history** — inline `[HH:MM] Sender: text` format with word-wrap and
  infinite scroll-back, scrolled line-by-line
- **Presence** — `(online)` / `(offline)` / `(typing...)` next to a DM's name
- **Real-time updates** — incoming messages land in the right buffer instantly and
  bump dialogs to the top of the list
- **Send messages** — straight from the terminal; per-buffer drafts are preserved
- **Persistent session** — log in once; your session is cached in `session.json`
- **Zero-config** — bundled API credentials work out of the box

## Installation

### Option 1 — download a binary

Grab the build for your platform from the
[Releases](https://github.com/Rafaelkocharli/vimgram/releases) page:

| Platform            | File                          |
| ------------------- | ----------------------------- |
| macOS (Apple Silicon) | `vimgram-darwin-arm64`      |
| Linux (x86-64)      | `vimgram-linux-amd64`         |
| Linux (ARM64)       | `vimgram-linux-arm64`         |
| Windows (x86-64)    | `vimgram-windows-amd64.exe`   |

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
> ~2 GB of RAM. The `Makefile` already passes `GOGC=20 GOMEMLIMIT=2GiB -p=1` to keep
> your machine from freezing.

### Optional: use your own Telegram app

Vimgram ships with working API credentials. To use your own, get them at
[my.telegram.org/apps](https://my.telegram.org/apps) and drop a `.env` file next to
the binary:

```
TG_APP_ID=12345
TG_APP_HASH=your_hash_here
```

Both fields must be set for the override to take effect.

## Usage

On first launch you'll be asked for your phone number, the login code Telegram sends
you, and your 2FA password (if you have one). After that, your session is cached.

### Sign-in screen

| Key     | Action  |
| ------- | ------- |
| `enter` | Submit  |
| `esc`   | Quit    |

Vimgram opens on the **Chats** buffer. Move the cursor and press `enter` to open a
chat — it becomes its own buffer. Switch between loaded buffers instantly; split the
screen to see more than one at a time.

### Chat list (Chats buffer)

| Key                   | Action      |
| --------------------- | ----------- |
| `j` / `k` / `↑` / `↓` | Move cursor |
| `g` / `G`             | Top / bottom |
| `enter`               | Open the selected chat (as a buffer) |

### Chat view — VISUAL mode

| Key                                   | Action            |
| ------------------------------------- | ----------------- |
| `a` / `i`                             | Enter INSERT mode |
| `j` / `k`                             | Scroll one line   |
| `pgup` / `pgdn` / `ctrl+u` / `ctrl+d` | Half-page scroll  |
| `g` / `G`                             | Oldest / newest   |

### Chat view — INSERT mode

| Key     | Action         |
| ------- | -------------- |
| `enter` | Send message   |
| `esc`   | Back to VISUAL |

### Buffers (`:` commands)

| Command              | Action                                         |
| -------------------- | ---------------------------------------------- |
| `:ls` / `:buffers`   | List loaded buffers (`%` current, `#` alternate) |
| `:b N`               | Switch to buffer N                             |
| `:b#`                | Switch to the alternate (previous) buffer      |
| `:bnext` / `:bn`     | Next buffer                                    |
| `:bprev` / `:bp`     | Previous buffer                                |
| `:bdelete` / `:bd [N]` | Close a buffer (the Chats buffer can't be deleted; the Telegram chat is untouched) |

### Windows / splits (`:` commands)

| Command / Key            | Action                                       |
| ------------------------ | -------------------------------------------- |
| `:vsplit` / `:vs`        | Split vertically (new window shows the same buffer) |
| `:vs N` / `:vs Chats`    | Split and open buffer N / the chat list in the new window |
| `<C-w>h` / `<C-w>l`      | Focus the window to the left / right         |
| `<C-w>w`                 | Cycle focus between windows                  |
| `:close`                 | Close the focused window (buffers stay loaded) |

### Quitting

There is no `Ctrl+C` escape hatch — this is a vim app.

- `:q` — close the focused window; if it's the last window, quit the app
- `:qa` / `:wq` — quit the app from anywhere

To leave a chat and go back to the list without quitting, switch buffers: `:b 1`
(the Chats buffer) or `:bd` to drop the current chat buffer.

## License

MIT
