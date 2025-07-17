# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.7.0] - 2025-07-13

### Added

- **Custom Claude Commands**: Automatic installation of `/ship` and `/plan` commands
- **BDD Planning Command**: `/plan` command for creating behavior-driven development specifications
- **Ship Command**: `/ship` command for streamlined git workflow (formerly push-to-prod)
- **Project-level Commands**: Commands are created in `.claude/commands/` directory in project root
- **Command Templates**: Commands stored as template files in plugin for easier management
- **Manual Setup Command**: `:ClaudeCodeSetupCommands` for manual command installation

### Enhanced

- **Automated Setup**: Commands are automatically installed when plugin is loaded or updated
- **Git Integration**: Ship command supports commits, PRs, and releases in one workflow
- **Planning Workflow**: Interactive BDD specification creation with structured output to `.ai/` directory
- **Configuration Option**: `setup_claude_commands` config option to control automatic setup
- **Documentation**: Comprehensive documentation of custom commands feature

### Changed

- **Renamed Command**: `push-to-prod` renamed to `ship` for better clarity

### Commands Added

- `:ClaudeCodeSetupCommands` - Manually setup Claude custom commands

### Custom Commands Available

- `/plan` - Interactive BDD specification builder for feature planning
- `/ship` - Streamlined git workflow: commit, push, PR, and release

## [1.6.0] - 2025-07-06

### Added

- **Session Browsing**: Full implementation of session browsing with `<leader>clb`
- **Session Loading**: Load saved sessions into active Claude sessions for context restoration
- **Smart Esc Handling**: Intelligent Esc key behavior that preserves Claude functionality
- **Multiple Exit Options**: `<C-[>`, `<C-n>`, and double `<Esc><Esc>` to exit terminal mode

### Enhanced

- **Session Viewing**: Browse sessions with formatted display and markdown syntax highlighting
- **Context Restoration**: Use `/resume` command to restore session context in Claude
- **Key Conflict Resolution**: Solved Esc key conflict between Claude (cancel) and Neovim (exit terminal)
- **Documentation**: Comprehensive documentation of all key bindings and behaviors

### Fixed

- **Session Browsing**: Removed "not yet implemented" message and added full functionality
- **Terminal Navigation**: Resolved conflict between Claude's Esc usage and vim navigation needs

### User Experience

- **Intuitive Navigation**: Single Esc for Claude actions, double Esc for vim navigation
- **Session Management**: Complete workflow from save → browse → view → load sessions
- **Flexible Controls**: Multiple ways to exit terminal mode based on user preference

## [1.5.0] - 2025-07-06

### Added

- **Smart Session Saving**: Intelligent content parsing that reduces token usage by 70-80%
- **Incremental Saving**: Updates existing sessions with only new content to avoid duplication
- **Content Classification**: Automatically identifies user prompts, Claude responses, and system messages
- **Token Reduction**: Filters out UI elements, terminal formatting, and condenses large code blocks
- **Configurable Exchange Limits**: Control session length with `max_exchanges` parameter (default: 20)

### Enhanced

- **Session Management**: Sessions now save only essential conversation content
- **File Format**: Clean, structured session files with metadata and organized exchanges
- **User Experience**: Smart prompting for updating existing sessions vs creating new ones

### Fixed

- **Session Bloat**: Eliminated saving of redundant terminal content and formatting
- **Token Efficiency**: Dramatically reduced file sizes while maintaining conversation context

### Technical Improvements

- **Content Parser**: Advanced pattern matching for different prompt and response formats
- **Exchange Tracking**: Proper grouping of user prompts with corresponding responses
- **Metadata Management**: Session tracking with creation time, updates, and exchange counts

## [1.4.0] - 2025-07-06

### Added

- **Current Window Mode**: New default behavior uses current window instead of creating tabs
- **Which-key Integration**: Automatic menu interface when pressing `<leader>cl`
- **Independent Vsplit Sessions**: Vsplit creates completely separate Claude sessions
- **Organized Keybindings**: All commands now use `<leader>cl` prefix for better organization

### Enhanced

- **Window Management**: Added "current" window type as new default
- **Session Independence**: Vsplit sessions no longer mirror the main session
- **User Experience**: Which-key popup shows all available Claude commands

### Changed

- **Default Window Type**: Changed from "tabnew" to "current" for cleaner workflow
- **Keybinding Structure**: Reorganized all keybindings under `<leader>cl` prefix

### Fixed

- **Session Mirroring**: Fixed vsplit sessions sharing the same conversation as main session

## [1.3.0] - 2025-07-04

### Added

- **Named Session Management**: Save sessions with custom names using `:ClaudeCodeSaveSession`
- **Session Browsing**: Browse and view previous sessions with `:ClaudeCodeSessions`
- **Session Updates**: Update current named session with `:ClaudeCodeUpdateSession`
- **Start with Selection**: Create new sessions with selected text as initial prompt using `:ClaudeCodeNewWithSelection`
- **Smart Session Handling**: Named sessions are preserved and updated on exit instead of creating duplicates
- **Terminal Mode Navigation**: Press `<Esc>` in terminal mode to exit to normal mode for vim navigation

### Enhanced

- **Session Management**: Improved session workflow with options to update existing or create new sessions
- **User Experience**: Added interactive prompts for session management operations
- **Keybindings**: Added new keybindings for session management features
- **Session Persistence**: Sessions now persist when switching tabs/buffers with proper buffer management

### Fixed

- **Buffer Management**: Fixed empty `[No Name]` buffers being created when toggling Claude Code
- **Window Closing**: Fixed "Cannot close last window" error when closing Claude Code
- **State Initialization**: Fixed "attempt to index global 'state' (a nil value)" error on startup
- **Treesitter Errors**: Disabled treesitter and render-markdown for terminal buffers to prevent decoration errors

### Technical Improvements

- **Simplified Architecture**: Rewrote core functionality using Neovim's built-in `:terminal` command
- **Better Buffer Tracking**: Terminal buffers are now tracked by name pattern (`term://.*claude$`)
- **Cleaner Window Management**: Uses `tab sb` and chained commands to avoid creating empty buffers

### Commands Added

- `:ClaudeCodeSaveSession` - Save current session with custom name
- `:ClaudeCodeUpdateSession` - Update the current named session
- `:ClaudeCodeSessions` - Browse and view previous Claude Code sessions
- `:ClaudeCodeNewWithSelection` - Start new session with selected text as prompt

### Keybindings Added

- `<leader>cS` - Save Claude Code session
- `<leader>cu` - Update current session
- `<leader>cb` - Browse Claude Code sessions
- `<leader>cw` - New session with selection (visual mode)
- `<Esc>` - Exit terminal mode to normal mode (in Claude Code buffer)

## [1.0.0] - 2025-07-02

### Added

- Initial release
- Core functionality for Claude Code integration
- Buffer-based interface with multiple window types
- Session management
- Visual selection sending
- LazyVim plugin specification
- Comprehensive documentation

### Features

- **Window Types**: Support for splits, vertical splits, tabs, and floating windows
- **Session Management**: Automatic session saving and restoration
- **Visual Integration**: Send selected code directly to Claude
- **Configurable**: Flexible window positioning and sizing options
- **LazyVim Ready**: Follows LazyVim conventions for easy integration

### Commands

- `:ClaudeCode` - Open Claude Code buffer
- `:ClaudeCodeNew` - Start new session
- `:ClaudeCodeToggle` - Toggle window visibility
- `:ClaudeCodeSend` - Send selection to Claude

### Keybindings

- `<leader>cc` - Toggle Claude Code
- `<leader>cn` - New session
- `<leader>cs` - Send selection (visual mode)
