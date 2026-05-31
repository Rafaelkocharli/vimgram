# vimgram cheatsheet

# Modes

a / i       enter Insert mode
esc         return to Normal mode
:           enter Command mode
v           enter Visual mode

# Chat List

j / k       move cursor down / up
g           first chat
G           last chat
enter       open chat

# Chat — Navigation

j / k       move cursor down / up
h / l       move cursor left / right
ctrl+u      half page up
ctrl+d      half page down
g           oldest message (top)
G           newest message (bottom)

# Chat — Messages

r           reply to message under cursor
e           edit message under cursor
dd / dm     delete message (for me)
da          delete message (for everyone)
yy          yank message text to register
p           paste from register into input, enter Insert mode

# Chat — Compose

enter       send message / submit edit
esc         cancel edit / cancel reply / back to Normal

# Windows

ctrl+w h    focus left window
ctrl+w l    focus right window
ctrl+w w    cycle focus

# Commands

:q          close window (quit if last window)
:q!         force quit
:wq / :qa   force quit
:vs [arg]   vertical split (arg: buffer id or "chats")
:close      close focused window

# Buffers

:ls         list buffers
:buffers    list buffers (alias)
:b N        switch to buffer N
:b#         switch to alternate buffer
:bn         next buffer
:bp         previous buffer
:bd [N]     delete buffer N (current if omitted)
:chats      switch to chat list (alias for :b1)
