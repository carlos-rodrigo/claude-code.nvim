-- claude-code.nvim Notification Module
-- Provides consistent notification handling

local M = {}

-- Send info notification
function M.info(msg)
  vim.notify(msg, vim.log.levels.INFO, { title = "Claude Code" })
end

-- Send warning notification
function M.warn(msg)
  vim.notify(msg, vim.log.levels.WARN, { title = "Claude Code" })
end

-- Send error notification
function M.error(msg)
  vim.notify(msg, vim.log.levels.ERROR, { title = "Claude Code" })
end

-- Send success notification
function M.success(msg)
  vim.notify(msg, vim.log.levels.INFO, { title = "Claude Code" })
end

return M