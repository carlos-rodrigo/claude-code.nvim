-- claude-code.nvim - Simplified terminal approach
-- A Neovim plugin for Claude Code integration
-- https://github.com/carlos-rodrigo/claude-code.nvim

local M = {}

-- Default configuration
local default_config = {
	claude_code_cmd = "claude",
	window = {
		type = "tabnew", -- "split", "vsplit", "tabnew"
	},
	auto_scroll = true,
	save_session = true,
	auto_save_session = true,
	auto_save_notify = true,
	session_dir = vim.fn.stdpath("data") .. "/claude-code-sessions/",
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
}

-- Plugin state
local state = {
	config = {},
	terminal_bufnr = nil,
}

-- Find Claude Code terminal buffer
local function find_claude_terminal()
	-- Search all buffers for our terminal
	local buffers = vim.api.nvim_list_bufs()
	for _, buf in ipairs(buffers) do
		if vim.api.nvim_buf_is_valid(buf) then
			local name = vim.api.nvim_buf_get_name(buf)
			if name:match("term://.*claude$") then
				-- Check if buffer is still alive
				local info = vim.fn.getbufinfo(buf)[1]
				if info and info.loaded == 1 then
					return buf
				end
			end
		end
	end
	return nil
end

-- Find window containing Claude terminal
local function find_claude_window()
	local buf = state.terminal_bufnr or find_claude_terminal()
	if not buf then return nil end
	
	-- Search all windows for this buffer
	local wins = vim.api.nvim_list_wins()
	for _, win in ipairs(wins) do
		if vim.api.nvim_win_get_buf(win) == buf then
			return win
		end
	end
	return nil
end

-- Open Claude Code terminal
function M.open()
	local cmd = state.config.claude_code_cmd
	
	-- Check if command exists
	if vim.fn.executable(cmd) == 0 then
		vim.notify("Command '" .. cmd .. "' not found. Please install Claude CLI.", vim.log.levels.ERROR)
		return
	end
	
	-- Check if we already have a terminal
	state.terminal_bufnr = find_claude_terminal()
	
	if state.terminal_bufnr then
		-- Terminal exists, find or create window for it
		local win = find_claude_window()
		if win then
			-- Window exists, just focus it
			vim.api.nvim_set_current_win(win)
		else
			-- Create new window for existing terminal
			local window_type = state.config.window.type
			if window_type == "tabnew" then
				-- Use tab split with the buffer to avoid creating empty buffer
				vim.cmd("tab sb " .. state.terminal_bufnr)
			elseif window_type == "vsplit" then
				vim.cmd("vsplit")
				vim.api.nvim_win_set_buf(0, state.terminal_bufnr)
			else
				vim.cmd("split")
				vim.api.nvim_win_set_buf(0, state.terminal_bufnr)
			end
		end
	else
		-- Create new terminal
		local window_type = state.config.window.type
		if window_type == "tabnew" then
			-- Create terminal directly in new tab to avoid empty buffer
			vim.cmd("tabnew | terminal " .. cmd)
		elseif window_type == "vsplit" then
			vim.cmd("vsplit | terminal " .. cmd)
		else
			vim.cmd("split | terminal " .. cmd)
		end
		
		state.terminal_bufnr = vim.api.nvim_get_current_buf()
		
		-- Set up buffer-local keymaps
		local buf = state.terminal_bufnr
		vim.api.nvim_buf_set_keymap(buf, 't', '<Esc>', '<C-\\><C-n>', { noremap = true })
		vim.api.nvim_buf_set_keymap(buf, 'n', 'q', ':q<CR>', { noremap = true })
		
		-- Disable various plugins for this buffer
		vim.schedule(function()
			if vim.api.nvim_buf_is_valid(buf) then
				vim.b[buf].ts_highlight = false
				vim.b[buf].render_markdown = false
			end
		end)
	end
	
	-- Enter insert mode
	vim.cmd("startinsert")
end

-- Close Claude Code window (not the terminal)
function M.close()
	local win = find_claude_window()
	if win then
		-- Check if it's the only window
		local wins = vim.api.nvim_list_wins()
		local tabs = vim.api.nvim_list_tabpages()
		
		if #wins == 1 and #tabs == 1 then
			-- Last window in last tab, can't close
			vim.notify("Claude Code hidden. Use toggle to bring it back.", vim.log.levels.INFO)
			vim.cmd("enew")
		else
			vim.api.nvim_win_close(win, false)
		end
	end
end

-- Toggle Claude Code
function M.toggle()
	local win = find_claude_window()
	if win then
		M.close()
	else
		M.open()
	end
end

-- Kill terminal and start fresh
function M.new_session()
	-- Find and kill existing terminal
	local buf = state.terminal_bufnr or find_claude_terminal()
	if buf and vim.api.nvim_buf_is_valid(buf) then
		vim.api.nvim_buf_delete(buf, { force = true })
	end
	
	state.terminal_bufnr = nil
	M.open()
end

-- Send selection to Claude Code
function M.send_selection(start_line, end_line)
	local buf = state.terminal_bufnr or find_claude_terminal()
	if not buf then
		vim.notify("Claude Code not running. Start it first with :ClaudeCode", vim.log.levels.WARN)
		return
	end
	
	-- Get selection
	local current_buf = vim.api.nvim_get_current_buf()
	local lines = vim.api.nvim_buf_get_lines(current_buf, start_line - 1, end_line, false)
	local text = table.concat(lines, "\n")
	
	-- Find or create window for terminal
	local win = find_claude_window()
	if not win then
		M.open()
	else
		vim.api.nvim_set_current_win(win)
	end
	
	-- Send text to terminal
	vim.fn.chansend(vim.b[buf].terminal_job_id, text .. "\n")
end

-- Setup function
function M.setup(opts)
	state.config = vim.tbl_deep_extend("force", default_config, opts or {})
	
	-- Create user commands
	vim.api.nvim_create_user_command("ClaudeCode", M.open, { desc = "Open Claude Code" })
	vim.api.nvim_create_user_command("ClaudeCodeToggle", M.toggle, { desc = "Toggle Claude Code" })
	vim.api.nvim_create_user_command("ClaudeCodeNew", M.new_session, { desc = "New Claude Code session" })
	vim.api.nvim_create_user_command("ClaudeCodeSend", function(cmd_opts)
		M.send_selection(cmd_opts.line1, cmd_opts.line2)
	end, { desc = "Send selection to Claude Code", range = true })
	
	-- Set up keybindings
	local keys = state.config.keybindings
	if keys and keys ~= false then
		if keys.toggle then
			vim.keymap.set("n", keys.toggle, "<cmd>ClaudeCodeToggle<cr>", { desc = "Toggle Claude Code" })
		end
		if keys.new_session then
			vim.keymap.set("n", keys.new_session, "<cmd>ClaudeCodeNew<cr>", { desc = "New Claude Code session" })
		end
		if keys.send_selection then
			vim.keymap.set("v", keys.send_selection, "<cmd>ClaudeCodeSend<cr>", { desc = "Send selection to Claude Code" })
		end
	end
end

return M