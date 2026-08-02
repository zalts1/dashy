# The README's GIF. Regenerate with demo/record.sh, which builds the fixture renderer
# first so a recording can only ever show the code that is in the tree.
#
# The background is #282c34 — the lighter of the two terminal backgrounds every colour
# in the palette was validated against (DESIGN.md §6). Recording on a theme board's
# colours were never checked against would put an unvalidated contrast claim at the top
# of the README.
#
# Height must leave the frame at least 32 rows. Below that the fit loop starts trimming
# the quiet band to `+N quiet`, which is correct behaviour and the wrong thing to lead
# with.

Output docs/board.gif

Set Shell bash
Set FontSize 13
Set Width 880
Set Height 560
Set Padding 20
Set Framerate 20
Set TypingSpeed 55ms
Set Theme { "background": "#282c34", "foreground": "#c3c2b7", "cursor": "#c3c2b7" }

# A bare prompt, and the fixture renderer ahead of any real board on PATH. COLUMNS and
# LINES are exported because the demo sizes the frame from them, exactly as the
# documented capture path does (`COLUMNS=118 LINES=44 ./board watch | cat`).
Hide
Type `export PS1='$ ' && export COLUMNS=$(tput cols) LINES=$(tput lines) && export PATH="$PWD/demo/bin:$PATH"`
Enter
Type "clear"
Enter
Show

Type "board watch"
Sleep 600ms
Enter

# Ends inside the last beat rather than after it: the demo exits by restoring the
# shell, and a loop that ends on a bare prompt reads as a crash.
Sleep 19s
