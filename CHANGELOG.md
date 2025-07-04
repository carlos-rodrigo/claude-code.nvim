# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.1.0] - 2025-07-04

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
