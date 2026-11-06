Output demo/nameary.gif

Set Shell "bash"
Set FontSize 14
Set Width 1200
Set Height 760
Set Padding 24
Set Theme "Catppuccin Mocha"
Set TypingSpeed 55ms

Type "./nameary acme --preset all --cache-mode off"
Enter
Wait+Screen /done/
Sleep 1s
Type "q"
Sleep 500ms
Type "./nameary acme --format console --preset all --cache-mode off"
Enter
Wait+Screen /available.*taken/
Sleep 1s
