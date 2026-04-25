# demo-themes.tape — vertical slice for live theme preview (^s).
#
# Showcases:
#   - ^s opens settings
#   - cycling the theme repaints the entire UI live
#   - a-z type-ahead jumps to themes by prefix
#   - color swatch under the theme row visualises the palette
#   - esc reverts the preview, enter persists
#
# Run with:  make demo-themes

Output docs/demo-themes.gif

Set Shell        zsh
Set FontSize     14
Set Width        1000
Set Height       540
Set Theme        "TokyoNight"
Set TypingSpeed  80ms

Hide
Type "./mkm"
Enter
Sleep 1500ms
Show

# Open settings. VHS requires the uppercase form (Ctrl+S) — the
# lowercase variant fails to parse on Ctrl+s specifically.
Ctrl+S
Sleep 1200ms

# Cycle a few themes with the right arrow — the palette swatch under the
# theme row updates in step.
Right
Sleep 700ms
Right
Sleep 700ms
Right
Sleep 700ms
Right
Sleep 800ms

# Type-ahead: 'm' jumps to the next theme starting with m (mono).
Type "m"
Sleep 1200ms

# 'g' jumps to the next g-theme (github-dark, then gruvbox-dark on repeat).
Type "g"
Sleep 1000ms
Type "g"
Sleep 1200ms

# Esc reverts the preview to the originally-saved theme and closes settings.
Escape
Sleep 800ms

# Quit.
Escape
Sleep 500ms
