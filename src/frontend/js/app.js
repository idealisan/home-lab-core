/* Home Lab Core — basic macOS-like desktop shell. */

const desktop = document.getElementById('desktop');
const windowsLayer = document.getElementById('windows');
const dock = document.getElementById('dock');
const pluginMenu = document.getElementById('plugin-menu');
const clock = document.getElementById('clock');

let zIndex = 10;
let cascade = 0;
const openWindows = new Map();

/* ---------- Clock ---------- */
function updateClock() {
  const now = new Date();
  const days = ['周日', '周一', '周二', '周三', '周四', '周五', '周六'];
  const pad = (n) => String(n).padStart(2, '0');
  clock.textContent = `${now.getMonth() + 1}月${now.getDate()}日 ${days[now.getDay()]} ${pad(now.getHours())}:${pad(now.getMinutes())}`;
}
updateClock();
setInterval(updateClock, 10000);

/* ---------- Window management ---------- */
function clamp(value, min, max) {
  return Math.max(min, Math.min(value, max));
}

function focusWindow(win) {
  win.style.zIndex = String(++zIndex);
  document.querySelectorAll('.dock-item.running').forEach((el) => el.classList.remove('running'));
  const item = dock.querySelector(`[data-app="${win.dataset.app}"]`);
  if (item) item.classList.add('running');
}

function setWindowRect(win, rect) {
  win.style.left = `${rect.x}px`;
  win.style.top = `${rect.y}px`;
  win.style.width = `${rect.width}px`;
  win.style.height = `${rect.height}px`;
}

function createWindow(config) {
  const win = document.createElement('section');
  win.className = 'window';
  win.dataset.app = config.id || '';
  win.dataset.title = config.title;

  const bounds = desktop.getBoundingClientRect();
  const width = Math.min(config.width || 460, bounds.width - 24);
  const height = Math.min(config.height || 320, bounds.height - 24);
  const offset = (cascade % 6) * 26;
  cascade += 1;
  const x = config.x ?? clamp(Math.round((bounds.width - width) / 2) - 120 + offset, 12, bounds.width - width - 12);
  const y = config.y ?? clamp(40 + offset, 12, bounds.height - height - 12);

  win.innerHTML = `
    <div class="window-titlebar">
      <div class="traffic-lights">
        <button class="light close" title="关闭" aria-label="关闭"></button>
        <button class="light minimize" title="最小化" aria-label="最小化"></button>
        <button class="light maximize" title="最大化" aria-label="最大化"></button>
      </div>
      <div class="window-title"></div>
    </div>
    <div class="window-body"></div>
  `;
  win.querySelector('.window-title').textContent = config.title;
  win.querySelector('.window-body').innerHTML = config.html || '';
  windowsLayer.appendChild(win);
  setWindowRect(win, { x, y, width, height });
  focusWindow(win);

  let restoreRect = { x, y, width, height };

  win.querySelector('.light.close').addEventListener('click', () => {
    win.remove();
    openWindows.delete(config.id);
  });
  win.querySelector('.light.minimize').addEventListener('click', () => {
    win.classList.toggle('minimized');
  });
  win.querySelector('.light.maximize').addEventListener('click', () => {
    const b = desktop.getBoundingClientRect();
    if (win.dataset.maximized === 'true') {
      setWindowRect(win, restoreRect);
      win.dataset.maximized = 'false';
    } else {
      restoreRect = { x: win.offsetLeft, y: win.offsetTop, width: win.offsetWidth, height: win.offsetHeight };
      setWindowRect(win, { x: 8, y: 8, width: b.width - 16, height: b.height - 16 });
      win.dataset.maximized = 'true';
    }
    focusWindow(win);
  });

  enableDragging(win);
  return win;
}

function openWindow(config) {
  const existing = openWindows.get(config.id);
  if (existing && document.contains(existing)) {
    existing.classList.remove('minimized');
    focusWindow(existing);
    return existing;
  }
  const win = createWindow(config);
  openWindows.set(config.id, win);
  return win;
}

function enableDragging(win) {
  const bar = win.querySelector('.window-titlebar');
  bar.addEventListener('pointerdown', (event) => {
    if (event.target.closest('.light')) return;
    if (win.dataset.maximized === 'true') return;
    focusWindow(win);

    const startX = event.clientX;
    const startY = event.clientY;
    const originX = win.offsetLeft;
    const originY = win.offsetTop;
    bar.setPointerCapture(event.pointerId);

    const onMove = (e) => {
      const bounds = desktop.getBoundingClientRect();
      const x = clamp(originX + e.clientX - startX, 0, bounds.width - 40);
      const y = clamp(originY + e.clientY - startY, 0, bounds.height - 38);
      win.style.left = `${x}px`;
      win.style.top = `${y}px`;
    };
    const onUp = () => {
      bar.removeEventListener('pointermove', onMove);
      bar.removeEventListener('pointerup', onUp);
      bar.removeEventListener('pointercancel', onUp);
    };
    bar.addEventListener('pointermove', onMove);
    bar.addEventListener('pointerup', onUp);
    bar.addEventListener('pointercancel', onUp);
  });

  win.addEventListener('pointerdown', () => focusWindow(win));
}

/* ---------- Demo applications ---------- */
const apps = [
  {
    id: 'about',
    label: '关于本机',
    icon: '🖥️',
    title: '关于本机',
    width: 440,
    height: 300,
    html: `
      <h2>Home Lab Core</h2>
      <p class="muted">基于 Go 的 Linux 管理 Web 面板 · 基础版本</p>
      <hr>
      <p><strong>后端：</strong>Go HTTP Server</p>
      <p><strong>前端：</strong>原生 HTML / CSS / JavaScript</p>
      <p><strong>插件体系：</strong>Lua（规划中）</p>
      <p class="muted">目标：通过 GUI 替代控制台 Shell 管理 Linux，并以 Lua 插件轻松接入外部组件。</p>
    `,
  },
  {
    id: 'files',
    label: '文件管理',
    icon: '📁',
    title: '文件管理',
    width: 520,
    height: 360,
    html: `<h2>文件管理</h2><p class="muted">用于浏览与管理文件的窗口占位。后续将提供可视化的文件系统操作，替代命令行。</p>`,
  },
  {
    id: 'settings',
    label: '系统设置',
    icon: '⚙️',
    title: '系统设置',
    width: 480,
    height: 340,
    html: `<h2>系统设置</h2><p class="muted">系统配置界面占位。这里将提供图形化的 Linux 系统设置能力。</p>`,
  },
  {
    id: 'plugins',
    label: '插件中心',
    icon: '🧩',
    title: '插件中心',
    width: 520,
    height: 360,
    html: `
      <h2>插件中心</h2>
      <p>插件体系将以 <strong>Lua</strong> 为基础，外部组件可以作为插件接入并作为功能提供。</p>
      <p class="muted">插件列表将在后续版本通过 <code>/api/plugins</code> 加载，并显示在顶部菜单栏。</p>
    `,
  },
  {
    id: 'monitor',
    label: '系统监控',
    icon: '📈',
    title: '系统监控',
    width: 500,
    height: 340,
    html: `<h2>系统监控</h2><p class="muted">CPU、内存、磁盘与网络等可视化监控占位。</p>`,
  },
];

/* ---------- Dock ---------- */
apps.forEach((app) => {
  const item = document.createElement('button');
  item.className = 'dock-item';
  item.dataset.app = app.id;
  item.dataset.label = app.label;
  item.setAttribute('aria-label', app.label);
  item.textContent = app.icon;
  const dot = document.createElement('span');
  dot.className = 'dock-dot';
  item.appendChild(dot);
  item.addEventListener('click', () => openWindow(app));
  dock.appendChild(item);
});

/* ---------- Desktop double click ---------- */
desktop.addEventListener('dblclick', (event) => {
  if (event.target.closest('.window')) return;
  openWindow({
    id: `window-${Date.now()}`,
    label: '新窗口',
    title: '新窗口',
    width: 420,
    height: 280,
    html: `<h2>新窗口</h2><p class="muted">这是一个占位窗口。双击桌面空白处可以继续新建。</p>`,
  });
});

/* ---------- Menu bar interactions ---------- */
document.querySelectorAll('.menu-item').forEach((item) => {
  item.addEventListener('click', () => {
    document.querySelectorAll('.menu-item').forEach((el) => el.classList.remove('active'));
    item.classList.add('active');
    setTimeout(() => item.classList.remove('active'), 200);
    if (item.dataset.menu === 'plugins') openWindow(apps.find((a) => a.id === 'plugins'));
    if (item.dataset.menu === 'help') openWindow(apps.find((a) => a.id === 'about'));
  });
});

/* ---------- Load plugin menu entries (reserved for Lua plugins) ---------- */
async function loadPluginEntries() {
  try {
    const response = await fetch('/api/plugins');
    const entries = await response.json();
    entries.forEach((entry) => {
      const button = document.createElement('button');
      button.className = 'plugin-entry';
      button.textContent = entry.name || entry.id;
      button.addEventListener('click', () =>
        openWindow({
          id: `plugin-${entry.id}`,
          title: entry.name || entry.id,
          html: `<h2>${entry.name || entry.id}</h2><p class="muted">插件功能占位。</p>`,
        })
      );
      pluginMenu.appendChild(button);
    });
  } catch (err) {
    console.warn('load plugin entries failed:', err);
  }
}
loadPluginEntries();

/* Open a couple of windows so the desktop is populated on first load. */
openWindow(apps[0]);
openWindow(apps[3]);
