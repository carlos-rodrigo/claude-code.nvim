# claude-code.nvim

A Neovim plugin that integrates [Claude Code](https://docs.anthropic.com/en/docs/build-with-claude/claude-code) directly into your editor as a buffer. Work with Claude AI seamlessly within your Neovim workflow using splits, tabs, or floating windows.

## ✨ Features

- **Buffer-based integration** - Works as regular Neovim buffers with splits/tabs
- **Flexible window management** - Choose between splits, tabs, or floating windows
- **Visual selection sending** - Send selected code directly to Claude with a keymap
- **Session persistence** - Keep your Claude conversation active while navigating files
- **Session management** - Automatically saves sessions and supports multiple concurrent sessions
- **Auto-save on focus loss** - Sessions are automatically saved when you switch buffers or lose focus
- **Named session saving** - Save sessions with custom names and manage them easily
- **Session browsing** - Browse and view previous Claude Code conversations
- **Session restoration** - Restore saved sessions as new active sessions to continue conversations
- **Start with selection** - Create new sessions with selected text as initial prompt
- **Auto-scrolling** - Keeps the latest Claude responses visible
- **LazyVim integration** - Follows LazyVim conventions with lazy loading
- **Project context** - Send your project structure to Claude for better assistance
- **Terminal mode navigation** - Use Esc to exit terminal mode and navigate with vim motions

## 📦 Installation

### Prerequisites

1. **Install Claude Code CLI** - Available from [Anthropic](https://docs.anthropic.com)
2. **Ensure it's in your PATH** - `claude-code` command should be accessible
3. **LazyVim setup** - This plugin is designed for LazyVim

### Using LazyVim

Add this to your LazyVim plugins directory (`~/.config/nvim/lua/plugins/claude-code.lua`):

#### Option 1: Full LazyVim Integration (Recommended)

```lua
return {
  "carlos-rodrigo/claude-code.nvim",
  keys = {
    { "<leader>cc", "<cmd>ClaudeCodeToggle<cr>", desc = "claude: toggle" },
    { "<leader>cn", "<cmd>ClaudeCodeNew<cr>", desc = "claude: new session" },
    { "<leader>cs", "<cmd>ClaudeCodeSend<cr>", desc = "claude: send selection", mode = "v" },
    { "<leader>cS", "<cmd>ClaudeCodeSaveSession<cr>", desc = "claude: save session" },
    { "<leader>cu", "<cmd>ClaudeCodeUpdateSession<cr>", desc = "claude: update session" },
    { "<leader>cb", "<cmd>ClaudeCodeSessions<cr>", desc = "claude: browse sessions" },
    { "<leader>cr", "<cmd>ClaudeCodeRestoreSession<cr>", desc = "claude: restore session" },
    { "<leader>cw", "<cmd>ClaudeCodeNewWithSelection<cr>", desc = "claude: new with selection", mode = "v" },
  },
  config = function()
    require("claude-code").setup({
      claude_code_cmd = "claude",
      window = {
        type = "vsplit",        -- "split", "vsplit", "tabnew", "float"
        position = "right",     -- "right", "left", "top", "bottom"
        size = 80,             -- columns for vsplit, lines for split
      },
      auto_scroll = true,
      save_session = true,
      auto_save_session = true,    -- Auto-save on focus loss
      auto_save_notify = true,     -- Show notifications when auto-saving
      session_dir = vim.fn.stdpath("data") .. "/claude-code-sessions/",
      -- Disable built-in keybindings since we're using LazyVim keys spec
      keybindings = false,
    })
  end,
}
```

#### Option 2: Command-based Loading

```lua
return {
  "carlos-rodrigo/claude-code.nvim",
  cmd = { 
    "ClaudeCode", 
    "ClaudeCodeNew", 
    "ClaudeCodeToggle", 
    "ClaudeCodeSessions", 
    "ClaudeCodeSaveSession", 
    "ClaudeCodeUpdateSession",
    "ClaudeCodeRestoreSession",
    "ClaudeCodeNewWithSelection" 
  },
  config = function()
    require("claude-code").setup({
      claude_code_cmd = "claude",
      window = {
        type = "vsplit",        -- "split", "vsplit", "tabnew", "float"
        position = "right",     -- "right", "left", "top", "bottom"
        size = 80,             -- columns for vsplit, lines for split
      },
      auto_scroll = true,
      save_session = true,
      auto_save_session = true,    -- Auto-save on focus loss
      auto_save_notify = true,     -- Show notifications when auto-saving
      session_dir = vim.fn.stdpath("data") .. "/claude-code-sessions/",
      -- Default keybindings (can be customized)
      keybindings = {
        toggle = "<leader>cc",
        new_session = "<leader>cn", 
        send_selection = "<leader>cs",
        save_session = "<leader>cS",
        update_session = "<leader>cu",
        browse_sessions = "<leader>cb",
        restore_session = "<leader>cr",
        new_with_selection = "<leader>cw",
      },
    })
  end,
}
```

### Manual Installation

1. Clone this repository to your Neovim configuration:

```bash
git clone https://github.com/carlos-rodrigo/claude-code.nvim ~/.config/nvim/pack/plugins/start/claude-code.nvim
```

2. Add the setup to your `init.lua`:

```lua
require("claude-code").setup()
```

## 🚀 Usage

### Commands

| Command                      | Description                                        |
| ---------------------------- | -------------------------------------------------- |
| `:ClaudeCode`                | claude: open buffer                               |
| `:ClaudeCodeNew`             | claude: new session                               |
| `:ClaudeCodeToggle`          | claude: toggle window                             |
| `:ClaudeCodeSend`            | claude: send selection                            |
| `:ClaudeCodeSaveSession`     | claude: save session                              |
| `:ClaudeCodeUpdateSession`   | claude: update session                            |
| `:ClaudeCodeSessions`        | claude: browse sessions                           |
| `:ClaudeCodeRestoreSession`  | claude: restore session                           |
| `:ClaudeCodeNewWithSelection`| claude: new with selection                        |

### Default Keybindings

| Key          | Mode   | Action                        |
| ------------ | ------ | ----------------------------- |
| `<leader>cc` | Normal | claude: toggle window         |
| `<leader>cn` | Normal | claude: new session           |
| `<leader>cs` | Visual | claude: send selection        |
| `<leader>cS` | Normal | claude: save session          |
| `<leader>cu` | Normal | claude: update session        |
| `<leader>cb` | Normal | claude: browse sessions       |
| `<leader>cr` | Normal | claude: restore session       |
| `<leader>cw` | Visual | claude: new with selection    |

### In Claude Code Buffer

| Key     | Mode     | Action                      |
| ------- | -------- | --------------------------- |
| `q`     | Normal   | Close the window            |
| `<Esc>` | Normal   | Close the window            |
| `<Esc>` | Terminal | Exit to normal mode         |
| `<C-q>` | Terminal | Close the window            |
| `i`     | Normal   | Enter terminal (insert) mode|

### Tab Navigation (tabnew mode only)

| Key            | Mode     | Action           |
| -------------- | -------- | ---------------- |
| `<C-PageDown>` | Terminal | Next tab         |
| `<C-PageUp>`   | Terminal | Previous tab     |
| `<A-l>`        | Terminal | Next tab         |
| `<A-h>`        | Terminal | Previous tab     |
| `<PageDown>`   | Normal   | Next tab         |
| `<PageUp>`     | Normal   | Previous tab     |
| `<A-l>`        | Normal   | Next tab         |
| `<A-h>`        | Normal   | Previous tab     |

## ⚙️ Configuration

### Window Types

```lua
-- Vertical split on the right (default)
window = {
  type = "vsplit",
  position = "right",
  size = 80,  -- 80 columns wide
}

-- Horizontal split at the bottom
window = {
  type = "split",
  position = "bottom",
  size = 15,  -- 15 lines tall
}

-- New tab (full screen)
window = {
  type = "tabnew",
}

-- Floating window
window = {
  type = "float",
  position = "right",
  size = 0.4,  -- 40% of screen width
}
```

### Full Configuration Options

```lua
require("claude-code").setup({
  -- Command to run Claude Code CLI
  claude_code_cmd = "claude",

  -- Window configuration
  window = {
    type = "vsplit",        -- "split", "vsplit", "tabnew", "float"
    position = "right",     -- "right", "left", "top", "bottom"
    size = 80,             -- for splits: lines/columns, for float: percentage (0.4)
  },

  -- Auto-scroll to bottom when new content appears
  auto_scroll = true,

  -- Save sessions to files
  save_session = true,
  auto_save_session = true,  -- Automatically save sessions on focus loss
  auto_save_notify = true,   -- Show notification when auto-saving sessions
  session_dir = vim.fn.stdpath("data") .. "/claude-code-sessions/",

  -- Default keybindings (set to false to disable, or change keys)
  keybindings = {
    toggle = "<leader>cc",           -- Toggle Claude Code window
    new_session = "<leader>cn",      -- Start new session
    send_selection = "<leader>cs",   -- Send selection to Claude (visual mode)
    save_session = "<leader>cS",     -- Save session with name
    update_session = "<leader>cu",   -- Update current session
    browse_sessions = "<leader>cb",  -- Browse sessions
    restore_session = "<leader>cr",  -- Restore session
    new_with_selection = "<leader>cw", -- New session with selection (visual mode)
  },
})
```

## 🔧 Customization

### Built-in Keybindings

The plugin now sets up keybindings automatically! You don't need to specify them in your LazyVim configuration. The default keybindings are set up when you call `setup()`.

### Customizing Keybindings

You can customize or disable keybindings through the setup configuration:

```lua
require("claude-code").setup({
  keybindings = {
    toggle = "<leader>ai",           -- Change to different key
    new_session = "<leader>an",      -- Custom keybinding
    send_selection = false,          -- Disable this keybinding
    save_session = "<leader>aS",     -- Use different key combination
    -- ... other keybindings
  },
})
```

### Alternative: Manual Keybindings (LazyVim keys spec)

If you prefer to manage keybindings manually, disable the built-in ones and use LazyVim's keys spec:

```lua
return {
  "carlos-rodrigo/claude-code.nvim",
  cmd = { "ClaudeCode", "ClaudeCodeNew", "ClaudeCodeToggle", ... },
  keys = {
    { "<leader>ai", "<cmd>ClaudeCodeToggle<cr>", desc = "claude: toggle claude code" },
    { "<leader>an", "<cmd>ClaudeCodeNew<cr>", desc = "claude: new claude code session" },
    { "<leader>as", "<cmd>ClaudeCodeSend<cr>", mode = "v", desc = "claude: send to claude code" },
  },
  config = function()
    require("claude-code").setup({
      keybindings = false, -- Disable all built-in keybindings
      -- ... other config
    })
  end,
}
```

### Integration with Other Plugins

The plugin works well with:

- **nvim-tree** - Use with vertical splits for a complete IDE layout
- **telescope** - Send search results to Claude for analysis
- **trouble** - Get Claude's help with diagnostics
- **gitsigns** - Send diffs to Claude for code review

## 📝 Examples

### Workflow Examples

1. **Code Review**: Select code, press `<leader>cs`, ask Claude to review
2. **Debugging**: Send error logs to Claude for analysis
3. **Documentation**: Send functions to Claude to generate docs
4. **Refactoring**: Get Claude's suggestions for code improvements

### Sample Session

```lua
-- Open Claude Code in a vertical split
:ClaudeCode

-- In the Claude buffer, you can:
-- - Ask questions about your codebase
-- - Get help with debugging
-- - Request code explanations
-- - Get refactoring suggestions
```

## 🐛 Troubleshooting

### Common Issues

**Claude Code not found**

```bash
# Make sure Claude Code is installed and in PATH
which claude-code
# Should return the path to claude-code executable
```

**Window positioning issues**

- Adjust `window.position` and `window.size` in your configuration
- Try different window types (`split`, `vsplit`, `tabnew`, `float`)

**Performance issues**

- Disable `auto_scroll` if you have performance problems
- Reduce session saving by setting `save_session = false`

**Keybinding conflicts**

- Modify the `keys` section in your plugin specification
- Check for conflicts with `:verbose map <leader>cc`

## 🤝 Contributing

Contributions are welcome! Please feel free to submit a Pull Request. For major changes, please open an issue first to discuss what you would like to change.

### Development Setup

1. Fork the repository
2. Clone your fork: `git clone https://github.com/carlos-rodrigo/claude-code.nvim`
3. Create a feature branch: `git checkout -b feature-name`
4. Make your changes
5. Test with a local Neovim setup
6. Submit a pull request

## 📄 License

MIT License - see [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- [Anthropic](https://anthropic.com) for creating Claude and Claude Code
- [LazyVim](https://github.com/LazyVim/LazyVim) for the excellent Neovim distribution
- The Neovim community for the amazing ecosystem

## 📚 Related Projects

- [Claude Code CLI](https://docs.anthropic.com) - The official Claude Code command-line tool
- [LazyVim](https://github.com/LazyVim/LazyVim) - The Neovim configuration this plugin is designed for
- [nvim-terminal.lua](https://github.com/s1n7ax/nvim-terminal) - Terminal integration inspiration

---

⭐ If you find this plugin useful, please give it a star on GitHub!
