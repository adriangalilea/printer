# Printer 🖨️

A sophisticated TUI (Terminal User Interface) for managing print queues on macOS, built with Go and [Bubble Tea](https://github.com/charmbracelet/bubbletea).

## Features

### 📊 Queue Management
- **Real-time monitoring** of system print queue with automatic refresh
- **Batch operations** for canceling multiple jobs
- **Job details** including file names, sizes, and status
- **Quick actions** to open files or their containing folders

### 📁 Advanced File Browser
- **Dual-pane interface** with keyboard-driven navigation
- **Smart file detection** automatically identifies printable formats (PDF, DOC, images)
- **Glob pattern matching** for filtering files (e.g., `*.pdf`, `report-*.doc`)
- **Directory navigation** with arrow keys or vim bindings
- **Batch selection** with toggle-all functionality for printable files
- **Visual indicators** distinguish directories 📁, printable files 📄, and regular files

### 🎨 Responsive UI
- **Adaptive layout** that scales to terminal size
- **Focus indicators** with colored borders showing active pane
- **Scroll support** for long file lists with position indicators
- **Path display** showing current directory in file browser
- **XDG-compliant** data storage respecting system standards

## Installation

```bash
# Clone and build
git clone https://github.com/adriangalilea/printer.git
cd printer
make install  # Installs to ~/.local/bin
```

Ensure `~/.local/bin` is in your PATH.

## Usage

### Launch Modes

```bash
# Open queue manager
printer

# Start in file picker with pattern
printer add ~/Documents/*.pdf
printer add "~/reports/2024-*.pdf"
```

### Keyboard Shortcuts

#### Queue Mode
| Key | Action |
|-----|--------|
| `a` | Add files (opens file browser) |
| `o` | Open selected file |
| `O` | Open file's folder |
| `x` | Cancel selected job |
| `X` | Cancel all marked jobs |
| `Space` | Mark/unmark job |
| `r` | Refresh queue |
| `q` | Quit |

#### File Browser Mode
| Key | Action |
|-----|--------|
| `Tab` | Switch between input field and file list |
| `↑/k` | Navigate up (moves to input when at top) |
| `↓/j` | Navigate down (moves to files from input) |
| `←/h/Backspace` | Go to parent directory |
| `→/l` | Enter directory |
| `Space` | Mark/unmark file (or toggle all) |
| `Enter` | Add marked files / enter directory |
| `Esc` | Return to queue |

### File Pattern Matching

The input field supports glob patterns:
- `*.pdf` - All PDFs in current directory
- `report-*.doc` - All docs starting with "report-"
- `**/*.pdf` - All PDFs recursively (if supported)

## Architecture

### Core Components

- **main.go** - TUI application logic with Bubble Tea framework
- **system.go** - System integration for print queue operations
- **tracker.go** - Persistent job tracking with XDG-compliant storage

### Data Persistence

Print job metadata is stored in `$XDG_DATA_HOME/printer/jobs.json` (defaults to `~/.local/share/printer/`) to maintain file path associations and enable file/folder opening features.

## Supported File Types

Automatically detected as printable:
- Documents: `.pdf`, `.txt`, `.doc`, `.docx`
- Images: `.jpg`, `.jpeg`, `.png`, `.gif`

## Requirements

- macOS (uses `lpr`, `lpstat`, `cancel` commands)
- Go 1.19+ for building
- CUPS printing system (standard on macOS)

## Development

```bash
# Build
make build

# Run without installing
make run

# Clean build artifacts
make clean
```

## Technical Stack

- **[Bubble Tea](https://github.com/charmbracelet/bubbletea)** - Terminal UI framework
- **[Lip Gloss](https://github.com/charmbracelet/lipgloss)** - Style definitions for terminal layouts
- **[Bubbles](https://github.com/charmbracelet/bubbles)** - TUI component library

## License

MIT

## Author

Adrian Galilea ([@adriangalilea](https://github.com/adriangalilea))