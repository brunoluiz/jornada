window.recorder = {
  events: [],
  rrweb: undefined,
  runner: undefined,
  clientId: 'default',
  user: {},
  session: {
    synced: false,
    get() {
      const session = window.sessionStorage.getItem('rrweb');
      const out = session ? JSON.parse(session) : {
        user: window.recorder.user,
        clientId: window.recorder.clientId,
      };
      window.recorder = Object.assign({}, window.recorder, out);
      return out;
    },
    save(data) {
      const session = window.recorder.session.get();
      window.sessionStorage.setItem('rrweb', JSON.stringify(Object.assign({}, session, data)));
      window.recorder.session.synced = false;

      return window.recorder.session
    },
    clear() {
      window.sessionStorage.removeItem('rrweb')
    },
    sync() {
      if (window.recorder.session.synced) return;

      return fetch('{{ .URL }}/api/v1/sessions', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(window.recorder.session.get()),
      }).then(res => {
        window.recorder.session.synced = true;
        return res.json();
      })
    }
  },
  setUser: function({ id, email, name }) {
    const session = window.recorder.session.get();
    session.user = { id, email, name };
    window.recorder.session.save(session)
    window.recorder.user = session.user;

    return window.recorder;
  },
  setMeta: function(meta = {}) {
    const session = window.recorder.session.get();
    session.meta = meta;
    window.recorder.session.save(session)
    window.recorder.meta = session.meta;

    return window.recorder;
  },
  setClientId(id) {
    const session = window.recorder.session.get();
    session.clientId = id;
    window.recorder.session.save(session)
    window.recorder.clientId = session.clientId;

    return window.recorder;
  },
  sync() {
    if (!window.recorder.events.length) return;

    const session = window.recorder.session.get();
    fetch('{{ .URL }}/api/v1/sessions/' + session.id + '/events', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(window.recorder.events),
    });
    window.recorder.events = []; // cleans-up events for next cycle
  },
  start() {
    if (window.recorder.runner) return;

    window.recorder.runner = setInterval(function save() {
      window.recorder.session.sync();
      window.recorder.sync();
    }, 1000);
  },
  close() {
    clearInterval();
    window.recorder.session.clear();
    window.recorder.rrwebStop();
  }
};

new Promise((resolve, reject) => {
  const script = document.createElement('script');
  script.src = 'https://cdn.jsdelivr.net/npm/rrweb@0.9.14/dist/rrweb.min.js';
  script.addEventListener('load', resolve);
  script.addEventListener('error', e => reject(e.error));
  document.head.appendChild(script);
}).then(() => {
  window.recorder.rrweb = rrweb;
  // TODO: This should be optimised 🤠
  window.recorder.rrwebStop = rrweb.record({
    emit(event) {
      window.recorder.events.push(event);
    },
    collectFonts: true,
    slimDOMOptions: {
      script: true,
      comment: true,
      headFavicon: true,
      headWhitespace: true,
      headMetaDescKeywords: true,
      headMetaSocial: true,
      headMetaRobots: true,
      headMetaHttpEquiv: true,
      headMetaAuthorship: true,
      headMetaVerification: true,
    },
    // sampling: {
    //   mousemove: true,
    //   mouseInteraction: false,
    //   scroll: 150,
    //   input: 'last',
    // },
  });

  return window.recorder.session.sync();
}).then(res => {
  window.recorder.session.save({ id: res.id });
  window.recorder.start();
})
.catch(console.err);