# pgincident demo
# Prerequisites: docker compose up -d && make dev-load (running in another terminal)

Output demo.gif

Set FontSize 14
Set Width 1200
Set Height 700
Set Theme "Dracula"
Set Shell "bash"

# Start pgincident with the dev config
Type "pgincident --config dev/pgincident-dev.toml"
Sleep 500ms
Enter

# Overview screen — let it render and refresh once
Sleep 4s

# Navigate to Dashboard screen
Type "o"
Sleep 4s

# Move cursor down to a Long-running query row and open SQL detail
Type "j"
Sleep 300ms
Enter
Sleep 3s

# Close detail and quit
Type "q"
Sleep 500ms
