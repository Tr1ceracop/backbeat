# Backbeat

*You do the work. Backbeat keeps the beat.*

An automatic time tracker that watches your desktop activity and logs it to Jira Tempo — so you can stop guessing "what did I do for the last 3 hours?" every afternoon.

It runs quietly in the background, notices when you're active, when you're idle, when you're in a meeting, and when your laptop takes a nap. At the end of the day, just sync and your timesheets are done. Magic? No. Just a little daemon with good vibes.

## How it works

- **Active/idle detection** — talks to GNOME via D-Bus, no keylogging, no creepy stuff
- **Sleep/wake aware** — pauses when your laptop hibernates, resumes when you're back
- **Meeting detection** — notices when your mic or camera lights up via PipeWire
- **Local-first** — everything is stored in a local SQLite database, nothing leaves your machine until you say so
- **Sync on your terms** — push worklogs to Tempo whenever you're ready

## Get it

### Quick spin

```bash
nix shell github:Tr1ceracop/backbeat
```

### NixOS / Home Manager

```nix
# flake inputs
backbeat.url = "github:Tr1ceracop/backbeat";
backbeat.inputs.nixpkgs.follows = "nixpkgs";

# overlay + module
nixpkgs.overlays = [ backbeat.overlays.default ];
imports = [ backbeat.homeManagerModules.default ];

# that's it
services.backbeat.enable = true;
```

### From source

```bash
nix develop
go build -o backbeat .
```

## Getting started

```bash
# One-time setup — walks you through Jira & Tempo credentials
backbeat init

# Fire it up
backbeat start

# Tell it what you're working on
backbeat track PROJ-123

# Peek at what's going on
backbeat status

# Review your day
backbeat log

# Happy with it? Ship it to Tempo
backbeat sync

# Call it a day
backbeat stop
```

## Requirements

- Linux with GNOME on Wayland
- PipeWire
- Jira Cloud + Tempo Timesheets
- A distaste for manual timesheets
