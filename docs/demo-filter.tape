# demo-filter.tape — vertical slice for the fuzzy-filter feature.
#
# Showcases:
#   - description-aware matching ("redis" finds cache-flushdb via desc)
#   - ^w to delete the previous filter word
#   - the N/M match-count display
#
# Run with:  make demo-filter

Output docs/demo-filter.gif

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

# "redis" isn't in any target name — it's only in cache-flushdb's
# description ("Flush the entire Redis DB"). Description matching
# surfaces it anyway.
Type "redis"
Sleep 2s

# ^w wipes the word in one keystroke.
Ctrl+w
Sleep 800ms

# A more typical name match — narrows from hundreds to a handful.
# The N/M counter on the top right updates as you type.
Type "deploy"
Sleep 2s

# Two-stage esc: first press clears the filter, second quits.
Escape
Sleep 1s
Escape
Sleep 500ms
