# zx

**zx** is a powerful, interactive terminal-based search tool with a modern TUI interface. Built in Go with Bubble Tea, it combines the power of regex search with intuitive file management and smart performance optimization.

---

## Features

### **Interactive TUI Interface**
- **File Browser Mode**: Navigate directories with vim-style keys
- **Search Input Mode**: Enter regex patterns with real-time feedback
- **Search Results Mode**: Browse matches with syntax highlighting
- **Progress Mode**: Real-time search progress with ETA calculations

### **Smart File Management**
- **Multi-Selection**: Select files and directories for targeted searches
- **Directory Navigation**: Enter/exit directories seamlessly
- **Selection Modes**: 
  - `Space`: Toggle individual files/directories
  - `d`: Toggle directory selection (multiple allowed)
  - `f`: Select all files
  - `Ctrl+D`: Select all directories
  - `a`: Select all items
  - `A`: Deselect all

### **Advanced Search Capabilities**
- **Regex Support**: Full regular expression pattern matching
- **Parallel Processing**: Multi-threaded search with configurable workers
- **Smart Filtering**: Automatic binary file detection and exclusion
- **Memory Management**: Configurable limits for large datasets
- **Progress Tracking**: Real-time progress with file count and data processed

### **Performance Optimization**
- **Auto-Configuration**: Automatically adjusts settings based on dataset size
- **Large File Handling**: Configurable file size limits (100MB - 2GB)
- **Concurrent Workers**: Scales from 10 to 100+ workers based on CPU cores
- **Memory Limits**: Prevents memory exhaustion on massive datasets
- **Binary Detection**: Skips binary files for faster processing

### **Analysis & Diagnostics**
- **Folder Analysis**: Shows file statistics and recommendations
- **Configuration Mode**: Tune performance settings manually
- **Error Reporting**: Detailed error messages and suggestions
- **Search Statistics**: File counts, processing time, and match statistics

---

## Quick Start

### Interactive Mode (Recommended)
```bash
./zx
```
Navigate with arrow keys or vim keys (`j`/`k`), select files/directories, and press `s` to search.

### Command Line Mode (Legacy)
```bash
./zx "pattern" /path/to/search
```

---

## Key Bindings

### File Browser Mode
| Key | Action |
|-----|--------|
| `↑`/`k` | Move up |
| `↓`/`j` | Move down |
| `Enter` | Enter directory / Toggle file selection |
| `Space` | Toggle file/directory selection |
| `Ctrl+Enter` | Toggle directory selection (without entering) |
| `d` | Toggle directory selection (multiple allowed) |
| `f` | Select all files only |
| `Ctrl+D` | Select all directories |
| `a` | Select all files and directories |
| `A` | Deselect all |
| `s`/`/` | Start search |
| `c` | Configuration mode |
| `i` | Analyze folder structure |
| `r` | Refresh directory |
| `h`/`?` | Toggle help |
| `q`/`Ctrl+C` | Quit |

### Search Input Mode
| Key | Action |
|-----|--------|
| `Enter` | Start search |
| `Esc`/`Ctrl+C` | Cancel |
| `Backspace` | Delete character |

### Search Results Mode
| Key | Action |
|-----|--------|
| `↑`/`k` | Move up through results |
| `↓`/`j` | Move down through results |
| `g`/`Home` | Go to first result |
| `G`/`End` | Go to last result |
| `s`/`/` | Start new search |
| `Esc`/`q` | Return to file browser |

---

## Configuration

### Performance Settings
Access configuration mode with `c` key:

- **Max File Size**: 100MB → 1GB (files larger than limit are skipped)
- **Max Results**: 10K → 50K (maximum search results in memory)
- **Concurrency**: 50 → 2x CPU cores (parallel worker threads)

### Auto-Configuration
The tool automatically analyzes your dataset and adjusts settings:
- **Small projects** (< 1K files): Conservative settings
- **Medium projects** (1K-10K files): Balanced settings  
- **Large projects** (10K+ files): High-performance settings

---

## Performance

### Optimized for Large Datasets
- **100GB+ codebases**: Tested and optimized
- **Parallel processing**: Utilizes all CPU cores
- **Memory efficient**: Streaming search prevents memory exhaustion
- **Smart filtering**: Skips binary files, hidden files, and oversized files
- **Progress tracking**: Real-time ETA for long searches

### Benchmark Examples
- **Linux kernel** (~70K files): ~30 seconds
- **Chromium** (~300K files): ~2-3 minutes
- **Small projects** (<1K files): <1 second

---

## Visual Features

- **Syntax Highlighting**: Matches highlighted in search results
- **File Metadata**: Shows file sizes, modification times
- **Progress Bars**: Visual progress indication with percentages
- **Status Messages**: Clear feedback for all operations
- **Error Handling**: Graceful error display with suggestions
- **Modern UI**: Clean, responsive terminal interface

---

## Search Examples

### Basic Patterns
```
func.*main          # Find function definitions containing 'main'
TODO|FIXME          # Find TODO or FIXME comments
error.*return       # Find error handling patterns
import.*fmt         # Find fmt imports
```

### Advanced Regex
```
\b[A-Z][a-z]+Error\b    # Find custom error types
func\s+\w+\([^)]*\)     # Find function signatures
//.*TODO.*              # Find TODO comments
```

---

## Building from Source

```bash
git clone <repository>
cd zx
go build -o zx
```

### Dependencies
- Go 1.19+
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - TUI framework
- [Lipgloss](https://github.com/charmbracelet/lipgloss) - Styling


---

## 📄 License

MIT License - see LICENSE file for details.

---

## Acknowledgments

- Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) TUI framework
- Inspired by modern terminal tools like `fzf`, `ripgrep`, and `fd`
