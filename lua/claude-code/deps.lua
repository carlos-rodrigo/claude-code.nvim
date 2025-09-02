-- claude-code.nvim Dependency Manager
-- Handles automatic installation of required dependencies

local M = {}

-- Check if a plugin is installed
local function is_plugin_installed(plugin_name)
  -- Check common plugin installation paths
  local paths = {
    vim.fn.stdpath("data") .. "/lazy/" .. plugin_name,
    vim.fn.stdpath("data") .. "/site/pack/*/start/" .. plugin_name,
    vim.fn.stdpath("data") .. "/site/pack/*/opt/" .. plugin_name,
    vim.fn.stdpath("config") .. "/pack/*/start/" .. plugin_name,
    vim.fn.stdpath("config") .. "/pack/*/opt/" .. plugin_name,
  }
  
  -- Also check if already loaded via pcall
  local ok = pcall(require, plugin_name:match("([^/]+)$"):gsub("%.nvim$", ""))
  if ok then return true end
  
  -- Check filesystem paths
  for _, path in ipairs(paths) do
    if vim.fn.isdirectory(vim.fn.expand(path)) == 1 then
      return true
    end
  end
  
  return false
end

-- Detect which plugin manager is being used
local function detect_plugin_manager()
  -- Check for Lazy.nvim
  if pcall(require, "lazy") then
    return "lazy"
  end
  
  -- Check for Packer
  if pcall(require, "packer") then
    return "packer"
  end
  
  -- Check for vim-plug
  if vim.fn.exists("*plug#begin") == 1 then
    return "vim-plug"
  end
  
  -- Check for paq-nvim
  if pcall(require, "paq") then
    return "paq"
  end
  
  return nil
end

-- Install plenary using the detected plugin manager
local function install_with_lazy()
  local ok, lazy = pcall(require, "lazy")
  if not ok then return false end
  
  -- Add plenary to lazy's spec
  lazy.setup({
    { "nvim-lua/plenary.nvim" }
  }, {
    -- Merge with existing config
    install = { missing = true },
  })
  
  -- Trigger installation
  vim.cmd("Lazy install plenary.nvim")
  return true
end

local function install_with_packer()
  local ok, packer = pcall(require, "packer")
  if not ok then return false end
  
  packer.use("nvim-lua/plenary.nvim")
  vim.cmd("PackerSync")
  return true
end

local function install_with_paq()
  local ok, paq = pcall(require, "paq")
  if not ok then return false end
  
  paq({ "nvim-lua/plenary.nvim" })
  vim.cmd("PaqInstall")
  return true
end

-- Manual installation using git
local function install_manually()
  local install_path = vim.fn.stdpath("data") .. "/site/pack/claude-code-deps/start/plenary.nvim"
  
  if vim.fn.isdirectory(install_path) == 0 then
    vim.notify("Claude Code: Installing plenary.nvim dependency...", vim.log.levels.INFO)
    
    local success = vim.fn.system({
      "git", "clone", "--depth", "1",
      "https://github.com/nvim-lua/plenary.nvim",
      install_path
    })
    
    if vim.v.shell_error ~= 0 then
      vim.notify("Claude Code: Failed to install plenary.nvim. Please install it manually.", vim.log.levels.ERROR)
      return false
    end
    
    vim.notify("Claude Code: plenary.nvim installed successfully. Please restart Neovim.", vim.log.levels.INFO)
    
    -- Add to runtimepath immediately
    vim.cmd("packadd plenary.nvim")
    return true
  end
  
  return true
end

-- Main function to ensure dependencies are installed
function M.ensure_dependencies()
  -- Check if plenary is already installed
  if is_plugin_installed("plenary.nvim") then
    return true
  end
  
  -- Detect plugin manager and try to install
  local plugin_manager = detect_plugin_manager()
  
  if plugin_manager == "lazy" then
    -- For Lazy.nvim users, just notify them to add the dependency
    vim.notify("Claude Code: Please add 'nvim-lua/plenary.nvim' to your plugin dependencies in your Lazy config.", vim.log.levels.WARN)
    vim.notify("Example: dependencies = { 'nvim-lua/plenary.nvim' }", vim.log.levels.INFO)
    return false
  elseif plugin_manager == "packer" then
    return install_with_packer()
  elseif plugin_manager == "paq" then
    return install_with_paq()
  else
    -- Try manual installation
    return install_manually()
  end
end

-- Check specific dependency
function M.check_dependency(name)
  if name == "plenary" or name == "plenary.nvim" then
    return is_plugin_installed("plenary.nvim")
  end
  return false
end

-- Get installation instructions for missing dependencies
function M.get_install_instructions(dependency)
  local plugin_manager = detect_plugin_manager()
  
  local instructions = {
    ["plenary.nvim"] = {
      lazy = "dependencies = { 'nvim-lua/plenary.nvim' }",
      packer = "use 'nvim-lua/plenary.nvim'",
      ["vim-plug"] = "Plug 'nvim-lua/plenary.nvim'",
      paq = "require('paq')({ 'nvim-lua/plenary.nvim' })",
      manual = "git clone https://github.com/nvim-lua/plenary.nvim ~/.local/share/nvim/site/pack/claude-code-deps/start/plenary.nvim"
    }
  }
  
  if instructions[dependency] then
    if plugin_manager and instructions[dependency][plugin_manager] then
      return instructions[dependency][plugin_manager]
    else
      return instructions[dependency].manual
    end
  end
  
  return nil
end

return M