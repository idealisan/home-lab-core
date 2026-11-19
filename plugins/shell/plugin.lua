-- Shell 控制台插件（样本）
--
-- 生命周期说明：
--   on_install()        插件首次被安装（每个插件只执行一次）
--   on_load()           插件被载入 Lua VM（每次启动/重载都执行）
--   on_first_run()      用户第一次打开该插件的界面
--   on_ui_show(session) 窗口显示 / 从最小化恢复
--   on_ui_hide(session) 窗口隐藏 / 最小化
--   on_close(session)   窗口关闭，会话结束
--
-- 注意：初始化是一个过程，不是一个时机，所以这里不用 init()。

plugin = {
  id = "shell",
  name = "Shell 控制台",
  version = "0.1.0",
  description = "基于持久 shell 会话的命令行控制台样本插件",
  icon = ">_",
  menu = "插件",
  dock = true,
  web = "index.html",
  window = { width = 760, height = 460 },
}

function on_install()
  host.log("info", "shell: on_install - 首次安装")
end

function on_load()
  host.log("info", "shell: on_load")
end

function on_first_run()
  host.log("info", "shell: on_first_run - 首次打开界面")
end

function on_ui_show(session)
  host.shell_start(session.id)
  host.emit(session.id, "Shell 控制台已就绪，输入命令并回车执行。\r\n")
end

function on_ui_hide(session)
  host.log("info", "shell: on_ui_hide " .. session.id)
end

function on_command(session, cmd)
  host.shell_write(session.id, cmd .. "\n")
end

function on_close(session)
  host.shell_close(session.id)
  host.log("info", "shell: on_close " .. session.id)
end
