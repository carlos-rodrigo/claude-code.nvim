-- which-key.lua - Which-key integration for claude-code.nvim
local M = {}

-- Check if which-key is available
local has_which_key, which_key = pcall(require, "which-key")

function M.setup(keybindings)
  if not has_which_key then
    -- which-key not installed, skip setup
    return
  end

  -- Register the Claude Code prefix
  which_key.register({
    ["<leader>cl"] = { name = "󰚩 Claude Code" },
  })

  -- Build mappings for which-key
  local mappings = {
    ["<leader>cl"] = {
      name = "󰚩 Claude Code",
      c = { "<cmd>ClaudeCodeToggle<cr>", "Toggle Claude" },
      n = { "<cmd>ClaudeCodeNew<cr>", "New Session" },
      v = { "<cmd>ClaudeCodeVsplit<cr>", "Open Vsplit" },
      S = { "<cmd>ClaudeCodeSaveSession<cr>", "Save Session" },
      u = { "<cmd>ClaudeCodeUpdateSession<cr>", "Update Session" },
      b = { "<cmd>ClaudeCodeSessions<cr>", "Browse Sessions" },
      r = { "<cmd>ClaudeCodeRestoreSession<cr>", "Restore Session" },
    },
  }

  -- Visual mode mappings
  local visual_mappings = {
    ["<leader>cl"] = {
      name = "󰚩 Claude Code",
      s = { "<cmd>ClaudeCodeSend<cr>", "Send Selection" },
      w = { "<cmd>ClaudeCodeNewWithSelection<cr>", "New with Selection" },
    },
  }

  -- Register normal mode mappings
  which_key.register(mappings, { mode = "n", buffer = nil, silent = true, noremap = true, nowait = true })
  
  -- Register visual mode mappings
  which_key.register(visual_mappings, { mode = "v", buffer = nil, silent = true, noremap = true, nowait = true })
end

return M