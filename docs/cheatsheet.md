# vimgram cheatsheet

## Modes

| Key    | Action                  |
| ------ | ----------------------- |
| `a`/`i` | Enter Insert mode      |
| `v`    | Enter Visual mode       |
| `:`    | Enter Command mode      |
| `esc`  | Return to Normal mode   |

## Chat list — Normal mode

| Key     | Action              |
| ------- | ------------------- |
| `j`/`k` | Move cursor down/up |
| `H`/`L` | Top/bottom of viewport |
| `g`     | First chat          |
| `G`     | Last chat           |
| `enter` | Open chat           |

## Chat — Normal mode

### Navigation

| Key           | Action                        |
| ------------- | ----------------------------- |
| `j`/`k`       | Move cursor down/up           |
| `H`/`L`       | Top/bottom of viewport        |
| `ctrl+u`      | Half page up                  |
| `ctrl+d`      | Half page down                |
| `g`           | Oldest message (top)          |
| `G`           | Newest message (bottom)       |
| `m{a-z}`      | Set mark                      |
| `'{a-z}`      | Jump to mark                  |

### Messages

| Key        | Action                               |
| ---------- | ------------------------------------ |
| `r`        | Reply to message under cursor        |
| `e`        | Edit message under cursor            |
| `f`        | Forward message under cursor         |
| `dd`/`dm`  | Delete message (for me)              |
| `da`       | Delete message (for everyone)        |
| `yy`       | Yank message text to register        |
| `p`        | Paste from register into input       |

### Compose

| Key     | Action                                        |
| ------- | --------------------------------------------- |
| `enter` | Send message / submit edit                    |
| `esc`   | Cancel edit / cancel reply / back to Normal   |

## Chat — Visual mode

Enter with `v`, extend selection with `j`/`k` (also `H`/`L`, `g`/`G`, `ctrl+u`/`ctrl+d`).

| Key       | Action                                       |
| --------- | -------------------------------------------- |
| `y`       | Yank all selected messages to register       |
| `r`       | Reply to the last selected message           |
| `f`       | Forward all selected messages                |
| `dd`/`dm` | Delete selected messages (for me)            |
| `da`      | Delete selected messages (for everyone)      |
| `esc`     | Exit Visual mode                             |

## Windows

| Key / Command  | Action                                        |
| -------------- | --------------------------------------------- |
| `ctrl+w h`     | Focus left window                             |
| `ctrl+w l`     | Focus right window                            |
| `ctrl+w w`     | Cycle focus                                   |
| `:vs [arg]`    | Vertical split (arg: buffer id or `chats`)    |
| `:close`       | Close focused window                          |

## Buffers

| Command      | Action                                           |
| ------------ | ------------------------------------------------ |
| `:ls`        | List buffers                                     |
| `:b N`       | Switch to buffer N                               |
| `:b#`        | Switch to alternate buffer                       |
| `:bn`        | Next buffer                                      |
| `:bp`        | Previous buffer                                  |
| `:bd [N]`    | Delete buffer N (current if omitted)             |
| `:chats`     | Switch to chat list                              |

## Settings & other commands

| Command                 | Action                             |
| ----------------------- | ---------------------------------- |
| `:set showarchive`      | Show archived chats in the list    |
| `:set noshowarchive`    | Hide archived chats (default)      |
| `:help`                 | Open this help in a buffer         |
| `:q`                    | Close window (quit if last)        |
| `:q!` / `:wq` / `:qa`  | Force quit                         |
