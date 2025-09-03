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
	session_dir = nil, -- Will be set dynamically to project_root/.claude/sessions/
	setup_claude_commands = false, -- Don't automatically setup Claude custom commands
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

local last_auto_save_time = 0
local auto_save_in_progress = false

-- Get session directory (.claude/sessions/ in project root) with validation
local function get_session_dir()
	if not state.config.session_dir then
		local cwd = vim.fn.getcwd()
		-- Ensure we're working with absolute paths and validate
		cwd = vim.fn.fnamemodify(cwd, ":p")
		
		-- Validate that cwd is a real directory
		if vim.fn.isdirectory(cwd) == 0 then
			error("Invalid working directory: " .. cwd)
		end
		
		-- Build the session directory path
		state.config.session_dir = vim.fs.normalize(cwd .. "/.claude/sessions")
		
		-- Additional validation to prevent traversal
		if state.config.session_dir:find("%.%.") then
			error("Invalid session directory path: contains directory traversal")
		end
		
		-- Ensure path ends with separator
		if not state.config.session_dir:match("/$") then
			state.config.session_dir = state.config.session_dir .. "/"
		end
	end
	return state.config.session_dir
end

-- Create session directory if it doesn't exist with error handling
local function create_session_dir()
	local success, session_dir = pcall(get_session_dir)
	if not success then
		error("Failed to get session directory: " .. tostring(session_dir))
	end
	
	-- Validate the path doesn't contain dangerous patterns
	if session_dir:find("%.%.") or session_dir:find("~") then
		error("Invalid session directory path: " .. session_dir)
	end
	
	if vim.fn.isdirectory(session_dir) == 0 then
		local mkdir_success = pcall(vim.fn.mkdir, session_dir, "p")
		if not mkdir_success then
			error("Failed to create session directory: " .. session_dir)
		end
	end
end

-- Clean terminal artifacts and UI elements from buffer content
local function clean_terminal_content(buffer_lines)
	local cleaned = {}
	
	for _, line in ipairs(buffer_lines) do
		-- Skip lines that are just terminal UI elements (box drawing)
		if line:match("^[╭─│╰╮╯┌┐└┘├┤┬┴┼]+%s*$") then
			-- Skip pure box-drawing lines
			goto continue
		end
		
		-- Skip the input prompt box line with just ">"
		if line:match("^│%s*>%s*│$") then
			goto continue
		end
		
		-- Skip lines with corrupted unicode/escape sequences
		if line:find("�") or line:find("\027%[") then
			-- Try to clean the line by removing escape sequences
			local cleaned_line = line:gsub("\027%[[%d;]*m", ""):gsub("�", "")
			if cleaned_line:match("^%s*$") then
				goto continue
			end
			line = cleaned_line
		end
		
		-- Skip the "? for shortcuts" line at the end
		if line:match("^%s*%? for shortcuts%s*$") then
			goto continue
		end
		
		table.insert(cleaned, line)
		::continue::
	end
	
	-- Remove trailing empty lines
	while #cleaned > 0 and cleaned[#cleaned]:match("^%s*$") do
		table.remove(cleaned)
	end
	
	return cleaned
end

-- Format full session content as markdown
local function format_full_session_content(buffer_lines, metadata)
	local lines = {}
	
	-- Clean the buffer content first
	local cleaned_content = clean_terminal_content(buffer_lines)
	
	-- Add markdown metadata header
	table.insert(lines, "# Claude Code Session")
	table.insert(lines, "")
	table.insert(lines, "**Session:** " .. (metadata.name or "Untitled"))
	table.insert(lines, "**Created:** " .. (metadata.created_at or os.date("%Y-%m-%d %H:%M:%S")))
	table.insert(lines, "**Updated:** " .. os.date("%Y-%m-%d %H:%M:%S"))
	table.insert(lines, "**Total Lines:** " .. #cleaned_content)
	table.insert(lines, "")
	table.insert(lines, "---")
	table.insert(lines, "")
	table.insert(lines, "## Session Content")
	table.insert(lines, "")
	
	-- Add cleaned buffer content in a code block to preserve formatting
	table.insert(lines, "```terminal")
	for _, line in ipairs(cleaned_content) do
		table.insert(lines, line)
	end
	table.insert(lines, "```")
	
	return lines
end

-- Retry mechanism for failed operations
local function retry_operation(operation, max_retries, delay_ms)
	max_retries = max_retries or 3
	delay_ms = delay_ms or 100
	
	local retries = 0
	local last_error
	
	repeat
		local success, result = pcall(operation)
		if success then
			return true, result
		end
		
		last_error = result
		retries = retries + 1
		
		if retries < max_retries then
			-- Wait before retrying
			vim.wait(delay_ms, function() return false end)
		end
	until retries >= max_retries
	
	return false, "Operation failed after " .. max_retries .. " retries. Last error: " .. tostring(last_error)
end

-- Secure filename sanitization to prevent directory traversal and other attacks
local function sanitize_filename(name)
	if not name or name == "" then
		return "untitled"
	end
	
	-- Remove path separators and dangerous characters
	local safe_name = name:gsub("[/\\:*?\"<>|%c]", "")  -- Remove dangerous chars
	                      :gsub("%.%.", "")              -- Remove directory traversal
	                      :gsub("^%s*(.-)%s*$", "%1")   -- Trim whitespace
	                      :gsub("%s+", "_")              -- Replace spaces with underscores
	                      :sub(1, 50)                    -- Limit length to 50 chars
	
	-- Ensure non-empty result
	if safe_name == "" then
		safe_name = "untitled"
	end
	
	-- Ensure it doesn't start with a dot (hidden file)
	if safe_name:sub(1, 1) == "." then
		safe_name = "_" .. safe_name:sub(2)
	end
	
	return safe_name
end

-- Get incremental buffer changes since last save
local function get_buffer_changes_since_last_save()
	if not state.terminal_bufnr or not vim.api.nvim_buf_is_valid(state.terminal_bufnr) then
		return nil, "Invalid buffer"
	end
	
	local current_line_count = vim.api.nvim_buf_line_count(state.terminal_bufnr)
	local last_saved = state.current_session.last_saved_line or 0
	
	-- If buffer has been cleared or reduced, return full content
	if current_line_count < last_saved then
		return vim.api.nvim_buf_get_lines(state.terminal_bufnr, 0, -1, false), true
	end
	
	-- If no changes, return empty
	if current_line_count == last_saved then
		return {}, false
	end
	
	-- Return only new lines
	return vim.api.nvim_buf_get_lines(state.terminal_bufnr, last_saved, -1, false), false
end

-- Auto-save session to file with proper error handling and concurrency control
local function auto_save_session()
	-- Prevent concurrent saves
	if auto_save_in_progress then
		return
	end
	
	-- Debounce: prevent saves more frequent than every 2 seconds
	local current_time = vim.loop.now()
	if current_time - last_auto_save_time < 2000 then
		return
	end
	
	if not (state.config.save_session and state.config.auto_save_session and state.terminal_bufnr and vim.api.nvim_buf_is_valid(state.terminal_bufnr)) then
		return
	end
	
	auto_save_in_progress = true
	last_auto_save_time = current_time
	
	-- Wrap the entire operation in pcall for error handling
	local success, err = pcall(function()
		-- Save using current session if it exists
		if state.current_session.is_named and state.current_session.filepath then
			M.update_current_session()
		else
			-- Create session directory with error handling
			local dir_success, dir_err = pcall(create_session_dir)
			if not dir_success then
				error("Failed to create session directory: " .. tostring(dir_err))
			end
			
			-- Use incremental saving for better performance
			local changes, full_reload = get_buffer_changes_since_last_save()
			if not changes then
				error("Failed to get buffer changes: " .. tostring(full_reload))
			end
			
			-- Skip if no changes
			if #changes == 0 and not full_reload then
				return
			end
			
			local lines
			if full_reload or not state.current_session.auto_session_file then
				-- Get full buffer content for initial save or after buffer clear
				local lines_success
				lines_success, lines = pcall(vim.api.nvim_buf_get_lines, state.terminal_bufnr, 0, -1, false)
				if not lines_success then
					error("Failed to read buffer content: " .. tostring(lines))
				end
			else
				-- Append only new lines to existing session
				lines = changes
			end
			
			-- Format all buffer content as markdown
			local metadata = {
				name = "auto_session",
				created_at = os.date("%Y-%m-%d %H:%M:%S"),
				incremental = not full_reload and state.current_session.auto_session_file ~= nil,
			}
			
			local format_success, formatted_lines = pcall(format_full_session_content, lines, metadata)
			if not format_success then
				error("Failed to format session content: " .. tostring(formatted_lines))
			end
			
			-- Determine session file
			local session_file
			if state.current_session.auto_session_file and not full_reload then
				-- Append to existing file with retry
				session_file = state.current_session.auto_session_file
				local append_success, append_result = retry_operation(function()
					local result = vim.fn.writefile(formatted_lines, session_file, "a")
					if result ~= 0 then
						error("writefile append returned " .. result)
					end
					return true
				end, 3, 200)
				if not append_success then
					error("Failed to append to session file: " .. tostring(append_result))
				end
			else
				-- Create new file
				local timestamp = os.date("%Y%m%d_%H%M%S")
				session_file = get_session_dir() .. "auto_session_" .. timestamp .. ".md"
				state.current_session.auto_session_file = session_file
				
				-- Write file with retry mechanism
				local write_success, write_result = retry_operation(function()
					local result = vim.fn.writefile(formatted_lines, session_file)
					if result ~= 0 then
						error("writefile returned " .. result)
					end
					return true
				end, 3, 200)
				if not write_success then
					error("Failed to write session file: " .. tostring(write_result))
				end
			end
			
			-- Update last saved line
			state.current_session.last_saved_line = vim.api.nvim_buf_line_count(state.terminal_bufnr)
			
			-- Show a brief notification if enabled
			if state.config.auto_save_notify then
				vim.notify("Session auto-saved" .. (metadata.incremental and " (incremental)" or ""), vim.log.levels.INFO, { timeout = 2000 })
			end
		end
	end)
	
	auto_save_in_progress = false
	
	if not success then
		vim.notify("Auto-save failed: " .. tostring(err), vim.log.levels.ERROR)
	end
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
		
		-- Auto-save session on focus loss with proper cleanup
		if state.config.save_session and state.config.auto_save_session then
			-- Create unique autocmd groups for this buffer
			local auto_save_group = vim.api.nvim_create_augroup("ClaudeCodeAutoSave_" .. buf, { clear = true })
			local explorer_save_group = vim.api.nvim_create_augroup("ClaudeCodeExplorerSave_" .. buf, { clear = true })
			
			-- Buffer-specific auto-save
			vim.api.nvim_create_autocmd({"FocusLost", "BufLeave"}, {
				buffer = buf,
				group = auto_save_group,
				callback = auto_save_session,
			})
			
			-- Global auto-save when switching away from Claude terminal
			vim.api.nvim_create_autocmd({"BufEnter", "WinEnter"}, {
				group = auto_save_group,
				callback = function()
					-- Check if we're entering a different buffer/window from Claude terminal
					local current_buf = vim.api.nvim_get_current_buf()
					if state.terminal_bufnr and 
					   vim.api.nvim_buf_is_valid(state.terminal_bufnr) and 
					   current_buf ~= state.terminal_bufnr then
						auto_save_session()
					end
				end,
			})
			
			-- Auto-save when entering file explorer windows
			vim.api.nvim_create_autocmd({"FileType"}, {
				pattern = {"NvimTree", "nerdtree", "neo-tree", "TelescopePrompt", "fzf"},
				group = explorer_save_group,
				callback = function()
					if state.terminal_bufnr and vim.api.nvim_buf_is_valid(state.terminal_bufnr) then
						auto_save_session()
					end
				end,
			})
			
			-- Clean up autocmd groups when buffer is deleted
			vim.api.nvim_create_autocmd("BufDelete", {
				buffer = buf,
				callback = function()
					pcall(vim.api.nvim_del_augroup_by_name, "ClaudeCodeAutoSave_" .. buf)
					pcall(vim.api.nvim_del_augroup_by_name, "ClaudeCodeExplorerSave_" .. buf)
				end,
				once = true,
			})
		end
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
		-- Clear autocmd groups before deleting buffer
		pcall(vim.api.nvim_del_augroup_by_name, "ClaudeCodeAutoSave")
		pcall(vim.api.nvim_del_augroup_by_name, "ClaudeCodeExplorerSave")
		vim.api.nvim_buf_delete(buf, { force = true })
	end
	
	-- Reset state
	state.terminal_bufnr = nil
	state.current_session = {}
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

-- Track last auto-save time to prevent rapid repeated saves
-- Moved to before M.open function

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
	local commands = {"ship.md", "plan.md", "code.md", "research.md"}
	
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

-- Setup Claude agents with location choice
local function setup_claude_agents(location)
	-- Determine the target directory based on location choice
	local claude_agents_dir
	if location == "personal" then
		-- Personal level: ~/.claude/agents
		claude_agents_dir = vim.fn.expand("~") .. "/.claude/agents"
	else
		-- Project level: ./.claude/agents (default)
		local cwd = vim.fn.getcwd()
		claude_agents_dir = cwd .. "/.claude/agents"
	end
	
	-- Create agents directory if it doesn't exist
	if vim.fn.isdirectory(claude_agents_dir) == 0 then
		vim.fn.mkdir(claude_agents_dir, "p")
	end
	
	-- Get the plugin directory
	local plugin_dir = vim.fn.fnamemodify(debug.getinfo(1, "S").source:sub(2), ":h")
	local templates_dir = plugin_dir .. "/agents"
	
	-- List of agent templates to copy
	local agents = {"product-analyst.md", "software-engineer.md", "code-reviewer.md"}
	
	-- Copy each agent template to the target directory
	for _, agent_file in ipairs(agents) do
		local template_path = templates_dir .. "/" .. agent_file
		local target_path = claude_agents_dir .. "/" .. agent_file
		
		-- Check if template exists
		if vim.fn.filereadable(template_path) == 1 then
			-- Read template content
			local template_content = vim.fn.readfile(template_path)
			-- Write to target location
			vim.fn.writefile(template_content, target_path)
		else
			vim.notify("Warning: Agent template not found: " .. template_path, vim.log.levels.WARN)
		end
	end
	
	local location_desc = location == "personal" and "personal (~/.claude/agents)" or "project (.claude/agents)"
	vim.notify("Claude Code: Agents installed at " .. location_desc .. " level", vim.log.levels.INFO)
end



-- Functions moved before auto_save_session to fix dependency order

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
	
	-- Prepare metadata
	local metadata = {
		name = name,
		created_at = state.current_session.created_at or os.date("%Y-%m-%d %H:%M:%S"),
	}
	
	-- Format all content as markdown
	local formatted_lines = format_full_session_content(lines, metadata)
	
	-- Determine filepath
	local filepath
	if is_new or not state.current_session.filepath then
		-- Create new file
		local safe_name = sanitize_filename(name)
		local timestamp = os.date("%Y%m%d_%H%M%S")
		local filename = string.format("session_%s_%s.md", timestamp, safe_name)
		filepath = get_session_dir() .. filename
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
		vim.notify("Session saved: " .. filename .. " (" .. #lines .. " lines)", vim.log.levels.INFO)
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
	local session_files = vim.fn.glob(get_session_dir() .. "session_*.md", false, true)
	
	-- Sort sessions by date (newest first)
	table.sort(session_files, function(a, b) return a > b end)
	
	local sessions = {}
	for _, file in ipairs(session_files) do
		local basename = vim.fn.fnamemodify(file, ":t")
		local timestamp, custom_name = basename:match("session_(%d+_%d+)_?(.*)%.md")
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
	
	-- Command to manually install Claude commands
	vim.api.nvim_create_user_command("ClaudeCodeInstallCommands", function()
		setup_claude_commands()
		vim.notify("Claude Code: Custom commands installed", vim.log.levels.INFO)
	end, { desc = "Install Claude Code custom commands" })
	
	-- Command to manually install Claude agents
	vim.api.nvim_create_user_command("ClaudeCodeInstallAgents", function()
		-- Ask user for installation location preference
		vim.ui.select(
			{"Project level (.claude/agents)", "Personal level (~/.claude/agents)"},
			{
				prompt = "Where would you like to install Claude Code agents?",
				format_item = function(item) return item end,
			},
			function(choice)
				if choice == "Project level (.claude/agents)" then
					setup_claude_agents("project")
				elseif choice == "Personal level (~/.claude/agents)" then
					setup_claude_agents("personal")
				end
			end
		)
	end, { desc = "Install Claude Code agents (product-analyst, software-engineer, code-reviewer)" })
	
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