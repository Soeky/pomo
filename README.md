# 🍅 pomo – Minimalist CLI Pomodoro Timer

pomo is a lightweight and fast Pomodoro timer for the command line.

Ideal for users who want to manage focus sessions directly via the terminal or integrate it into status bars like Waybar.

## Features

- Start focus sessions (`start`) and break sessions (`break`)
- Real-time status display (`status`)
- Stop the current session (`stop`)
- Show statistics (`stat`) for day, week, month, or year
- Session correction (`correct`) to retroactively start a session
- Bash and Zsh tab-completion
- Stores all sessions in a local SQLite database
- Configurable default times via `config.json`

## Installation

1. Install directly via go install (no need to clone):

```bash
go install github.com/Soeky/pomo@latest
```

Alternatively, clone manually:

```bash
git clone https://github.com/Soeky/pomo.git
cd pomo
make build
```

2. Install bash completion:

```bash
./install_completion.sh
```

This will automatically generate the completion script and update your system setup.  

On Arch Linux, make sure `bash-completion` is installed:

```bash
sudo pacman -S bash-completion
```
and optionally check that `.bashrc` includes:

```bash
if [ -f /usr/share/bash-completion/bash_completion ]; then
    . /usr/share/bash-completion/bash_completion
fi
```

## Usage

Start a focus session:

```bash
pomo start [duration] [topic]
```

Start a break:

```bash
pomo break [duration]
```

Stop the current session:

```bash
pomo stop
```

Show current session status:

```bash
pomo status
```

Display session statistics:

```bash
pomo stat [day|week|month|year|all]
```

Correct a session if you forgot to start:

```bash
pomo correct start 15m ProjectX
```

## Integration into Waybar

In my [dot](https://github.com/Soeky/dot) (dotfiles) repository, I integrate pomo into Waybar.

Example Waybar module:

```json
"custom/pomodoro": {
  "exec": "/home/seymen/repos/github.com/Soeky/dotfiles/scripts/pomoS",
  "return-type": "text",
  "interval": 1
},
```

## Integration into Swiftbar (on Mac)

In my [dot](https://github.com/Soeky/dot) (dotfiles) repository, I integrate pomo into Waybar.

Example Waybar module:

```json
"custom/pomodoro": {
  "exec": "/home/seymen/repos/github.com/Soeky/dotfiles/scripts/pomoS",
  "return-type": "text",
  "interval": 1
},
```

This way you can see your current session directly in your status bar.
