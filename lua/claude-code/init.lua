-- claude-code.nvim
-- A Neovim plugin for Claude Code integration
-- https://github.com/your-username/claude-code.nvim

local M = {}

-- Default configuration
local default_config = {
	claude_code_cmd = "claude-code",
	window = {
		type = "vsplit", -- "split", "vsplit", "tabnew", "float"
		position = "right", -- for splits: "right", "left", "top", "bottom"
		size = 80, -- for splits: number of lines/columns, for float: percentage (0.4)
	},
	auto_scroll = true,
	save_session = true,
	session_dir = vim.fn.stdpath("data") .. "/claude-code-sessions/",
}

-- Plugin state
local state = {
	bufnr = nil,
	winnr = nil,
	term_job_id = nil,
	config = {},
	session_file = nil,
}

-- Utility functions
local function create_session_dir()
	local session_dir = state.config.session_dir
	if vim.fn.isdirectory(session_dir) == 0 then
		vim.fn.mkdir(session_dir, "p")
	end
end

local function get_session_file()
	if not state.session_file then
		create_session_dir()
		local timestamp = os.date("%Y%m%d_%H%M%S")
		state.session_file = state.config.session_dir .. "session_" .. timestamp .. ".txt"
	end
	return state.session_file
end

local function get_window_config()
	local config = state.config.window

	if config.type == "float" then
		local ui = vim.api.nvim_list_uis()[1]
		local size = config.size <= 1 and config.size or 0.4 -- Ensure float size is percentage

		if config.position == "right" then
			return {
				type = "float",
				relative = "editor",
				width = math.floor(ui.width * size),
				height = ui.height - 2,
				col = ui.width - math.floor(ui.width * size),
				row = 0,
				border = "single",
				title = " Claude Code ",
				title_pos = "center",
			}
		elseif config.position == "left" then
			return {
				type = "float",
				relative = "editor",
				width = math.floor(ui.width * size),
				height = ui.height - 2,
				col = 0,
				row = 0,
				border = "single",
				title = " Claude Code ",
				title_pos = "center",
			}
		elseif config.position == "top" then
			return {
				type = "float",
				relative = "editor",
				width = ui.width,
				height = math.floor(ui.height * size),
				col = 0,
				row = 0,
				border = "single",
				title = " Claude Code ",
				title_pos = "center",
			}
		else -- bottom
			return {
				type = "float",
				relative = "editor",
				width = ui.width,
				height = math.floor(ui.height * size),
				col = 0,
				row = ui.height - math.floor(ui.height * size) - 2,
				border = "single",
				title = " Claude Code ",
				title_pos = "center",
			}
		end
	else
		-- Return configuration for splits
		return {
			type = config.type,
			position = config.position,
			size = config.size,
		}
	end
end

-- Create Claude Code buffer and window
local function create_claude_buffer()
	-- Create buffer
	state.bufnr = vim.api.nvim_create_buf(false, true)

	-- Set buffer options
	vim.api.nvim_buf_set_option(state.bufnr, "buftype", "terminal")
	vim.api.nvim_buf_set_option(state.bufnr, "swapfile", false)
	vim.api.nvim_buf_set_name(state.bufnr, "Claude Code")

	-- Create window based on configuration
	local win_config = get_window_config()

	if win_config.type == "float" then
		-- Create floating window
		state.winnr = vim.api.nvim_open_win(state.bufnr, true, win_config)
		vim.api.nvim_win_set_option(state.winnr, "winhl", "Normal:Normal,FloatBorder:FloatBorder")
	elseif win_config.type == "tabnew" then
		-- Create new tab
		vim.cmd("tabnew")
		state.winnr = vim.api.nvim_get_current_win()
		vim.api.nvim_win_set_buf(state.winnr, state.bufnr)
	else
		-- Create split window
		local split_cmd = ""

		if win_config.type == "vsplit" then
			if win_config.position == "left" then
				split_cmd = "topleft " .. win_config.size .. "vsplit"
			else -- right or default
				split_cmd = "botright " .. win_config.size .. "vsplit"
			end
		else -- horizontal split
			if win_config.position == "top" then
				split_cmd = "topleft " .. win_config.size .. "split"
			else -- bottom or default
				split_cmd = "botright " .. win_config.size .. "split"
			end
		end

		vim.cmd(split_cmd)
		state.winnr = vim.api.nvim_get_current_win()
		vim.api.nvim_win_set_buf(state.winnr, state.bufnr)
	end

	-- Start Claude Code in terminal
	local cmd = state.config.claude_code_cmd
	state.term_job_id = vim.fn.termopen(cmd, {
		on_exit = function(job_id, exit_code, event_type)
			if state.config.save_session then
				-- Save session when Claude Code exits
				local lines = vim.api.nvim_buf_get_lines(state.bufnr, 0, -1, false)
				vim.fn.writefile(lines, get_session_file())
			end
			state.term_job_id = nil
		end,
	})

	-- Set up buffer keymaps
	local opts = { buffer = state.bufnr, silent = true }
	vim.keymap.set("n", "q", function()
		M.close()
	end, opts)
	vim.keymap.set("n", "<Esc>", function()
		M.close()
	end, opts)
	vim.keymap.set("t", "<C-q>", function()
		M.close()
	end, opts)

	-- Auto-scroll to bottom if enabled
	if state.config.auto_scroll then
		vim.api.nvim_create_autocmd("TermResponse", {
			buffer = state.bufnr,
			callback = function()
				if vim.api.nvim_win_is_valid(state.winnr) then
					vim.api.nvim_win_set_cursor(state.winnr, { vim.api.nvim_buf_line_count(state.bufnr), 0 })
				end
			end,
		})
	end

	-- Enter terminal mode
	vim.cmd("startinsert")
end

-- Public functions
function M.setup(opts)
	state.config = vim.tbl_deep_extend("force", default_config, opts or {})

	-- Create user commands
	vim.api.nvim_create_user_command("ClaudeCode", function()
		M.open()
	end, { desc = "Open Claude Code" })

	vim.api.nvim_create_user_command("ClaudeCodeNew", function()
		M.new_session()
	end, { desc = "Start new Claude Code session" })

	vim.api.nvim_create_user_command("ClaudeCodeToggle", function()
		M.toggle()
	end, { desc = "Toggle Claude Code window" })

	vim.api.nvim_create_user_command("ClaudeCodeSend", function(opts)
		M.send_selection(opts.line1, opts.line2)
	end, { desc = "Send selection to Claude Code", range = true })
end

function M.open()
	if state.bufnr and vim.api.nvim_buf_is_valid(state.bufnr) then
		if state.winnr and vim.api.nvim_win_is_valid(state.winnr) then
			vim.api.nvim_set_current_win(state.winnr)
		else
			-- Recreate window for existing buffer
			local win_config = get_window_config()

			if win_config.type == "float" then
				state.winnr = vim.api.nvim_open_win(state.bufnr, true, win_config)
				vim.api.nvim_win_set_option(state.winnr, "winhl", "Normal:Normal,FloatBorder:FloatBorder")
			elseif win_config.type == "tabnew" then
				vim.cmd("tabnew")
				state.winnr = vim.api.nvim_get_current_win()
				vim.api.nvim_win_set_buf(state.winnr, state.bufnr)
			else
				-- Create split for existing buffer
				local split_cmd = ""

				if win_config.type == "vsplit" then
					if win_config.position == "left" then
						split_cmd = "topleft " .. win_config.size .. "vsplit"
					else
						split_cmd = "botright " .. win_config.size .. "vsplit"
					end
				else
					if win_config.position == "top" then
						split_cmd = "topleft " .. win_config.size .. "split"
					else
						split_cmd = "botright " .. win_config.size .. "split"
					end
				end

				vim.cmd(split_cmd)
				state.winnr = vim.api.nvim_get_current_win()
				vim.api.nvim_win_set_buf(state.winnr, state.bufnr)
			end
		end
	else
		create_claude_buffer()
	end
end

function M.close()
	if state.winnr and vim.api.nvim_win_is_valid(state.winnr) then
		local win_config = get_window_config()

		if win_config.type == "tabnew" then
			-- Close the tab if it only contains Claude Code
			local tab_wins = vim.api.nvim_tabpage_list_wins(0)
			if #tab_wins == 1 and tab_wins[1] == state.winnr then
				vim.cmd("tabclose")
			else
				vim.api.nvim_win_close(state.winnr, true)
			end
		else
			vim.api.nvim_win_close(state.winnr, true)
		end
		state.winnr = nil
	end
end

function M.toggle()
	if state.winnr and vim.api.nvim_win_is_valid(state.winnr) then
		M.close()
	else
		M.open()
	end
end

function M.new_session()
	-- Close current session
	if state.term_job_id then
		vim.fn.jobstop(state.term_job_id)
	end
	if state.bufnr and vim.api.nvim_buf_is_valid(state.bufnr) then
		vim.api.nvim_buf_delete(state.bufnr, { force = true })
	end

	-- Reset state
	state.bufnr = nil
	state.winnr = nil
	state.term_job_id = nil
	state.session_file = nil

	-- Create new session
	M.open()
end

function M.send_selection(start_line, end_line)
	if not state.bufnr or not vim.api.nvim_buf_is_valid(state.bufnr) then
		vim.notify("Claude Code not running. Start it first with :ClaudeCode", vim.log.levels.WARN)
		return
	end

	-- Get current buffer and selection
	local current_buf = vim.api.nvim_get_current_buf()
	local lines = vim.api.nvim_buf_get_lines(current_buf, start_line - 1, end_line, false)
	local text = table.concat(lines, "\n")

	-- Send to Claude Code
	if state.term_job_id then
		vim.fn.chansend(state.term_job_id, text .. "\n")
	end

	-- Focus Claude Code window
	if state.winnr and vim.api.nvim_win_is_valid(state.winnr) then
		vim.api.nvim_set_current_win(state.winnr)
	end
end

function M.send_text(text)
	if not state.bufnr or not vim.api.nvim_buf_is_valid(state.bufnr) then
		vim.notify("Claude Code not running. Start it first with :ClaudeCode", vim.log.levels.WARN)
		return
	end

	if state.term_job_id then
		vim.fn.chansend(state.term_job_id, text .. "\n")
	end
end

return M
