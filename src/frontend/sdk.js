/* Plugin SDK: loaded by plugin iframes to talk to the desktop host. */
(function () {
  const params = new URLSearchParams(location.search);
  const plugin = params.get('plugin') || '';
  const session = params.get('session') || 'default';
  let seq = 0;
  const pending = new Map();

  window.addEventListener('message', function (e) {
    const d = e.data;
    if (!d || d.__hlc !== true || d.type !== 'result') return;
    const p = pending.get(d.id);
    if (!p) return;
    pending.delete(d.id);
    if (d.ok) p.resolve(d.result);
    else p.reject(new Error(d.error || 'host call failed'));
  });

  window.Host = {
    plugin: plugin,
    session: session,

    call: function (method, args) {
      args = args || [];
      return new Promise(function (resolve, reject) {
        const id = ++seq;
        pending.set(id, { resolve: resolve, reject: reject });
        parent.postMessage({
          __hlc: true,
          type: 'call',
          id: id,
          plugin: plugin,
          session: session,
          method: method,
          args: args,
        }, '*');
      });
    },

    eventsURL: function () {
      return '/api/plugins/' + encodeURIComponent(plugin) +
        '/events?session=' + encodeURIComponent(session);
    },
  };
})();
