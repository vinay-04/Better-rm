# 🗑️ better-rm - A Better `rm` Command

> _Another vibe-coded app that makes file deletion safe and recoverable_

**better-rm** is a drop-in replacement for the traditional Unix `rm` command, but with a twist - it's actually safe to use! Instead of permanently deleting your files into the void, it moves them to a smart recycle bin with compression, giving you a second chance when you inevitably delete something important.

## 🌟 Why better-rm?

We've all been there - that heart-stopping moment when you realize you just ran `rm -rf` on the wrong directory. Traditional `rm` is unforgiving and permanent. **better-rm** keeps all the familiar `rm` functionality you know and love, but adds a safety net that actually works.

### ✨ Key Features

- 🛡️ **Safe by Default**: Files go to a recycle bin instead of digital heaven
- 🗜️ **Smart Compression**: Uses gzip to save space while preserving everything
- 🔄 **Easy Recovery**: Restore files with a simple command
- ⚡ **Fast Performance**: Optimized compression for speed
- 🎯 **Full Compatibility**: Supports all GNU `rm` options and flags
- 🧹 **Auto Cleanup**: Configurable retention periods (default: 7 days)
- 🔐 **Security First**: Protection against path traversal and other nasties
- 💬 **Human Friendly**: Natural prompts and clear error messages

## 🚀 Quick Start

### Installation

```bash
# Clone and build
git clone <your-repo-url>
cd better-rm
go build -o better-rm
sudo mv better-rm /usr/local/bin/
```

### First Run Setup

```bash
# Setup your recycle bin (one-time setup)
better-rm --setup-recycle-bin

# Now use it just like regular rm!
better-rm myfile.txt
better-rm -rf old_project/
```

## 📖 Usage Guide

### Basic Operations

```bash
# Remove a file (goes to recycle bin)
better-rm document.pdf

# Remove multiple files
better-rm *.tmp log.txt cache.db

# Remove directory recursively
better-rm -r old_folder/

# Verbose output (see what's happening)
better-rm -v important_file.txt
```

### Interactive Mode

```bash
# Prompt before each removal
better-rm -i *.txt

# Prompt once for multiple files
better-rm -I many files here

# Force removal (skip prompts)
better-rm -f stubborn_file.txt
```

### Recycle Bin Management

```bash
# List what's in your recycle bin
better-rm --list-recycle-bin

# Restore a specific file
better-rm --restore=document.pdf

# Clear everything permanently (careful!)
better-rm --clear-recycle-bin

# Set custom retention period
better-rm --recycle-bin-days=14
```

### Advanced Options

```bash
# Bypass recycle bin (permanent deletion)
better-rm --permanent sensitive_data.txt

# Remove empty directories
better-rm -d empty_folder/

# Stay on same filesystem
better-rm --one-file-system large_directory/
```

## 🎛️ All Command Options

| Option                  | Description                                         |
| ----------------------- | --------------------------------------------------- |
| `-f, --force`           | Never prompt, ignore nonexistent files              |
| `-i`                    | Prompt before every removal                         |
| `-I`                    | Prompt once before removing 3+ files or recursively |
| `-r, -R, --recursive`   | Remove directories and contents recursively         |
| `-d, --dir`             | Remove empty directories                            |
| `-v, --verbose`         | Explain what is being done                          |
| `--interactive[=WHEN]`  | Control prompting (never/once/always)               |
| `--one-file-system`     | Stay within same filesystem                         |
| `--preserve-root[=all]` | Don't remove '/' (default behavior)                 |
| `--no-preserve-root`    | Allow removal of '/' (not recommended!)             |
| `--permanent`           | Skip recycle bin, delete immediately                |
| `--setup-recycle-bin`   | Configure recycle bin settings                      |
| `--list-recycle-bin`    | Show what's in the recycle bin                      |
| `--clear-recycle-bin`   | Permanently empty recycle bin                       |
| `--restore=PATH`        | Restore file from recycle bin                       |
| `--recycle-bin-days=N`  | Set retention period (default: 7 days)              |
| `--help`                | Show help message                                   |
| `--version`             | Show version information                            |

## 💾 How It Works

### The Smart Recycle Bin

When you "delete" a file with better-rm:

1. **File gets moved** to `~/.local/share/better-rm/recycle-bin/`
2. **Compressed with gzip** (using fastest compression for performance)
3. **Metadata stored** with original path, deletion time, and compression info
4. **Unique naming** prevents conflicts using timestamp + hash
5. **Auto cleanup** removes files older than retention period

### File Naming Convention

```
20240909_143022_a1b2c3d4_document.pdf.gz
│       │        │         │           └─ Compression extension
│       │        │         └─ Original filename
│       │        └─ 8-char hash of original path
│       └─ Time deleted (YYYYMMDD_HHMMSS)
└─ Date deleted
```

### Compression Stats

Files are compressed using gzip with `BestSpeed` setting for optimal performance:

- **Text files**: Often 60-80% size reduction
- **Images/Videos**: Minimal compression (already compressed)
- **Code files**: Usually 40-70% smaller
- **Directories**: Contents compressed individually

## 🔒 Security Features

- ✅ **Path traversal protection** - Can't escape intended directories
- ✅ **Root directory protection** - Won't let you delete `/` by accident
- ✅ **Atomic operations** - Metadata writes are crash-safe
- ✅ **Secure permissions** - Files: 0600, Directories: 0700
- ✅ **Input validation** - All user inputs are sanitized
- ✅ **Size limits** - Configurable max recycle bin size

## ⚙️ Configuration

### Default Configuration

```json
{
  "version": "1.0.0",
  "recycle_bin_path": "~/.local/share/better-rm/recycle-bin",
  "retention_days": 7,
  "max_size_mb": 1024
}
```

### Custom Configuration

```bash
# Setup with custom path and retention
better-rm --setup-recycle-bin
# Follow the prompts to customize

# Or modify config file directly
~/.config/better-rm/config.json
```

## 🔍 Examples

### Real-World Scenarios

```bash
# Cleaning up a project (safe!)
better-rm -r node_modules/ dist/ *.log

# Oops, need that back
better-rm --restore=package-lock.json

# Check what you've deleted recently
better-rm --list-recycle-bin

# Actually delete sensitive files permanently
better-rm --permanent secrets.txt tokens.json
```

### Migration from `rm`

```bash
# Instead of this dangerous command:
rm -rf /var/log/*.log

# Use this safer version:
better-rm -rf /var/log/*.log

# And if you need to restore something:
better-rm --restore=/var/log/important.log
```

## 🚨 Important Notes

### What's Protected

- **Root directory** (`/`) - Always protected unless `--no-preserve-root`
- **Current/Parent dirs** (`.` and `..`) - Refused by default
- **Device files** - System protection built-in
- **Read-only files** - Will prompt in interactive mode

### Performance Considerations

- **Large files**: Compression is fast but still takes time
- **Network filesystems**: May be slower due to compression overhead
- **SSD vs HDD**: Compression can actually improve performance on SSDs

## 🤝 Contributing

This is a vibe-coded project, born from the frustration of `rm` being too scary to use. Feel free to contribute improvements, but keep the human-friendly philosophy!

### Development

```bash
# Build for development
go build -o better-rm

# Run tests
./test_better_rm.sh

# Format code
go fmt ./...

# Vet for issues
go vet ./...
```

## 📜 License

MIT License - Use it, modify it, share it. Just don't blame us if you still manage to delete something important! 😄

## 🙋 FAQ

**Q: Is this really safer than regular `rm`?**  
A: Absolutely! Unless you use `--permanent`, everything goes to a recoverable recycle bin.

**Q: What about performance?**  
A: It's optimized for speed. The compression is fast, and for most use cases, you won't notice a difference.

**Q: Can I use this as a complete `rm` replacement?**  
A: Yes! It supports all the same flags and options as GNU `rm`.

**Q: What happens when the recycle bin gets full?**  
A: It automatically cleans up old files and warns you when approaching limits.

**Q: Can I restore files after the retention period?**  
A: Nope, they're gone forever after cleanup. But 7 days is usually plenty of time to notice!

---

**Made with ❤️ and lots of caffeine** - _Another vibe-coded app for humans who make mistakes_
