# Contributing to Vimgram

## Prerequisites

- Go 1.21+
- A Telegram account with API credentials from [my.telegram.org/apps](https://my.telegram.org/apps)

## Setup

```bash
git clone https://github.com/Rafaelkocharli/vimgram.git
cd vimgram
cp .env.example .env  # fill in TG_APP_ID and TG_APP_HASH
make build
./vimgram
```

> The build links `gotd/td`, a large auto-generated package. The `Makefile` passes
> `GOGC=20 GOMEMLIMIT=2GiB -p=1` to avoid OOM.

## Project layout

```
cmd/vimgram/        entry point
internal/
  app/              domain types — ParseCommand, command constants
  config/           env-based config
  storage/          session.json persistence
  telegram/         MTProto client, event emission
  ui/               all Bubble Tea code (model, update, view, render)
```

See [AGENTS.md](AGENTS.md) for a deeper architectural reference.

## Making changes

### Adding a `:` command

1. Add a `CmdFoo` constant in `internal/app/command.go`
2. Add the verb string to `ParseCommand`'s switch
3. Handle `app.CmdFoo` in `executeCommand` in `internal/ui/update.go`

### Adding a Normal-mode key

- Single key: add a `case "x":` in `updateChatNormal`
- Two-key chord: add a `pendingFoo bool` to `Model`, set it on the first key,
  handle the second key at the top of `updateChatNormal` (before the switch)

### Adding a Telegram action

1. Define a `fooReq` struct in `internal/telegram/`
2. Add a public method that sends to `c.requests`
3. Handle the req in the select loop in `client.go`
4. Emit a result event

## Commit style

```
type(scope): short description
```

Types: `feat`, `fix`, `docs`, `refactor`, `chore`
Scopes: `chat`, `chat-list`, `ui`, `telegram`, `config`, `storage`

Examples:
```
feat(chat): reply to message with r
fix(chat-list): sort dialogs after live update
docs: update README for release
```

No emoji. No period at the end.

## Pull requests

- Keep PRs focused — one feature or fix per PR
- Make sure `make build` succeeds before opening a PR
- Describe *why*, not just *what*, in the PR body
