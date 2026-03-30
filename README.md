# tgs

Send files and folders to Telegram from the terminal.

```
tgs photo.jpg                   # send to Saved Messages
tgs ./my-project                # zip & send folder to Saved Messages
tgs @username report.pdf        # send to a specific user/chat
```

Folders are automatically zipped with common junk excluded (`node_modules`, `__pycache__`, `.venv`, `.git`, etc).

## Install

### Homebrew (macOS / Linux)

```bash
brew install freQuensy23-coder/tap/tgs
```

### AUR (Arch Linux)

```bash
yay -S tgs-bin
```

### From source

```bash
go install github.com/freQuensy23-coder/tgs/cmd/tgs@latest
```

### Binary

Download from [Releases](https://github.com/freQuensy23-coder/tgs/releases).

## Setup

### As user (full features)

```bash
tgs login user
```

Prompts for API ID & Hash from [my.telegram.org](https://my.telegram.org), phone number, auth code, and optional 2FA. Supports Saved Messages, dialog search, files up to 2 GB.

### As bot

```bash
tgs login bot
```

Prompts for bot token from [@BotFather](https://t.me/BotFather) and your numeric chat ID. Files up to 50 MB.

## Usage

```
tgs <file>                  Send file to Saved Messages
tgs <folder>                Send folder as zip to Saved Messages
tgs <user> <file|folder>    Send to specific user/chat
tgs login bot               Setup bot authentication
tgs login user              Setup user authentication
```

When sending to a user/chat, tgs searches your top 50 dialogs by name first, then falls back to username lookup.

Config is stored in `~/.tgs/`.

## License

MIT
