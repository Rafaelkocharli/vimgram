# Vimgram

A vim-style terminal client for Telegram. Navigate chats, read history, and send
messages without ever leaving your keyboard — modal editing, `:` commands, and all.

## Why?

Telegram's desktop and web clients are mouse-driven and heavy. If you live in the
terminal and think in `hjkl`, switching to a GUI just to reply to a message breaks
your flow.

Vimgram brings the modal editing model you already know to Telegram:

- **Visual mode** to move around
- **Insert mode** to type
- **Command mode** (`:q`, `:wq`, `:qa`) to quit or go back

No mouse, no clutter, no context switch.

## Features

- **Modal vim controls** — `VISUAL` (navigate) · `INSERT` (type) · `COMMAND` (`:`)
- **Three screens** — sign-in → chat list → chat view
- **Full chat list** — all your dialogs, sorted by last message, fully paginated
- **Message history** — inline `[HH:MM] Sender: text` format with word-wrap and
  infinite scroll-back through older messages
- **Real-time updates** — incoming messages appear instantly in the open chat and
  bump dialogs to the top of the list
- **Send messages** — straight from the terminal
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

### Chat list

| Key                   | Action          |
| --------------------- | --------------- |
| `j` / `k` / `↑` / `↓` | Move cursor     |
| `g` / `G`             | Top / bottom    |
| `enter`               | Open chat       |
| `:q` `:wq` `:qa`      | Quit            |

### Chat view — VISUAL mode

| Key                                     | Action            |
| --------------------------------------- | ----------------- |
| `a` / `i`                               | Enter INSERT mode |
| `j` / `k`                               | Scroll one line   |
| `pgup` / `pgdn` / `ctrl+u` / `ctrl+d`   | Half-page scroll  |
| `g` / `G`                               | Oldest / newest   |
| `:q`                                    | Back to chat list |
| `:wq` / `:qa`                           | Quit              |

### Chat view — INSERT mode

| Key     | Action            |
| ------- | ----------------- |
| `enter` | Send message      |
| `esc`   | Back to VISUAL    |

### A note on quitting

There is no `Ctrl+C` escape hatch — this is a vim app. Use `:q` to step back,
`:qa` / `:wq` to quit entirely.

## License

MIT
