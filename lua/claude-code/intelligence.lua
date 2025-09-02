-- claude-code.nvim Intelligence Service Client
-- Communicates with the Go intelligence service for AI features

local M = {}
local notify = require("claude-code.notify")
local deps = require("claude-code.deps")

-- Check and ensure plenary dependency
local has_plenary = deps.check_dependency("plenary.nvim")
if not has_plenary then
  -- Try to auto-install
  has_plenary = deps.ensure_dependencies()
  if not has_plenary then
    notify.warn("Intelligence module requires plenary.nvim. Attempting to install...")
    local instructions = deps.get_install_instructions("plenary.nvim")
    if instructions then
      notify.info("To enable intelligence features, add to your config: " .. instructions)
    end
    return M
  end
end

-- Load plenary curl module
local curl = require("plenary.curl")

-- Default configuration
M.config = {
  service_url = "http://localhost:7345",
  timeout = 30000,
  enabled = false,
  auto_compress = false,
  compression_threshold_kb = 100,
}

-- Check if service is available
function M.check_service()
  local ok, response = pcall(curl.get, {
    url = M.config.service_url .. "/health",
    timeout = 5000,
  })
  
  if not ok or not response or response.status ~= 200 then
    return false
  end
  
  local data = vim.fn.json_decode(response.body)
  return data and data.status == "healthy"
end

-- Initialize the intelligence client
function M.setup(config)
  M.config = vim.tbl_deep_extend("force", M.config, config or {})
  
  -- Check if service is available
  if M.config.enabled then
    vim.defer_fn(function()
      if M.check_service() then
        notify.info("Intelligence service connected at " .. M.config.service_url)
      else
        notify.warn("Intelligence service not available. AI features disabled.")
        notify.warn("Start with: cd claude-code-intelligence && make dev")
        M.config.enabled = false
      end
    end, 100)
  end
end

-- Compress a session using AI
function M.compress_session(session_path, callback)
  if not M.config.enabled then
    callback(nil, "Intelligence service not enabled")
    return
  end
  
  -- Read session content
  local file = io.open(session_path, "r")
  if not file then
    callback(nil, "Failed to read session file")
    return
  end
  local content = file:read("*all")
  file:close()
  
  -- Check size threshold
  local size_kb = #content / 1024
  if size_kb < M.config.compression_threshold_kb then
    callback(nil, "Session too small for compression")
    return
  end
  
  -- Send compression request
  local ok, response = pcall(curl.post, {
    url = M.config.service_url .. "/api/v1/sessions/compress",
    headers = { ["Content-Type"] = "application/json" },
    body = vim.fn.json_encode({
      content = content,
      options = {
        style = "balanced",
        max_length = 2000,
        priority = "balanced",
      }
    }),
    timeout = M.config.timeout,
  })
  
  if not ok or not response or response.status ~= 200 then
    callback(nil, "Compression request failed")
    return
  end
  
  local result = vim.fn.json_decode(response.body)
  if result and result.summary then
    -- Save compressed version
    local compressed_path = session_path:gsub("%.md$", ".compressed.md")
    local compressed_file = io.open(compressed_path, "w")
    if compressed_file then
      compressed_file:write("# Compressed Session\n\n")
      compressed_file:write("Original Size: " .. string.format("%.1f KB\n", size_kb))
      compressed_file:write("Compressed Size: " .. string.format("%.1f KB\n", #result.summary / 1024))
      compressed_file:write("Compression Ratio: " .. string.format("%.1f%%\n", (1 - result.compression_ratio) * 100))
      compressed_file:write("Model: " .. result.model .. "\n")
      compressed_file:write("Processing Time: " .. result.processing_time .. "\n\n")
      compressed_file:write("## Summary\n\n")
      compressed_file:write(result.summary)
      compressed_file:close()
      
      notify.info(string.format("Session compressed: %.1f%% reduction", (1 - result.compression_ratio) * 100))
      callback(compressed_path, nil)
    else
      callback(nil, "Failed to save compressed session")
    end
  else
    callback(nil, "Invalid compression response")
  end
end

-- Search sessions using semantic search
function M.search_sessions(query, callback)
  if not M.config.enabled then
    callback(nil, "Intelligence service not enabled")
    return
  end
  
  local ok, response = pcall(curl.post, {
    url = M.config.service_url .. "/api/v1/sessions/search",
    headers = { ["Content-Type"] = "application/json" },
    body = vim.fn.json_encode({
      query = query,
      limit = 10,
    }),
    timeout = M.config.timeout,
  })
  
  if not ok or not response or response.status ~= 200 then
    callback(nil, "Search request failed")
    return
  end
  
  local result = vim.fn.json_decode(response.body)
  callback(result.results, nil)
end

-- Test different models with sample content
function M.test_models(content, models, callback)
  if not M.config.enabled then
    callback(nil, "Intelligence service not enabled")
    return
  end
  
  local ok, response = pcall(curl.post, {
    url = M.config.service_url .. "/api/v1/ai/test-models",
    headers = { ["Content-Type"] = "application/json" },
    body = vim.fn.json_encode({
      content = content,
      models = models,
    }),
    timeout = M.config.timeout * 3, -- Longer timeout for testing
  })
  
  if not ok or not response or response.status ~= 200 then
    callback(nil, "Model testing failed")
    return
  end
  
  local result = vim.fn.json_decode(response.body)
  callback(result.results, nil)
end

-- Get service statistics
function M.get_stats(callback)
  if not M.config.enabled then
    callback(nil, "Intelligence service not enabled")
    return
  end
  
  local ok, response = pcall(curl.get, {
    url = M.config.service_url .. "/api/v1/info/stats",
    timeout = M.config.timeout,
  })
  
  if not ok or not response or response.status ~= 200 then
    callback(nil, "Failed to get stats")
    return
  end
  
  local stats = vim.fn.json_decode(response.body)
  callback(stats, nil)
end

-- Commands for user interaction
function M.register_commands()
  -- Compress current session
  vim.api.nvim_create_user_command("ClaudeCompressSession", function()
    local session_path = vim.fn.expand("%:p")
    if not session_path:match("%.md$") then
      notify.error("Not a markdown session file")
      return
    end
    
    notify.info("Compressing session...")
    M.compress_session(session_path, function(compressed_path, err)
      if err then
        notify.error("Compression failed: " .. err)
      else
        notify.success("Session compressed to: " .. compressed_path)
        -- Optionally open the compressed file
        vim.cmd("vsplit " .. compressed_path)
      end
    end)
  end, { desc = "Compress current session using AI" })
  
  -- Search sessions
  vim.api.nvim_create_user_command("ClaudeSearchSessions", function(opts)
    M.search_sessions(opts.args, function(results, err)
      if err then
        notify.error("Search failed: " .. err)
        return
      end
      
      if not results or #results == 0 then
        notify.warn("No results found")
        return
      end
      
      -- Show results in quickfix
      local qf_items = {}
      for _, result in ipairs(results) do
        table.insert(qf_items, {
          filename = result.session_name,
          text = result.content_preview or result.summary or "",
          pattern = result.session_id,
        })
      end
      
      vim.fn.setqflist(qf_items)
      vim.cmd("copen")
      notify.info("Found " .. #results .. " matching sessions")
    end)
  end, {
    nargs = 1,
    desc = "Search sessions using AI",
  })
  
  -- Test models
  vim.api.nvim_create_user_command("ClaudeTestModels", function()
    local content = vim.api.nvim_buf_get_lines(0, 0, 100, false)
    content = table.concat(content, "\n")
    
    if #content < 100 then
      notify.error("Need more content to test models (at least 100 chars)")
      return
    end
    
    notify.info("Testing models...")
    M.test_models(content, nil, function(results, err)
      if err then
        notify.error("Testing failed: " .. err)
        return
      end
      
      -- Display results
      local lines = { "# Model Test Results", "" }
      for _, result in ipairs(results) do
        table.insert(lines, "## " .. result.model)
        table.insert(lines, string.format("- Success: %s", result.success and "✓" or "✗"))
        if result.success then
          table.insert(lines, string.format("- Processing Time: %s", result.processing_time))
          table.insert(lines, string.format("- Compression Ratio: %.2f", result.compression_ratio))
          table.insert(lines, string.format("- Quality Score: %.1f/10", result.quality_score))
        else
          table.insert(lines, string.format("- Error: %s", result.error or "Unknown"))
        end
        table.insert(lines, "")
      end
      
      -- Create new buffer with results
      vim.cmd("new")
      vim.api.nvim_buf_set_lines(0, 0, -1, false, lines)
      vim.bo.filetype = "markdown"
      vim.bo.buftype = "nofile"
      vim.bo.modifiable = false
    end)
  end, { desc = "Test AI models with current buffer content" })
  
  -- Show stats
  vim.api.nvim_create_user_command("ClaudeStats", function()
    M.get_stats(function(stats, err)
      if err then
        notify.error("Failed to get stats: " .. err)
        return
      end
      
      local lines = { "# Claude Intelligence Service Stats", "" }
      
      -- Service stats
      table.insert(lines, "## Service")
      table.insert(lines, "- Uptime: " .. (stats.service and stats.service.uptime or "N/A"))
      table.insert(lines, "- Version: " .. (stats.service and stats.service.version or "N/A"))
      table.insert(lines, "")
      
      -- Database stats
      table.insert(lines, "## Database")
      if stats.database then
        table.insert(lines, "- Total Sessions: " .. (stats.database.total_sessions or 0))
        table.insert(lines, "- Compressed Sessions: " .. (stats.database.compressed_sessions or 0))
        table.insert(lines, string.format("- Avg Compression: %.1f%%", 
          (1 - (stats.database.avg_compression_ratio or 0.5)) * 100))
      end
      table.insert(lines, "")
      
      -- Model performance
      table.insert(lines, "## Model Performance")
      if stats.model_performance then
        for _, perf in ipairs(stats.model_performance) do
          table.insert(lines, string.format("- %s: %.1f%% success, %.1fms avg", 
            perf.model_name, perf.success_rate, perf.avg_processing_time_ms))
        end
      end
      
      -- Create new buffer with stats
      vim.cmd("new")
      vim.api.nvim_buf_set_lines(0, 0, -1, false, lines)
      vim.bo.filetype = "markdown"
      vim.bo.buftype = "nofile"
      vim.bo.modifiable = false
    end)
  end, { desc = "Show intelligence service statistics" })
end

return M