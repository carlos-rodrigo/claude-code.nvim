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
    require("claude-code").setup()
  end,
}