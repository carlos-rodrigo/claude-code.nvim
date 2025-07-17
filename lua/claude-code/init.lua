-- claude-code.nvim - Simplified terminal approach
-- A Neovim plugin for Claude Code integration
-- https://github.com/carlos-rodrigo/claude-code.nvim

local M = {}

-- Default configuration
local default_config = {
	claude_code_cmd = "claude",
	window = {
		type = "current", -- "split", "vsplit", "tabnew", "current"
	},
	auto_scroll = true,
	save_session = true,
	auto_save_session = true,
	auto_save_notify = true,
	session_dir = vim.fn.stdpath("data") .. "/claude-code-sessions/",
	max_exchanges = 20, -- Maximum exchanges to keep in session
	setup_claude_commands = true, -- Automatically setup Claude custom commands
	keybindings = {
		toggle = "<leader>clc",
		new_session = "<leader>cln",
		send_selection = "<leader>cls",
		save_session = "<leader>clS",
		update_session = "<leader>clu",
		browse_sessions = "<leader>clb",
		restore_session = "<leader>clr",
		new_with_selection = "<leader>clw",
		open_vsplit = "<leader>clv",
	},
}

-- Plugin state
local state = {
	config = {},
	terminal_bufnr = nil,
	sessions = {}, -- Store multiple session buffers
	current_session = {
		name = nil,
		filepath = nil,
		last_saved_line = 0,
		is_named = false,
	},
}

-- Find Claude Code terminal buffer (main session only)
local function find_claude_terminal()
	-- Only return the main terminal buffer, not vsplit sessions
	if state.terminal_bufnr and vim.api.nvim_buf_is_valid(state.terminal_bufnr) then
		local name = vim.api.nvim_buf_get_name(state.terminal_bufnr)
		if name:match("term://.*claude$") then
			local info = vim.fn.getbufinfo(state.terminal_bufnr)[1]
			if info and info.loaded == 1 then
				return state.terminal_bufnr
			end
		end
	end
	
	-- If state.terminal_bufnr is invalid, search for any claude terminal
	-- but exclude sessions array to avoid picking up vsplit sessions
	local buffers = vim.api.nvim_list_bufs()
	for _, buf in ipairs(buffers) do
		-- Skip if this buffer is in our sessions array
		local is_session = false
		for _, session_buf in ipairs(state.sessions) do
			if buf == session_buf then
				is_session = true
				break
			end
		end
		
		if not is_session and vim.api.nvim_buf_is_valid(buf) then
			local name = vim.api.nvim_buf_get_name(buf)
			if name:match("term://.*claude$") then
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
			if window_type == "current" then
				-- Use current window
				vim.api.nvim_win_set_buf(0, state.terminal_bufnr)
			elseif window_type == "tabnew" then
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
		if window_type == "current" then
			-- Use current window
			vim.cmd("terminal " .. cmd)
		elseif window_type == "tabnew" then
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
		-- Smart Esc handling - double tap to exit terminal mode
		vim.api.nvim_buf_set_keymap(buf, 't', '<Esc><Esc>', '<C-\\><C-n>', { noremap = true })
		-- Ctrl+[ as alternative to exit terminal mode
		vim.api.nvim_buf_set_keymap(buf, 't', '<C-[>', '<C-\\><C-n>', { noremap = true })
		-- Ctrl+n as another alternative
		vim.api.nvim_buf_set_keymap(buf, 't', '<C-n>', '<C-\\><C-n>', { noremap = true })
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
		-- Check window type
		local window_type = state.config.window.type
		
		if window_type == "current" then
			-- For current window mode, switch to a new empty buffer
			vim.cmd("enew")
		else
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

-- Open Claude Code in vsplit (new independent session)
function M.open_vsplit()
	local cmd = state.config.claude_code_cmd
	
	-- Check if command exists
	if vim.fn.executable(cmd) == 0 then
		vim.notify("Command '" .. cmd .. "' not found. Please install Claude CLI.", vim.log.levels.ERROR)
		return
	end
	
	-- Always create a new independent session for vsplit
	vim.cmd("vsplit | terminal " .. cmd)
	local new_buf = vim.api.nvim_get_current_buf()
	
	-- Store this as a separate session
	table.insert(state.sessions, new_buf)
	
	-- Set up buffer-local keymaps
	-- Smart Esc handling - double tap to exit terminal mode
	vim.api.nvim_buf_set_keymap(new_buf, 't', '<Esc><Esc>', '<C-\\><C-n>', { noremap = true })
	-- Ctrl+[ as alternative to exit terminal mode
	vim.api.nvim_buf_set_keymap(new_buf, 't', '<C-[>', '<C-\\><C-n>', { noremap = true })
	-- Ctrl+n as another alternative
	vim.api.nvim_buf_set_keymap(new_buf, 't', '<C-n>', '<C-\\><C-n>', { noremap = true })
	vim.api.nvim_buf_set_keymap(new_buf, 'n', 'q', ':q<CR>', { noremap = true })
	
	-- Disable various plugins for this buffer
	vim.schedule(function()
		if vim.api.nvim_buf_is_valid(new_buf) then
			vim.b[new_buf].ts_highlight = false
			vim.b[new_buf].render_markdown = false
		end
	end)
	
	-- Enter insert mode
	vim.cmd("startinsert")
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

-- Create session directory if it doesn't exist
local function create_session_dir()
	local session_dir = state.config.session_dir
	if vim.fn.isdirectory(session_dir) == 0 then
		vim.fn.mkdir(session_dir, "p")
	end
end

-- Setup Claude custom commands
local function setup_claude_commands()
	-- Get the current working directory (project root)
	local cwd = vim.fn.getcwd()
	local claude_commands_dir = cwd .. "/.claude/commands"
	
	-- Create .claude/commands directory if it doesn't exist
	if vim.fn.isdirectory(claude_commands_dir) == 0 then
		vim.fn.mkdir(claude_commands_dir, "p")
	end
	
	-- Get the plugin directory
	local plugin_dir = vim.fn.fnamemodify(debug.getinfo(1, "S").source:sub(2), ":h")
	local templates_dir = plugin_dir .. "/commands"
	
	-- List of command templates to copy
	local commands = {"ship.md", "plan.md"}
	
	-- Copy each command template to the project directory
	for _, command_file in ipairs(commands) do
		local template_path = templates_dir .. "/" .. command_file
		local target_path = claude_commands_dir .. "/" .. command_file
		
		-- Check if template exists
		if vim.fn.filereadable(template_path) == 1 then
			-- Read template content
			local template_content = vim.fn.readfile(template_path)
			-- Write to target location
			vim.fn.writefile(template_content, target_path)
		else
			vim.notify("Warning: Command template not found: " .. template_path, vim.log.levels.WARN)
		end
	end
	
	-- Only notify on first setup, not on every plugin load
	if not vim.g.claude_code_commands_setup then
		vim.g.claude_code_commands_setup = true
		vim.notify("Claude Code: Custom commands installed in " .. claude_commands_dir, vim.log.levels.INFO)
	end
end

-- Pattern recognition for content classification
local user_prompt_patterns = {
	"^Human:%s*(.+)",                  -- Standard Claude format
	"^>%s*(.+)",                       -- Command line style
	"^%$%s*(.+)",                      -- Shell prompt style
	"^┃%s*(.+)",                       -- Claude's input line marker
	"^│%s*(.+)",                       -- Alternative input marker
	"^You:%s*(.+)",                    -- Claude Code "You:" format
	"^%s*>>>%s*(.+)",                  -- Python-style prompt
}

local claude_response_patterns = {
	"^Assistant:%s*(.+)",              -- Standard response format
	"^Claude:%s*(.+)",                 -- Alternative format
}

-- Classify line type
local function classify_line(line)
	-- Remove ANSI escape sequences
	local clean_line = line:gsub("\27%[[;%d]*m", ""):gsub("[\r\n\t]", " ")
	
	-- Skip empty lines
	if clean_line:match("^%s*$") then
		return "skip", nil
	end
	
	-- Skip terminal UI elements
	if clean_line:match("^[─┌┐└┘├┤┬┴┼═║╔╗╚╝╠╣╦╩╬│]+%s*$") then
		return "ui_element", nil
	end
	
	-- Check for user prompts
	for _, pattern in ipairs(user_prompt_patterns) do
		local content = clean_line:match(pattern)
		if content then
			return "user_prompt", content
		end
	end
	
	-- Check for Claude responses
	for _, pattern in ipairs(claude_response_patterns) do
		local content = clean_line:match(pattern)
		if content then
			return "claude_start", content
		end
	end
	
	-- Code fence detection
	if clean_line:match("^%s*```") then
		return "code_fence", clean_line
	end
	
	-- System messages to filter
	if clean_line:match("^%[System%]") or 
	   clean_line:match("^Loading%.%.%.") or
	   clean_line:match("^Thinking%.%.%.") or
	   clean_line:match("^%[%d+m%s*$") then
		return "system_message", nil
	end
	
	-- Default to content
	return "content", clean_line
end

-- Extract essential conversation elements
local function extract_essential_content(buffer_lines, start_line)
	local essential_content = {}
	local in_code_block = false
	local code_block_lines = {}
	local last_user_prompt_idx = nil
	local current_response = {}
	local in_claude_response = false
	
	start_line = start_line or 1
	
	for i = start_line, #buffer_lines do
		local line = buffer_lines[i]
		local line_type, content = classify_line(line)
		
		if line_type == "user_prompt" then
			-- Save any pending response
			if in_claude_response and #current_response > 0 then
				table.insert(essential_content, {
					type = "response",
					lines = current_response,
					start_line = current_response[1].line_num
				})
				current_response = {}
				in_claude_response = false
			end
			
			-- Add user prompt
			table.insert(essential_content, {
				type = "user",
				content = content,
				line_num = i
			})
			last_user_prompt_idx = #essential_content
			
		elseif line_type == "claude_start" then
			in_claude_response = true
			current_response = {{content = content, line_num = i}}
			
		elseif line_type == "code_fence" then
			if not in_code_block then
				in_code_block = true
				code_block_lines = {}
				if in_claude_response then
					table.insert(current_response, {content = content, line_num = i})
				end
			else
				-- End of code block
				if in_claude_response then
					table.insert(current_response, {content = content, line_num = i})
					-- Add code summary if block is large
					if #code_block_lines > 20 then
						table.insert(current_response, {
							content = string.format("[Code block condensed: %d lines]", #code_block_lines),
							line_num = i
						})
					else
						-- Include all lines for small blocks
						for _, code_line in ipairs(code_block_lines) do
							table.insert(current_response, code_line)
						end
					end
				end
				in_code_block = false
				code_block_lines = {}
			end
			
		elseif in_code_block then
			table.insert(code_block_lines, {content = line, line_num = i})
			
		elseif line_type == "content" and in_claude_response then
			-- Keep content that's part of Claude's response
			table.insert(current_response, {content = content, line_num = i})
			
		elseif line_type == "skip" or line_type == "ui_element" or line_type == "system_message" then
			-- Skip these lines
		end
	end
	
	-- Save any pending response
	if in_claude_response and #current_response > 0 then
		table.insert(essential_content, {
			type = "response",
			lines = current_response,
			start_line = current_response[1].line_num
		})
	end
	
	return essential_content
end

-- Reduce tokens by keeping only recent exchanges
local function reduce_tokens(essential_content, max_exchanges)
	max_exchanges = max_exchanges or 10
	
	-- Group into exchanges
	local exchanges = {}
	local current_exchange = nil
	
	for _, item in ipairs(essential_content) do
		if item.type == "user" then
			if current_exchange then
				table.insert(exchanges, current_exchange)
			end
			current_exchange = {
				user_prompt = item,
				response = nil
			}
		elseif item.type == "response" and current_exchange then
			current_exchange.response = item
		end
	end
	
	-- Add last exchange if exists
	if current_exchange then
		table.insert(exchanges, current_exchange)
	end
	
	-- Keep only recent exchanges
	local start_idx = math.max(1, #exchanges - max_exchanges + 1)
	local recent_exchanges = {}
	
	for i = start_idx, #exchanges do
		table.insert(recent_exchanges, exchanges[i])
	end
	
	return recent_exchanges
end

-- Format session content for saving
local function format_session_content(exchanges, metadata)
	local lines = {}
	
	-- Add metadata header
	table.insert(lines, "=== Claude Code Session ===")
	table.insert(lines, "Session: " .. (metadata.name or "Untitled"))
	table.insert(lines, "Created: " .. (metadata.created_at or os.date("%Y-%m-%d %H:%M:%S")))
	table.insert(lines, "Updated: " .. os.date("%Y-%m-%d %H:%M:%S"))
	table.insert(lines, "Exchanges: " .. #exchanges)
	table.insert(lines, "========================")
	table.insert(lines, "")
	
	-- Add exchanges
	for i, exchange in ipairs(exchanges) do
		-- User prompt
		if exchange.user_prompt then
			table.insert(lines, string.format("### Exchange %d ###", i))
			table.insert(lines, "Human: " .. exchange.user_prompt.content)
			table.insert(lines, "")
		end
		
		-- Claude response
		if exchange.response then
			table.insert(lines, "Assistant:")
			for _, resp_line in ipairs(exchange.response.lines) do
				table.insert(lines, resp_line.content)
			end
			table.insert(lines, "")
		end
	end
	
	return lines
end

-- Save session with a custom name
function M.save_session_interactive()
	local buf = state.terminal_bufnr or find_claude_terminal()
	if not buf or not vim.api.nvim_buf_is_valid(buf) then
		vim.notify("No active Claude Code session to save", vim.log.levels.WARN)
		return
	end
	
	-- Check if updating existing session or creating new
	if state.current_session.is_named and state.current_session.filepath then
		local current_name = state.current_session.name
		vim.ui.select(
			{"Update existing session", "Save as new session"},
			{
				prompt = "Session '" .. current_name .. "' already exists:",
				format_item = function(item) return item end,
			},
			function(choice)
				if choice == "Update existing session" then
					M.update_current_session()
				elseif choice == "Save as new session" then
					vim.ui.input({
						prompt = "Enter new session name: ",
						default = current_name .. "_v2",
					}, function(input)
						if input and input ~= "" then
							M.save_session_with_name(buf, input, true)
						end
					end)
				end
			end
		)
	else
		-- New session
		vim.ui.input({
			prompt = "Enter session name: ",
			default = "",
		}, function(input)
			if input and input ~= "" then
				M.save_session_with_name(buf, input, true)
			end
		end)
	end
end

-- Save session with specified name
function M.save_session_with_name(buf, name, is_new)
	if not buf or not vim.api.nvim_buf_is_valid(buf) then
		vim.notify("Invalid buffer for session saving", vim.log.levels.ERROR)
		return
	end
	
	create_session_dir()
	
	-- Get buffer lines
	local lines = vim.api.nvim_buf_get_lines(buf, 0, -1, false)
	
	-- Extract essential content starting from last saved line
	local start_line = is_new and 1 or (state.current_session.last_saved_line + 1)
	local essential_content = extract_essential_content(lines, start_line)
	
	-- Reduce tokens if needed
	local exchanges = reduce_tokens(essential_content, state.config.max_exchanges or 20)
	
	-- Prepare metadata
	local metadata = {
		name = name,
		created_at = state.current_session.created_at or os.date("%Y-%m-%d %H:%M:%S"),
	}
	
	-- Format content
	local formatted_lines = format_session_content(exchanges, metadata)
	
	-- Determine filepath
	local filepath
	if is_new or not state.current_session.filepath then
		-- Create new file
		local safe_name = name:gsub("[^%w%s%-_]", ""):gsub("%s+", "_")
		local timestamp = os.date("%Y%m%d_%H%M%S")
		local filename = string.format("session_%s_%s.txt", timestamp, safe_name)
		filepath = state.config.session_dir .. filename
	else
		-- Use existing filepath
		filepath = state.current_session.filepath
	end
	
	-- Save to file
	local success = pcall(vim.fn.writefile, formatted_lines, filepath)
	if success then
		-- Update state
		state.current_session = {
			name = name,
			filepath = filepath,
			last_saved_line = #lines,
			is_named = true,
			created_at = metadata.created_at,
		}
		
		local filename = vim.fn.fnamemodify(filepath, ":t")
		vim.notify("Session saved: " .. filename .. " (" .. #exchanges .. " exchanges)", vim.log.levels.INFO)
	else
		vim.notify("Failed to save session", vim.log.levels.ERROR)
	end
end

-- Update current session with new content
function M.update_current_session()
	if not state.current_session.is_named or not state.current_session.filepath then
		vim.notify("No named session to update. Use save session first.", vim.log.levels.WARN)
		return
	end
	
	local buf = state.terminal_bufnr or find_claude_terminal()
	if not buf or not vim.api.nvim_buf_is_valid(buf) then
		vim.notify("No active Claude Code session to update", vim.log.levels.WARN)
		return
	end
	
	M.save_session_with_name(buf, state.current_session.name, false)
end

-- List saved sessions
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

-- Browse saved sessions
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
				{"View session", "Load session content"},
				{
					prompt = "What would you like to do with: " .. selected_session.display_name,
					format_item = function(item) return item end,
				},
				function(action)
					if action == "View session" then
						M.view_session(selected_session.file)
					elseif action == "Load session content" then
						M.load_session_content(selected_session.file)
					end
				end
			)
		end
	end)
end

-- View session in a new buffer
function M.view_session(session_file)
	if not vim.fn.filereadable(session_file) then
		vim.notify("Session file not found: " .. session_file, vim.log.levels.ERROR)
		return
	end
	
	-- Create a new buffer for viewing the session
	local buf = vim.api.nvim_create_buf(false, true)
	vim.api.nvim_buf_set_name(buf, "Claude Session: " .. vim.fn.fnamemodify(session_file, ":t"))
	
	-- Read session content
	local lines = vim.fn.readfile(session_file)
	vim.api.nvim_buf_set_lines(buf, 0, -1, false, lines)
	
	-- Set buffer options
	vim.bo[buf].modifiable = false
	vim.bo[buf].buftype = "nofile"
	vim.bo[buf].swapfile = false
	vim.bo[buf].filetype = "markdown"
	
	-- Open in current window
	vim.api.nvim_win_set_buf(0, buf)
end

-- Load session content into current Claude session
function M.load_session_content(session_file)
	if not vim.fn.filereadable(session_file) then
		vim.notify("Session file not found: " .. session_file, vim.log.levels.ERROR)
		return
	end
	
	local buf = state.terminal_bufnr or find_claude_terminal()
	if not buf or not vim.api.nvim_buf_is_valid(buf) then
		vim.notify("No active Claude Code session. Start Claude first.", vim.log.levels.WARN)
		return
	end
	
	-- Read session content
	local lines = vim.fn.readfile(session_file)
	local content = table.concat(lines, "\n")
	
	-- Send /resume command followed by session content
	if vim.b[buf].terminal_job_id then
		vim.fn.chansend(vim.b[buf].terminal_job_id, "/resume\n")
		vim.defer_fn(function()
			vim.fn.chansend(vim.b[buf].terminal_job_id, content .. "\n")
		end, 200)
		vim.notify("Session content loaded into Claude", vim.log.levels.INFO)
	end
end

-- Setup function
function M.setup(opts)
	state.config = vim.tbl_deep_extend("force", default_config, opts or {})
	
	-- Setup Claude custom commands if enabled
	if state.config.setup_claude_commands then
		setup_claude_commands()
	end
	
	-- Create user commands
	vim.api.nvim_create_user_command("ClaudeCode", M.open, { desc = "Open Claude Code" })
	vim.api.nvim_create_user_command("ClaudeCodeToggle", M.toggle, { desc = "Toggle Claude Code" })
	vim.api.nvim_create_user_command("ClaudeCodeNew", M.new_session, { desc = "New Claude Code session" })
	vim.api.nvim_create_user_command("ClaudeCodeVsplit", M.open_vsplit, { desc = "Open Claude Code in vsplit" })
	vim.api.nvim_create_user_command("ClaudeCodeSend", function(cmd_opts)
		M.send_selection(cmd_opts.line1, cmd_opts.line2)
	end, { desc = "Send selection to Claude Code", range = true })
	
	-- Session saving functionality
	vim.api.nvim_create_user_command("ClaudeCodeSaveSession", function()
		M.save_session_interactive()
	end, { desc = "Save Claude Code session" })
	vim.api.nvim_create_user_command("ClaudeCodeUpdateSession", function()
		M.update_current_session()
	end, { desc = "Update Claude Code session" })
	vim.api.nvim_create_user_command("ClaudeCodeSessions", function()
		M.browse_sessions()
	end, { desc = "Browse Claude Code sessions" })
	vim.api.nvim_create_user_command("ClaudeCodeRestoreSession", function()
		vim.notify("Session restoration not yet implemented in this version", vim.log.levels.INFO)
	end, { desc = "Restore Claude Code session" })
	vim.api.nvim_create_user_command("ClaudeCodeNewWithSelection", function(cmd_opts)
		-- Get selection and start new session
		local current_buf = vim.api.nvim_get_current_buf()
		local lines = vim.api.nvim_buf_get_lines(current_buf, cmd_opts.line1 - 1, cmd_opts.line2, false)
		local text = table.concat(lines, "\n")
		
		M.new_session()
		
		-- Wait for terminal to be ready, then send text
		vim.defer_fn(function()
			local buf = state.terminal_bufnr or find_claude_terminal()
			if buf and vim.b[buf].terminal_job_id then
				vim.fn.chansend(vim.b[buf].terminal_job_id, text .. "\n")
			end
		end, 100)
	end, { desc = "New Claude Code session with selection", range = true })
	
	-- Command to manually setup Claude commands
	vim.api.nvim_create_user_command("ClaudeCodeSetupCommands", function()
		setup_claude_commands()
		vim.notify("Claude Code: Custom commands setup completed", vim.log.levels.INFO)
	end, { desc = "Setup Claude Code custom commands" })
	
	-- Set up keybindings
	local keys = state.config.keybindings
	if keys and keys ~= false then
		-- Normal mode keybindings
		if keys.toggle then
			vim.keymap.set("n", keys.toggle, "<cmd>ClaudeCodeToggle<cr>", { desc = "claude: toggle" })
		end
		if keys.new_session then
			vim.keymap.set("n", keys.new_session, "<cmd>ClaudeCodeNew<cr>", { desc = "claude: new session" })
		end
		if keys.open_vsplit then
			vim.keymap.set("n", keys.open_vsplit, "<cmd>ClaudeCodeVsplit<cr>", { desc = "claude: vsplit" })
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
		
		-- Visual mode keybindings
		if keys.send_selection then
			vim.keymap.set("v", keys.send_selection, "<cmd>ClaudeCodeSend<cr>", { desc = "claude: send selection" })
		end
		if keys.new_with_selection then
			vim.keymap.set("v", keys.new_with_selection, "<cmd>ClaudeCodeNewWithSelection<cr>", { desc = "claude: new with selection" })
		end
		
		-- Set up which-key integration
		vim.defer_fn(function()
			local ok, which_key_config = pcall(require, "claude-code.which-key")
			if ok then
				which_key_config.setup(keys)
			end
		end, 100)
	end
end

return M