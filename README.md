# claude-code.nvim

A Neovim plugin that integrates [Claude Code](https://docs.anthropic.com/en/docs/build-with-claude/claude-code) directly into your editor with **intelligent session management**. Features AI-powered session compression, semantic search, smart context building, and production-ready infrastructure for enhanced productivity.

## ✨ Features

### 🧠 Intelligent Session Management
- **AI-Powered Compression** - Reduces session size by 70-80% using local LLM processing
- **Semantic Search** - Find relevant conversations using vector embeddings
- **Smart Context Building** - Intelligently assembles context from multiple related sessions
- **Project Memory Consolidation** - Extracts patterns, decisions, and insights across sessions
- **Pattern Recognition** - Identifies recurring solutions, errors, and workflows
- **Real-time Analytics** - Comprehensive dashboard with usage statistics and insights

### 🔧 Production-Ready Infrastructure
- **Local LLM Processing** - Ollama integration with automatic model management
- **Enterprise Security** - API key authentication, rate limiting, input validation
- **High Availability** - Circuit breakers, graceful degradation, fallback systems
- **Monitoring & Observability** - Prometheus metrics, health checks, structured logging
- **Automated Backups** - Database backup with integrity verification and recovery
- **Performance Optimization** - Adaptive caching, memory management, resource optimization

### 💻 Editor Integration
- **Buffer-based integration** - Works as regular Neovim buffers with splits/tabs
- **Flexible window management** - Choose between splits, tabs, or floating windows
- **Multiple independent sessions** - Open Claude Code in tab and vsplit with separate conversations
- **Visual selection sending** - Send selected code directly to Claude with a keymap
- **Session persistence** - Keep your Claude conversation active while navigating files
- **Auto-scrolling** - Keeps the latest Claude responses visible
- **LazyVim integration** - Follows LazyVim conventions with lazy loading
- **Which-key integration** - Beautiful menu interface when pressing `<leader>cl`
- **Smart Esc handling** - Single Esc cancels Claude actions, double Esc exits terminal mode

### 🚀 Development Workflow
- **Custom Claude commands** - Install opinionated commands for planning, coding, and shipping
- **Built-in agents** - Three specialized agents (product-analyst, software-engineer, code-reviewer)
- **Flexible agent installation** - Choose between project-level or personal-level installation
- **Project context** - Send your project structure to Claude for better assistance
- **Incremental saving** - Updates existing sessions with only new content, avoiding duplication
- **Named session management** - Save, browse, and restore sessions with rich metadata

## 📦 Installation

### Prerequisites

1. **Install Claude Code CLI** - Available from [Anthropic](https://docs.anthropic.com)
2. **Ensure it's in your PATH** - `claude-code` command should be accessible
3. **LazyVim setup** - This plugin is designed for LazyVim
4. **Dependencies** - The plugin will automatically handle installation of required dependencies:
   - `plenary.nvim` - Auto-installed if missing (required for intelligence features)
5. **Intelligent Backend (Optional)** - Deploy the Claude Code Intelligence service for advanced features:
   - AI-powered session compression and search
   - Project memory consolidation
   - Advanced analytics and insights
   - See [Production Deployment Guide](claude-code-intelligence/docs/production/README.md)

### Using LazyVim

Add this to your LazyVim plugins directory (`~/.config/nvim/lua/plugins/claude-code.lua`):

#### Option 1: Full LazyVim Integration (Recommended)

```lua
return {
  "carlos-rodrigo/claude-code.nvim",
  dependencies = {
    "nvim-lua/plenary.nvim", -- Recommended: auto-installed if missing
  },
  keys = {
    { "<leader>clc", "<cmd>ClaudeCodeToggle<cr>", desc = "claude: toggle" },
    { "<leader>cln", "<cmd>ClaudeCodeNew<cr>", desc = "claude: new session" },
    { "<leader>cls", "<cmd>ClaudeCodeSend<cr>", desc = "claude: send selection", mode = "v" },
    { "<leader>clv", "<cmd>ClaudeCodeVsplit<cr>", desc = "claude: open in vsplit" },
    { "<leader>clS", "<cmd>ClaudeCodeSaveSession<cr>", desc = "claude: save session" },
    { "<leader>clu", "<cmd>ClaudeCodeUpdateSession<cr>", desc = "claude: update session" },
    { "<leader>clb", "<cmd>ClaudeCodeSessions<cr>", desc = "claude: browse sessions" },
    { "<leader>clr", "<cmd>ClaudeCodeRestoreSession<cr>", desc = "claude: restore session" },
    { "<leader>clw", "<cmd>ClaudeCodeNewWithSelection<cr>", desc = "claude: new with selection", mode = "v" },
  },
  config = function()
    require("claude-code").setup({
      claude_code_cmd = "claude",
      window = {
        type = "current",       -- "current", "split", "vsplit", "tabnew", "float"
        position = "right",     -- "right", "left", "top", "bottom" (for splits)
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
    "ClaudeCodeVsplit", 
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
        type = "current",       -- "current", "split", "vsplit", "tabnew", "float"
        position = "right",     -- "right", "left", "top", "bottom" (for splits)
        size = 80,             -- columns for vsplit, lines for split
      },
      auto_scroll = true,
      save_session = true,
      auto_save_session = true,    -- Auto-save on focus loss
      auto_save_notify = true,     -- Show notifications when auto-saving
      session_dir = vim.fn.stdpath("data") .. "/claude-code-sessions/",
      -- Default keybindings (can be customized)
      keybindings = {
        toggle = "<leader>clc",
        new_session = "<leader>cln", 
        send_selection = "<leader>cls",
        open_vsplit = "<leader>clv",
        save_session = "<leader>clS",
        update_session = "<leader>clu",
        browse_sessions = "<leader>clb",
        restore_session = "<leader>clr",
        new_with_selection = "<leader>clw",
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
| `:ClaudeCodeVsplit`          | claude: open new session in vsplit                |
| `:ClaudeCodeSend`            | claude: send selection                            |
| `:ClaudeCodeSaveSession`     | claude: save session                              |
| `:ClaudeCodeUpdateSession`   | claude: update session                            |
| `:ClaudeCodeSessions`        | claude: browse sessions                           |
| `:ClaudeCodeRestoreSession`  | claude: restore session                           |
| `:ClaudeCodeNewWithSelection`| claude: new with selection                        |
| `:ClaudeCodeSmartSearch`     | claude: semantic search sessions (with intelligence) |
| `:ClaudeCodeBuildContext`    | claude: build smart context (with intelligence)  |
| `:ClaudeCodeAnalytics`       | claude: show analytics dashboard (with intelligence) |
| `:ClaudeCodeProjectInsights` | claude: show project insights (with intelligence) |
| `:ClaudeCodeInstallCommands` | claude: install custom commands                   |
| `:ClaudeCodeInstallAgents`   | claude: install built-in agents                   |

### Default Keybindings

| Key           | Mode   | Action                        |
| ------------- | ------ | ----------------------------- |
| `<leader>clc` | Normal | claude: toggle window         |
| `<leader>cln` | Normal | claude: new session           |
| `<leader>cls` | Visual | claude: send selection        |
| `<leader>clv` | Normal | claude: new session in vsplit |
| `<leader>clS` | Normal | claude: save session          |
| `<leader>clu` | Normal | claude: update session        |
| `<leader>clb` | Normal | claude: browse sessions       |
| `<leader>clr` | Normal | claude: restore session       |
| `<leader>clw` | Visual | claude: new with selection    |
| `<leader>clf` | Normal | claude: smart search (intelligence) |
| `<leader>clC` | Normal | claude: build context (intelligence) |
| `<leader>cli` | Normal | claude: project insights (intelligence) |
| `<leader>cla` | Normal | claude: analytics (intelligence) |

### In Claude Code Buffer

| Key         | Mode     | Action                      |
| ----------- | -------- | --------------------------- |
| `q`         | Normal   | Close the window            |
| `<Esc>`     | Normal   | Close the window            |
| `<Esc>`     | Terminal | Send Esc to Claude (cancel action) |
| `<Esc><Esc>` | Terminal | Exit to normal mode         |
| `<C-[>`     | Terminal | Exit to normal mode         |
| `<C-n>`     | Terminal | Exit to normal mode         |
| `<C-q>`     | Terminal | Close the window            |
| `i`         | Normal   | Enter terminal (insert) mode|

**Smart Esc Handling**: Single `<Esc>` sends cancel to Claude, double `<Esc>` exits terminal mode.

### Custom Claude Commands (Opinionated Workflow)

The plugin provides optional custom commands that implement my personal development workflow. These commands reflect my "vibe coding" approach - focusing on planning first, then shipping quickly.

**⚠️ These are opinionated tools** - they follow specific patterns and conventions that work for my workflow. You may want to customize them or create your own commands based on your preferences.

To install these commands, run:

```vim
:ClaudeCodeInstallCommands
```

Available commands:

| Command | Description |
| ------- | ----------- |
| `/plan` | Interactive BDD specification builder for feature planning |
| `/code` | TDD-focused implementation agent that reads .ai specs |
| `/ship` | Streamlined git workflow: commit, push, PR, and release |

#### My Workflow Philosophy

- **Plan First**: Use `/plan` to think through features in BDD style before coding
- **Implement with TDD**: Use `/code` to implement features using Test-Driven Development
- **Ship Fast**: Use `/ship` to get changes live quickly with proper git workflow
- **Specifications in `.ai/`**: Keep planning docs separate from code
- **Incremental Development**: Build features in deployable slices
- **Test Coverage**: Every behavior should have a test

#### Command Details

These commands are created in `.claude/commands/` in your project root:
- `/plan` - Interactive planning tool that creates BDD-style feature specifications in `.ai/` directory
- `/code` - TDD implementation agent that reads `.ai/` specs and implements features incrementally  
- `/ship` - Git workflow automation for commit, push, PR creation, and releases

**Complete Development Cycle**: `/plan` → `/code` → `/ship` → repeat

Feel free to modify these commands in your project's `.claude/commands/` directory to match your own workflow preferences!

### Built-in Claude Agents

The plugin provides built-in agent templates that can be installed at either project or personal level:

To install these agents, run:

```vim
:ClaudeCodeInstallAgents
```

You'll be prompted to choose the installation location:
- **Project level** (`.claude/agents/`) - Available only for the current project
- **Personal level** (`~/.claude/agents/`) - Available across all your projects

Available agents:

| Agent | Color | Description |
| ----- | ----- | ----------- |
| `product-analyst` | Purple | Translates business requirements into clear technical specifications |
| `software-engineer` | Blue | Implements features using TDD with continuous code review, presents reasoning before changes, emphasizes readable code |
| `code-reviewer` | Purple | Performs thorough code reviews focusing on quality and security |

#### Agent Features

- **product-analyst**: Creates BDD-style specifications, gathers requirements interactively
- **software-engineer**: Reads .ai specs, implements with TDD, presents reasoning before changes, prioritizes readable/enjoyable code, reviews after each iteration
- **code-reviewer**: Analyzes code quality, security, performance, and provides actionable feedback

These agents work seamlessly with the custom commands workflow:
1. Use `/plan` or `product-analyst` to create specifications
2. Use `/code` or `software-engineer` to implement features
3. Use `code-reviewer` for thorough code reviews
4. Use `/ship` to deploy your changes

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

```lua
require("claude-code").setup({
  -- Command to run Claude Code CLI
  claude_code_cmd = "claude",

  -- Window configuration
  window = {
    type = "current",       -- "current", "split", "vsplit", "tabnew", "float"
    position = "right",     -- "right", "left", "top", "bottom" (for splits/float)
    size = 80,             -- for splits: lines/columns, for float: percentage (0.4)
  },

  -- Auto-scroll to bottom when new content appears
  auto_scroll = true,

  -- Save sessions to files
  save_session = true,
  auto_save_session = true,  -- Automatically save sessions on focus loss
  auto_save_notify = true,   -- Show notification when auto-saving sessions
  session_dir = vim.fn.stdpath("data") .. "/claude-code-sessions/",
  max_exchanges = 20,        -- Maximum exchanges to keep in saved sessions
  
  -- Custom Commands (Opinionated Workflow)
  setup_claude_commands = false, -- Don't automatically install custom commands (default: false)
                                  -- Use :ClaudeCodeInstallCommands to install /ship and /plan commands

  -- Default keybindings (set to false to disable, or change keys)
  keybindings = {
    toggle = "<leader>clc",           -- Toggle Claude Code window
    new_session = "<leader>cln",      -- Start new session
    send_selection = "<leader>cls",   -- Send selection to Claude (visual mode)
    open_vsplit = "<leader>clv",      -- Open Claude Code in vsplit
    save_session = "<leader>clS",     -- Save session with name
    update_session = "<leader>clu",   -- Update current session
    browse_sessions = "<leader>clb",  -- Browse sessions
    restore_session = "<leader>clr",  -- Restore session
    new_with_selection = "<leader>clw", -- New session with selection (visual mode)
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
    toggle = "<leader>aic",           -- Change to different key
    new_session = "<leader>ain",      -- Custom keybinding
    send_selection = false,           -- Disable this keybinding
    save_session = "<leader>aiS",     -- Use different key combination
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

## 🧠 Intelligent Session Management

### AI-Powered Session Compression

The plugin now includes a production-ready **Claude Code Intelligence service** that provides advanced AI features:

#### 🔥 Key Features:
- **70-80% Compression Ratio**: AI-powered compression using local LLM processing
- **Semantic Search**: Vector-based search across all your sessions
- **Smart Context Building**: Intelligently assembles context from multiple related sessions
- **Project Memory**: Consolidates insights, patterns, and decisions across your project
- **Pattern Recognition**: Identifies recurring solutions, errors, and workflows
- **Real-time Analytics**: Comprehensive dashboard with usage statistics

#### 🚀 Backend Service Architecture:
- **Local LLM Processing**: Ollama integration with automatic model management
- **Enterprise Security**: API key authentication, rate limiting, input validation
- **High Availability**: Circuit breakers, graceful degradation, fallback systems
- **Production Ready**: Monitoring, logging, automated backups, performance optimization

#### 📦 Quick Deployment:

```bash
# Option 1: Docker Compose (Recommended)
cd claude-code-intelligence/deployments/scripts
./deploy.sh -t docker-compose deploy

# Option 2: Kubernetes (Production)
./deploy.sh -t kubernetes -e prod deploy

# Option 3: Simple Docker
./deploy.sh -t docker deploy
```

#### 🔧 Plugin Configuration with Intelligence Service:

```lua
require("claude-code").setup({
  -- Enable intelligent features
  intelligence = {
    enabled = true,
    api_url = "http://localhost:8080/api",
    api_key = "your-api-key", -- Get from service startup logs
    features = {
      compression = true,
      semantic_search = true,
      context_building = true,
      analytics = true,
    }
  },
  -- Standard configuration
  claude_code_cmd = "claude",
  save_session = true,
  auto_save_session = true,
})
```

#### 📊 Advanced Features Available:

- **Session Analytics**: View compression stats, topic analysis, usage patterns
- **Smart Search**: `<leader>clf` - Find sessions by semantic similarity
- **Context Builder**: `<leader>clC` - Build context from multiple related sessions
- **Project Insights**: `<leader>cli` - View consolidated project knowledge
- **Pattern Analysis**: Identify recurring solutions and anti-patterns

### Basic Session Saving (Fallback)

Without the intelligence service, the plugin provides basic session saving:

#### Key Features:
- **Content Parsing**: Automatically identifies user prompts and Claude responses
- **Token Reduction**: Removes system messages, UI elements, and terminal formatting
- **Code Block Handling**: Condenses large code blocks to summaries, keeps small ones intact
- **Incremental Updates**: Only saves new content since last save, avoiding duplication
- **Configurable Limits**: Keep only recent exchanges (default: 20) to manage context and performance

#### Customizing Exchange Limits:
```lua
require("claude-code").setup({
  max_exchanges = 50,  -- For longer conversations
  max_exchanges = 10,  -- For faster performance
  max_exchanges = 100, -- For extensive project discussions
})
```

#### Saved Session Format:
```
=== Claude Code Session ===
Session: Project Architecture Discussion
Created: 2025-01-07 14:30:00
Updated: 2025-01-07 15:45:00
Exchanges: 12
========================

### Exchange 1 ###
Human: Can you help me design a plugin architecture?

Assistant: I'll help you design a flexible plugin architecture...
[Code block condensed: 45 lines]
The key principles are modularity and loose coupling...
```

### Which-key Integration

If you have [which-key.nvim](https://github.com/folke/which-key.nvim) installed, claude-code.nvim will automatically register a beautiful menu interface. Press `<leader>cl` to see all available Claude Code commands:

```
╭─────────────────────────────────────────╮
│ 󰚩 Claude Code                          │
├─────────────────────────────────────────┤
│ c → Toggle Claude    n → New Session    │
│ v → Open Vsplit      S → Save Session   │
│ u → Update Session   b → Browse Sessions│
│ r → Restore Session                     │
│                                         │
│ Visual Mode:                            │
│ s → Send Selection   w → New with Sel.  │
╰─────────────────────────────────────────╯
```

### Integration with Other Plugins

The plugin works well with:

- **which-key** - Automatic menu registration for all Claude commands
- **nvim-tree** - Use with vertical splits for a complete IDE layout
- **telescope** - Send search results to Claude for analysis
- **trouble** - Get Claude's help with diagnostics
- **gitsigns** - Send diffs to Claude for code review

## 📝 Examples

### Workflow Examples

#### Basic Workflows
1. **Quick Questions**: Press `<leader>clc` to toggle Claude in current window, ask quick questions
2. **Code Review**: Select code, press `<leader>cls`, ask Claude to review
3. **Parallel Sessions**: Use `<leader>clv` to open a separate Claude session in vsplit for different topics
4. **Session Management**: Use `<leader>clS` to save important conversations with smart token reduction
5. **Browse & Restore**: Use `<leader>clb` to browse saved sessions and load them into active Claude
6. **Smart Navigation**: Single `<Esc>` cancels Claude actions, double `<Esc><Esc>` for vim navigation

#### Intelligent Workflows (with Intelligence Service)
7. **Semantic Search**: Press `<leader>clf` to find related sessions by meaning, not just keywords
8. **Smart Context**: Press `<leader>clC` to build intelligent context from multiple related sessions
9. **Project Insights**: Press `<leader>cli` to view consolidated project knowledge and patterns
10. **Analytics Dashboard**: Press `<leader>cla` to view session analytics and compression stats
11. **Pattern Recognition**: Discover recurring solutions and anti-patterns across your project
12. **Knowledge Consolidation**: Get AI-generated summaries of decisions and insights over time

#### Development Workflows
13. **Debugging**: Send error logs to Claude for analysis with historical context
14. **Documentation**: Send functions to Claude to generate docs based on project patterns
15. **Refactoring**: Get Claude's suggestions with awareness of your project's architecture
16. **Learning**: Find sessions where you solved similar problems before

### Session Management Workflow

1. **Start a conversation** with Claude about your project
2. **Save the session** with `<leader>clS` - give it a descriptive name
3. **Continue the conversation** - Claude remembers the context
4. **Update the session** with `<leader>clu` - only new content is added
5. **Create variations** - when updating, choose to create a new version instead

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
- Check for conflicts with `:verbose map <leader>clc`

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

## 🚀 Production Deployment

For teams and power users, the Claude Code Intelligence service provides enterprise-grade features:

### Quick Start

```bash
# 1. Clone the repository
git clone https://github.com/carlos-rodrigo/claude-code.nvim
cd claude-code.nvim/claude-code-intelligence

# 2. Deploy with Docker Compose
cd deployments/scripts
./deploy.sh -t docker-compose deploy

# 3. Get API key from logs
docker-compose -f ../docker-compose/docker-compose.yml logs claude-code-intelligence | grep "admin API key"

# 4. Configure Neovim plugin
# Add the API key to your plugin configuration
```

### Production Features

- **🔒 Enterprise Security**: API key authentication, rate limiting, input validation
- **📊 Advanced Analytics**: Session analytics, compression stats, usage patterns
- **🔍 Semantic Search**: Vector-based search across all sessions
- **🧠 Smart Context**: AI-powered context building from multiple sessions
- **🏗️ High Availability**: Circuit breakers, graceful degradation, fallback systems
- **📈 Monitoring**: Prometheus metrics, Grafana dashboards, health checks
- **💾 Backup & Recovery**: Automated backups with integrity verification
- **🚢 Easy Deployment**: Docker, Kubernetes, Docker Compose support

### Documentation

- **[Production Deployment Guide](claude-code-intelligence/docs/production/README.md)** - Complete setup and configuration
- **[API Documentation](claude-code-intelligence/docs/production/API.md)** - REST API reference
- **[Architecture Overview](claude-code-intelligence/internal/)** - Technical implementation details

## 📚 Related Projects

- [Claude Code CLI](https://docs.anthropic.com) - The official Claude Code command-line tool
- [LazyVim](https://github.com/LazyVim/LazyVim) - The Neovim configuration this plugin is designed for
- [Ollama](https://ollama.com) - Local LLM runtime for the intelligence service
- [nvim-terminal.lua](https://github.com/s1n7ax/nvim-terminal) - Terminal integration inspiration

---

⭐ If you find this plugin useful, please give it a star on GitHub!
