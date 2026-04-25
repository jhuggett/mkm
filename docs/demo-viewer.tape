# demo-viewer.tape — vertical slice for the in-TUI Makefile viewer (^v).
#
# Showcases:
#   - ^v opens the source for the highlighted target
#   - light syntax styling on @param / .PHONY / recipe lines
#   - n / N to jump between target definitions
#   - g / G to top/bottom, ^d/^u to half-page
#   - position counter updating in the title bar
#
# Run with:  make demo-viewer

Output docs/demo-viewer.gif

Set Shell        zsh
Set FontSize     14
Set Width        1100
Set Height       640
Set Theme        "TokyoNight"
Set TypingSpeed  80ms

Hide
Type "./mkm"
Enter
Sleep 1500ms
Show

# Filter and view the source for the cursor target.
Type "deploy-app"
Sleep 600ms
Ctrl+v
Sleep 1500ms

# Walk forward through target definitions with n.
Type "nnnn"
Sleep 1500ms

# And back with N.
Type "NN"
Sleep 1500ms

# Half-page down, then jump to the bottom and back to the top.
Ctrl+d
Sleep 1000ms
Type "G"
Sleep 1200ms
Type "g"
Sleep 1000ms

# Back to list, then quit.
Escape
Sleep 500ms
Escape
Sleep 500ms
