(function () {
  const out = document.getElementById('output');
  const input = document.getElementById('cmd');
  document.getElementById('session-label').textContent = Host.session;

  // Strip ANSI escape sequences for this first plain-text version.
  function stripAnsi(s) {
    return s
      .replace(/\x1B\[[0-9;]*[A-Za-z]/g, '')
      .replace(/\x1B\][^\x07]*\x07/g, '');
  }

  function append(text) {
    out.textContent += stripAnsi(text);
    out.scrollTop = out.scrollHeight;
  }

  const stream = new EventSource(Host.eventsURL());
  stream.onmessage = function (e) {
    try {
      append(JSON.parse(e.data).chunk || '');
    } catch (_) { /* ignore malformed events */ }
  };
  stream.onerror = function () {
    append('\n[输出连接中断，正在重连…]\n');
  };

  input.addEventListener('keydown', async function (e) {
    if (e.key !== 'Enter') return;
    const cmd = input.value;
    input.value = '';
    if (!cmd.trim()) {
      append('$ ');
      return;
    }
    append('$ ' + cmd + '\n');
    try {
      await Host.call('on_command', [cmd]);
    } catch (err) {
      append('[调用失败] ' + err.message + '\n');
    }
  });

  input.focus();
})();
