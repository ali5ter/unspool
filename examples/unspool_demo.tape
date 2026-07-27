Output unspool_demo.gif

Source config.tape

Type "./unspool"
Sleep 500ms
Enter

# Splash: logo + sync against the real account
Sleep 5s

# Feed tab: browse a few videos — selection updates the preview pane,
# including a chafa-rendered thumbnail once it loads
Sleep 1s
Down
Sleep 2.5s
Down
Sleep 2.5s
Down
Sleep 2.2s

# Move focus to the preview column and scroll it
Right
Sleep 800ms
Down
Sleep 400ms
Down
Sleep 400ms
Up
Sleep 400ms

# Back to the list column
Left
Sleep 800ms
Down
Sleep 500ms

# Queue tab
Tab
Sleep 1.5s
Down
Sleep 400ms

# Playlists tab — three columns: playlists, that playlist's videos, video detail
Tab
Sleep 1.5s
Down
Sleep 600ms
Down
Sleep 600ms

# Focus the videos column
Right
Sleep 800ms
Down
Sleep 500ms
Down
Sleep 500ms

# Focus the detail column
Right
Sleep 800ms
Down
Sleep 400ms

# Back to playlists column
Left
Sleep 400ms
Left
Sleep 800ms

# Liked tab — loads on first view, live API call
Tab
Sleep 2.5s
Down
Sleep 400ms
Down
Sleep 400ms

# Recommended tab — synthesised locally from watch history, subscriptions,
# and per-channel cached videos; no network/quota cost
Tab
Sleep 1.5s
Down
Sleep 400ms
Down
Sleep 400ms

# Local search — instant, scans cached subscription videos, watch log, and
# playlist titles for free ("y" would escalate to a live, rationed YouTube
# search — not shown here to keep the demo's quota cost at zero)
Type "/"
Sleep 400ms
Type "synth"
Sleep 400ms
Enter
Sleep 1.2s
Escape
Sleep 500ms

# Back to Feed
Tab
Sleep 1s

# Quit
Type "q"
Sleep 1s
