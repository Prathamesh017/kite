# Records assets/demo.gif. Run from the repo root: vhs assets/demo.tape
# Needs a working .env, because the demo makes real model calls.

Output assets/demo.gif

Require go

Set Shell "bash"
Set FontSize 18
Set Width 1800
Set Height 1100
Set Padding 10
Set TypingSpeed 60ms

# Kite only creates new files, so notes.md must not exist before the task run.
Hide
Type "rm -f notes.md && go build -o /tmp/kite ./cmd && clear && /tmp/kite"
Enter
Sleep 4s
Show

Sleep 1s
Type "Hi, how are you?"
Sleep 500ms
Enter
Sleep 6s

Type "what does cmd/main.go do?"
Sleep 500ms
Enter
Sleep 12s

Type "first read cmd/main.go, then read internal/tui/loop.go, then write a short summary of both to notes.md"
Sleep 500ms
Enter
# Waits for the summary after the last task instead of a fixed time. A run times out at 1 minute.
Wait+Screen@90s /task \d completed[\s\S]*> kite/

Sleep 4s

Hide
Ctrl+C
Type "rm -f notes.md /tmp/kite"
Enter
