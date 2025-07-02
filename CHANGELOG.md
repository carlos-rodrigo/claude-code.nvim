# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
