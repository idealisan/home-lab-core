-- PowerShell 控制台插件
-- 用于验证插件体系是否能容纳第二个、行为不同的控制台插件。

plugin = {
  id = "powershell",
  name = "PowerShell 控制台",
  version = "0.1.0",
  description = "通过持久 PowerShell 会话管理本机",
  icon = "PS",
  menu = "插件",
  dock = true,
  shell = "powershell",
  web = "web/index.html",
  window = { width = 800, height = 480 },
}

function on_install()
  host.log("info", "powershell: on_install - 首次安装")
end

function on_load()
  host.log("info", "powershell: on_load")
end

function on_first_run()
  host.log("info", "powershell: on_first_run - 首次打开界面")
end

function on_ui_show(session)
  host.shell_start(session.id)
  host.emit(session.id, "PowerShell 控制台已就绪。\r\n")
end

function on_ui_hide(session)
  host.log("info", "powershell: on_ui_hide " .. session.id)
end

function on_command(session, cmd)
  host.shell_write(session.id, cmd .. "\n")
end

function on_close(session)
  host.shell_close(session.id)
  host.log("info", "powershell: on_close " .. session.id)
end
