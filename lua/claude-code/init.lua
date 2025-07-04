-- claude-code.nvim
-- A Neovim plugin for Claude Code integration
-- https://github.com/carlos-rodrigo/claude-code.nvim

local M = {}

-- Helper function to find existing Claude Code tab
local function find_claude_tab()
	local tabs = vim.api.nvim_list_tabpages()
	for _, tab in ipairs(tabs) do
		local wins = vim.api.nvim_tabpage_list_wins(tab)
		for _, win in ipairs(wins) do
			local buf = vim.api.nvim_win_get_buf(win)
			local buf_name = vim.api.nvim_buf_get_name(buf)
			if buf_name:match("claude%-code") or (state.bufnr and buf == state.bufnr) then
				return tab, win
			end
		end
	end
	return nil, nil
end

-- Default configuration
local default_config = {
	claude_code_cmd = "claude",
	window = {
		type = "buffer", -- "split", "vsplit", "tabnew", "buffer", "newbuffer", "float"
	},
	auto_scroll = true,
	save_session = true,
	auto_save_session = true, -- Automatically save sessions on focus loss
	auto_save_notify = true, -- Show notification when auto-saving sessions
	session_dir = vim.fn.stdpath("data") .. "/claude-code-sessions/",
	-- Default keybindings
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
	bufnr = nil,
	winnr = nil,
	term_job_id = nil,
	config = {},
	session_file = nil,
	named_session = false, -- Track if user saved with custom name
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

-- Auto-save session to file
local function auto_save_session()
	if state.config.save_session and state.config.auto_save_session and state.bufnr and vim.api.nvim_buf_is_valid(state.bufnr) then
		local lines = vim.api.nvim_buf_get_lines(state.bufnr, 0, -1, false)
		local session_file
		if not state.named_session then
			session_file = get_session_file()
			vim.fn.writefile(lines, session_file)
		else
			session_file = state.session_file
			vim.fn.writefile(lines, session_file)
		end
		
		-- Show a brief notification if enabled
		if state.config.auto_save_notify then
			local filename = vim.fn.fnamemodify(session_file, ":t")
			vim.notify("Session auto-saved: " .. filename, vim.log.levels.INFO, { timeout = 2000 })
		end
	end
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
	-- Check if command exists first
	local cmd = state.config.claude_code_cmd
	if vim.fn.executable(cmd) == 0 then
		vim.notify(
			"Command '" .. cmd .. "' not found. Please install Claude CLI and ensure it's in your PATH.",
			vim.log.levels.ERROR
		)
		return
	end

	-- Create window first, then buffer
	local win_config = get_window_config()

	if win_config.type == "float" then
		-- Create buffer for floating window
		state.bufnr = vim.api.nvim_create_buf(false, true)
		state.winnr = vim.api.nvim_open_win(state.bufnr, true, win_config)
		vim.wo[state.winnr].winhl = "Normal:Normal,FloatBorder:FloatBorder"
	elseif win_config.type == "tabnew" then
		-- Check if Claude Code tab already exists
		local existing_tab, existing_win = find_claude_tab()
		if existing_tab then
			-- Switch to existing tab
			vim.api.nvim_set_current_tabpage(existing_tab)
			vim.api.nvim_set_current_win(existing_win)
			state.winnr = existing_win
			state.bufnr = vim.api.nvim_win_get_buf(existing_win)
		else
			-- Create new tab and buffer
			vim.cmd("tabnew")
			state.winnr = vim.api.nvim_get_current_win()
			state.bufnr = vim.api.nvim_create_buf(false, true)
			vim.api.nvim_win_set_buf(state.winnr, state.bufnr)
		end
	elseif win_config.type == "buffer" then
		-- Create new buffer and switch to it (like :enew)
		state.winnr = vim.api.nvim_get_current_win()
		state.bufnr = vim.api.nvim_create_buf(true, false) -- listed=true, scratch=false
		vim.api.nvim_win_set_buf(state.winnr, state.bufnr)
	elseif win_config.type == "newbuffer" then
		-- Create new buffer using bufadd
		state.bufnr = vim.fn.bufadd("")
		state.winnr = vim.api.nvim_get_current_win()
		vim.api.nvim_win_set_buf(state.winnr, state.bufnr)
	else
		-- Create split window first
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

		-- Create buffer and set it in the current window
		state.bufnr = vim.api.nvim_create_buf(false, true)
		vim.api.nvim_win_set_buf(state.winnr, state.bufnr)
	end

	-- Set buffer name
	vim.api.nvim_buf_set_name(state.bufnr, "Claude Code")

	-- Set buffer options after the buffer is current in a window
	vim.bo[state.bufnr].swapfile = false
	-- Only wipe buffer on hide for temporary window types
	if win_config.type ~= "buffer" and win_config.type ~= "newbuffer" then
		vim.bo[state.bufnr].bufhidden = "wipe"
	end

	-- Start Claude Code in terminal (this will automatically set buftype)
	state.term_job_id = vim.fn.termopen(cmd, {
		on_exit = function(job_id, exit_code, event_type)
			if state.config.save_session and vim.api.nvim_buf_is_valid(state.bufnr) then
				-- Only save if not already saved with a custom name
				if not state.named_session then
					local lines = vim.api.nvim_buf_get_lines(state.bufnr, 0, -1, false)
					vim.fn.writefile(lines, get_session_file())
				else
					-- Update the existing named session file
					local lines = vim.api.nvim_buf_get_lines(state.bufnr, 0, -1, false)
					vim.fn.writefile(lines, state.session_file)
				end
			end
			state.term_job_id = nil
		end,
	})

	-- Check if termopen was successful
	if state.term_job_id == -1 then
		vim.notify(
			"Failed to start " .. cmd .. ". Please check if the command is correct and executable.",
			vim.log.levels.ERROR
		)
		return
	end

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

	-- Auto-save session on focus loss
	if state.config.save_session and state.config.auto_save_session then
		vim.api.nvim_create_autocmd({"FocusLost", "BufLeave"}, {
			buffer = state.bufnr,
			callback = auto_save_session,
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
	end, { desc = "claude: open claude code" })

	vim.api.nvim_create_user_command("ClaudeCodeNew", function()
		M.new_session()
	end, { desc = "claude: start new session" })

	vim.api.nvim_create_user_command("ClaudeCodeToggle", function()
		M.toggle()
	end, { desc = "claude: toggle window" })

	vim.api.nvim_create_user_command("ClaudeCodeSend", function(opts)
		M.send_selection(opts.line1, opts.line2)
	end, { desc = "claude: send selection", range = true })

	vim.api.nvim_create_user_command("ClaudeCodeNewWithSelection", function(opts)
		M.new_session_with_selection(opts.line1, opts.line2)
	end, { desc = "claude: new session with selection", range = true })

	vim.api.nvim_create_user_command("ClaudeCodeSessions", function()
		M.browse_sessions()
	end, { desc = "claude: browse sessions" })

	vim.api.nvim_create_user_command("ClaudeCodeSaveSession", function()
		M.save_session_interactive()
	end, { desc = "claude: save session with name" })

	vim.api.nvim_create_user_command("ClaudeCodeUpdateSession", function()
		M.update_current_session()
	end, { desc = "claude: update current session" })

	vim.api.nvim_create_user_command("ClaudeCodeRestoreSession", function()
		M.restore_session_interactive()
	end, { desc = "claude: restore session from file" })

	-- Set up keybindings (only if not disabled)
	local keys = state.config.keybindings
	if keys ~= false then
		if keys.toggle then
			vim.keymap.set("n", keys.toggle, "<cmd>ClaudeCodeToggle<cr>", { desc = "claude: toggle" })
		end
		if keys.new_session then
			vim.keymap.set("n", keys.new_session, "<cmd>ClaudeCodeNew<cr>", { desc = "claude: new session" })
		end
		if keys.send_selection then
			vim.keymap.set("v", keys.send_selection, "<cmd>ClaudeCodeSend<cr>", { desc = "claude: send selection" })
		end
		if keys.save_session then
			vim.keymap.set("n", keys.save_session, "<cmd>ClaudeCodeSaveSession<cr>", { desc = "claude: save session" })
		end
		if keys.update_session then
			vim.keymap.set("n", keys.update_session, "<cmd>ClaudeCodeUpdateSession<cr>", { desc = "claude: update session" })
		end
		if keys.browse_sessions then
			vim.keymap.set("n", keys.browse_sessions, "<cmd>ClaudeCodeSessions<cr>", { desc = "claude: browse sessions" })
		end
		if keys.restore_session then
			vim.keymap.set("n", keys.restore_session, "<cmd>ClaudeCodeRestoreSession<cr>", { desc = "claude: restore session" })
		end
		if keys.new_with_selection then
			vim.keymap.set("v", keys.new_with_selection, "<cmd>ClaudeCodeNewWithSelection<cr>", { desc = "claude: new with selection" })
		end
	end
end

function M.open()
	-- Always create a new session file on each open
	state.session_file = nil
	state.named_session = false
	
	if state.bufnr and vim.api.nvim_buf_is_valid(state.bufnr) then
		if state.winnr and vim.api.nvim_win_is_valid(state.winnr) then
			vim.api.nvim_set_current_win(state.winnr)
		else
			-- Recreate window for existing buffer
			local win_config = get_window_config()

			if win_config.type == "float" then
				state.winnr = vim.api.nvim_open_win(state.bufnr, true, win_config)
				vim.wo[state.winnr].winhl = "Normal:Normal,FloatBorder:FloatBorder"
			elseif win_config.type == "tabnew" then
				-- Check if Claude Code tab already exists
				local existing_tab, existing_win = find_claude_tab()
				if existing_tab then
					-- Switch to existing tab
					vim.api.nvim_set_current_tabpage(existing_tab)
					vim.api.nvim_set_current_win(existing_win)
					state.winnr = existing_win
				else
					-- Create new tab
					vim.cmd("tabnew")
					state.winnr = vim.api.nvim_get_current_win()
					vim.api.nvim_win_set_buf(state.winnr, state.bufnr)
				end
			elseif win_config.type == "buffer" then
				-- Switch to existing buffer in current window
				state.winnr = vim.api.nvim_get_current_win()
				vim.api.nvim_win_set_buf(state.winnr, state.bufnr)
			elseif win_config.type == "newbuffer" then
				-- Switch to existing buffer in current window
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
	state.named_session = false

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

function M.new_session_with_selection(start_line, end_line)
	-- Get current buffer and selection
	local current_buf = vim.api.nvim_get_current_buf()
	local lines = vim.api.nvim_buf_get_lines(current_buf, start_line - 1, end_line, false)
	local text = table.concat(lines, "\n")
	
	-- Start new session
	M.new_session()
	
	-- Wait a bit for the terminal to be ready
	vim.defer_fn(function()
		if state.term_job_id then
			vim.fn.chansend(state.term_job_id, text .. "\n")
		end
	end, 100)
end

function M.list_sessions()
	create_session_dir()
	local session_files = vim.fn.glob(state.config.session_dir .. "session_*.txt", false, true)
	
	-- Sort sessions by date (newest first)
	table.sort(session_files, function(a, b) return a > b end)
	
	local sessions = {}
	for _, file in ipairs(session_files) do
		local basename = vim.fn.fnamemodify(file, ":t")
		local timestamp, custom_name = basename:match("session_(%d+_%d+)_?(.*)%.txt")
		if timestamp then
			-- Format timestamp for display
			local year, month, day, hour, min, sec = timestamp:match("(%d%d%d%d)(%d%d)(%d%d)_(%d%d)(%d%d)(%d%d)")
			local display_name = string.format("%s-%s-%s %s:%s:%s", year, month, day, hour, min, sec)
			
			-- Add custom name if present
			if custom_name and custom_name ~= "" then
				display_name = display_name .. " - " .. custom_name:gsub("_", " ")
			end
			
			table.insert(sessions, {
				file = file,
				timestamp = timestamp,
				display_name = display_name
			})
		end
	end
	
	return sessions
end

function M.load_session(session_file)
	if not vim.fn.filereadable(session_file) then
		vim.notify("Session file not found: " .. session_file, vim.log.levels.ERROR)
		return
	end
	
	-- Create a new buffer for viewing the session
	local buf = vim.api.nvim_create_buf(false, true)
	vim.api.nvim_buf_set_name(buf, "Claude Code Session: " .. vim.fn.fnamemodify(session_file, ":t"))
	
	-- Read session content
	local lines = vim.fn.readfile(session_file)
	vim.api.nvim_buf_set_lines(buf, 0, -1, false, lines)
	
	-- Set buffer options
	vim.bo[buf].modifiable = false
	vim.bo[buf].buftype = "nofile"
	vim.bo[buf].swapfile = false
	
	-- Open in a new window
	local win_config = get_window_config()
	if win_config.type == "float" then
		vim.api.nvim_open_win(buf, true, win_config)
	elseif win_config.type == "tabnew" then
		-- Check if Claude Code tab already exists
		local existing_tab, existing_win = find_claude_tab()
		if existing_tab then
			-- Switch to existing tab and load buffer
			vim.api.nvim_set_current_tabpage(existing_tab)
			vim.api.nvim_set_current_win(existing_win)
			vim.api.nvim_win_set_buf(existing_win, buf)
		else
			-- Create new tab and load buffer
			vim.cmd("tabnew")
			vim.api.nvim_win_set_buf(0, buf)
		end
	elseif win_config.type == "buffer" then
		-- Use current window
		vim.api.nvim_win_set_buf(0, buf)
	elseif win_config.type == "newbuffer" then
		-- Switch to buffer in current window
		vim.api.nvim_win_set_buf(0, buf)
	else
		-- Use split
		local split_cmd = win_config.type == "vsplit" and "vsplit" or "split"
		vim.cmd(split_cmd)
		vim.api.nvim_win_set_buf(0, buf)
	end
end

function M.browse_sessions()
	local sessions = M.list_sessions()
	
	if #sessions == 0 then
		vim.notify("No Claude Code sessions found", vim.log.levels.INFO)
		return
	end
	
	-- Create selection menu
	local choices = {}
	for i, session in ipairs(sessions) do
		table.insert(choices, string.format("%d. %s", i, session.display_name))
	end
	
	vim.ui.select(choices, {
		prompt = "Select a Claude Code session:",
		format_item = function(item) return item end,
	}, function(choice, idx)
		if choice and idx then
			local selected_session = sessions[idx]
			-- Ask what to do with the selected session
			vim.ui.select(
				{"View session", "Restore session"},
				{
					prompt = "What would you like to do with: " .. selected_session.display_name,
					format_item = function(item) return item end,
				},
				function(action)
					if action == "View session" then
						M.load_session(selected_session.file)
					elseif action == "Restore session" then
						M.restore_session(selected_session.file)
					end
				end
			)
		end
	end)
end

function M.save_session_with_name(name)
	if not state.bufnr or not vim.api.nvim_buf_is_valid(state.bufnr) then
		vim.notify("No active Claude Code session to save", vim.log.levels.WARN)
		return
	end
	
	create_session_dir()
	
	-- Sanitize the name for filename
	local safe_name = name:gsub("[^%w%s%-_]", ""):gsub("%s+", "_")
	local timestamp = os.date("%Y%m%d_%H%M%S")
	local filename = string.format("session_%s_%s.txt", timestamp, safe_name)
	local filepath = state.config.session_dir .. filename
	
	-- Get buffer content
	local lines = vim.api.nvim_buf_get_lines(state.bufnr, 0, -1, false)
	
	-- Save to file
	vim.fn.writefile(lines, filepath)
	vim.notify("Session saved as: " .. filename, vim.log.levels.INFO)
	
	-- Mark this session as named and set the session file
	state.named_session = true
	state.session_file = filepath
end

function M.update_current_session()
	if not state.bufnr or not vim.api.nvim_buf_is_valid(state.bufnr) then
		vim.notify("No active Claude Code session to update", vim.log.levels.WARN)
		return
	end
	
	if not state.named_session or not state.session_file then
		vim.notify("No named session to update. Use :ClaudeCodeSaveSession first", vim.log.levels.WARN)
		return
	end
	
	-- Update existing file
	local lines = vim.api.nvim_buf_get_lines(state.bufnr, 0, -1, false)
	vim.fn.writefile(lines, state.session_file)
	local current_name = vim.fn.fnamemodify(state.session_file, ":t")
	vim.notify("Session updated: " .. current_name, vim.log.levels.INFO)
end

function M.save_session_interactive()
	-- If already has a named session, ask if they want to update or create new
	if state.named_session and state.session_file then
		local current_name = vim.fn.fnamemodify(state.session_file, ":t")
		vim.ui.select(
			{"Update current session", "Create new session"},
			{
				prompt = "Session already saved as: " .. current_name,
				format_item = function(item) return item end,
			},
			function(choice)
				if choice == "Update current session" then
					M.update_current_session()
				elseif choice == "Create new session" then
					-- Ask for new name
					vim.ui.input({
						prompt = "Enter new session name: ",
						default = "",
					}, function(input)
						if input and input ~= "" then
							M.save_session_with_name(input)
						end
					end)
				end
			end
		)
	else
		-- No existing named session, just ask for name
		vim.ui.input({
			prompt = "Enter session name: ",
			default = "",
		}, function(input)
			if input and input ~= "" then
				M.save_session_with_name(input)
			end
		end)
	end
end

function M.restore_session(session_file)
	if not vim.fn.filereadable(session_file) then
		vim.notify("Session file not found: " .. session_file, vim.log.levels.ERROR)
		return
	end
	
	-- Close current session if it exists
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
	state.named_session = false
	
	-- Create new Claude buffer
	create_claude_buffer()
	
	-- Wait for terminal to be ready, then send the session content
	vim.defer_fn(function()
		if state.term_job_id and state.bufnr and vim.api.nvim_buf_is_valid(state.bufnr) then
			-- Read session content and send it
			local lines = vim.fn.readfile(session_file)
			local content = table.concat(lines, "\n")
			
			-- Send the content to Claude Code to restore the context
			vim.fn.chansend(state.term_job_id, "/resume\n")
			vim.defer_fn(function()
				vim.fn.chansend(state.term_job_id, content .. "\n")
			end, 200)
		end
	end, 500)
	
	vim.notify("Session restored from: " .. vim.fn.fnamemodify(session_file, ":t"), vim.log.levels.INFO)
end

function M.restore_session_interactive()
	local sessions = M.list_sessions()
	
	if #sessions == 0 then
		vim.notify("No Claude Code sessions found to restore", vim.log.levels.INFO)
		return
	end
	
	-- Create selection menu
	local choices = {}
	for i, session in ipairs(sessions) do
		table.insert(choices, string.format("%d. %s", i, session.display_name))
	end
	
	vim.ui.select(choices, {
		prompt = "Select a Claude Code session to restore:",
		format_item = function(item) return item end,
	}, function(choice, idx)
		if choice and idx then
			M.restore_session(sessions[idx].file)
		end
	end)
end

return M
