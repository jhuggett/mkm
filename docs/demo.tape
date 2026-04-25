# demo.tape — VHS script for the mkm README hero GIF.
#
# Install VHS first (one of):
#   brew install vhs
#   go install github.com/charmbracelet/vhs@latest
#
# Then from the repo root:
#   make demo
# or
#   vhs docs/demo.tape
#
# Records `mkm` against this repo's own Makefile, which ships with rich
# fixtures (db-*, docker-*, ci-*, deploy-app, …) so the demo needs no
# extra setup beyond a fresh `make build` of the binary.

Output docs/demo.gif

Set Shell        zsh
Set FontSize     14
Set Width        1200
Set Height       720
Set Theme        "TokyoNight"
Set TypingSpeed  60ms

# Drop into mkm, hiding the launch command so the GIF starts on the
# picker (no shell prompt, no "./mkm" typing).
Hide
Type "./mkm"
Enter
Sleep 1500ms
Show

# Fuzzy filter narrows the list as you type.
Type "docker"
Sleep 2s
Backspace@40ms 6
Sleep 500ms

# Cursor onto a target with a @param annotation.
Type "deploy-app"
Sleep 1s
Enter
Sleep 1500ms

# Type-ahead in an enum field jumps straight to "prod".
Type "p"
Sleep 1s

# Move to the VERSION text field and type a tag.
Down
Sleep 300ms
Type "v1.2.3"
Sleep 1s

# Toggle DRY_RUN with space — watch the run pill on the cmdline update
# in lock-step with the form values.
Down
Sleep 300ms
Space
Sleep 2s

# Bail out before actually running (the demo is about the picker, not
# about executing the recipe). Two escapes: form → list → quit.
Escape
Sleep 500ms
Escape
Sleep 1s
