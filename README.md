# claude-code.nvim

A Neovim plugin that integrates [Claude Code](https://docs.anthropic.com/en/docs/build-with-claude/claude-code) directly into your editor as a buffer. Work with Claude AI seamlessly within your Neovim workflow using splits, tabs, or floating windows.

## ✨ Features

- **Buffer-based integration** - Works as regular Neovim buffers with splits/tabs
- **Flexible window management** - Choose between splits, tabs, or floating windows
- **Visual selection sending** - Send selected code directly to Claude with a keymap
- **Session management** - Automatically saves sessions and supports multiple concurrent sessions
- **Auto-scrolling** - Keeps the latest Claude responses visible
- **LazyVim integration** - Follows LazyVim conventions with lazy loading
- **Project context** - Send your project structure to Claude for better assistance

## 📦 Installation

### Prerequisites

1. **Install Claude Code CLI** - Available from [Anthropic](https://docs.anthropic.com)
2. **Ensure it's in your PATH** - `claude-code` command should be accessible
3. **LazyVim setup** - This plugin is designed for LazyVim

### Using LazyVim

Add this to your LazyVim plugins directory (`~/.config/nvim/lua/plugins/claude-code.lua`):

```lua
return {
  "carlos-rodrigo/claude-code.nvim",
  cmd = { "ClaudeCode", "ClaudeCodeNew", "ClaudeCodeToggle" },
  keys = {
    { "<leader>cc", "<cmd>ClaudeCodeToggle<cr>", desc = "Toggle Claude Code" },
    { "<leader>cn", "<cmd>ClaudeCodeNew<cr>", desc = "New Claude Code session" },
    { "<leader>cs", "<cmd>ClaudeCodeSend<cr>", mode = "v", desc = "Send selection to Claude" },
  },
  config = function()
    require("claude-code").setup({
      claude_code_cmd = "claude-code",
      window = {
        type = "vsplit",        -- "split", "vsplit", "tabnew", "float"
        position = "right",     -- "right", "left", "top", "bottom"
        size = 80,             -- columns for vsplit, lines for split
      },
      auto_scroll = true,
      save_session = true,
      session_dir = vim.fn.stdpath("data") .. "/claude-code-sessions/",
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

| Command             | Description                                |
| ------------------- | ------------------------------------------ |
| `:ClaudeCode`       | Open Claude Code buffer                    |
| `:ClaudeCodeNew`    | Start a new Claude Code session            |
| `:ClaudeCodeToggle` | Toggle Claude Code window visibility       |
| `:ClaudeCodeSend`   | Send selected text to Claude (visual mode) |

### Default Keybindings

| Key          | Mode   | Action                        |
| ------------ | ------ | ----------------------------- |
| `<leader>cc` | Normal | Toggle Claude Code window     |
| `<leader>cn` | Normal | Start new Claude Code session |
| `<leader>cs` | Visual | Send selection to Claude      |

### In Claude Code Buffer

| Key     | Mode     | Action           |
| ------- | -------- | ---------------- |
| `q`     | Normal   | Close the window |
| `<Esc>` | Normal   | Close the window |
| `<C-q>` | Terminal | Close the window |

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
  claude_code_cmd = "claude-code",

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
  session_dir = vim.fn.stdpath("data") .. "/claude-code-sessions/",
})
```

## 🔧 Customization

### Custom Keybindings

```lua
-- Add to your LazyVim plugin spec
keys = {
  { "<leader>ai", "<cmd>ClaudeCodeToggle<cr>", desc = "Toggle Claude AI" },
  { "<leader>an", "<cmd>ClaudeCodeNew<cr>", desc = "New Claude session" },
  { "<leader>as", "<cmd>ClaudeCodeSend<cr>", mode = "v", desc = "Send to Claude" },
},
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
