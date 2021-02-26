package view

// JSRecorder exports a template to be used as a recording script by the user application
const JSRecorder = `
window.recorder = {
	events: [],
	rrweb: undefined,
	runner: undefined,
	session: {
		synced: false,
		get() {
			const session = window.sessionStorage.getItem('rrweb');
			return session ? JSON.parse(session) : {
				user: {},
				clientId: 'default'
			};
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

		return window.recorder;
	},
	setMeta: function(meta = {}) {
		const session = window.recorder.session.get();
		session.meta = meta;
		window.recorder.session.save(session)

		return window.recorder;
	},
	setClientId(id) {
		const session = window.recorder.session.get();
		session.clientId = id;
		window.recorder.session.save(session)

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
		// slimDOMOptions: {
		//   script: false,
		//   comment: false,
		//   headFavicon: false,
		//   headWhitespace: false,
		//   headMetaDescKeywords: false,
		//   headMetaSocial: false,
		//   headMetaRobots: false,
		//   headMetaHttpEquiv: false,
		//   headMetaAuthorship: false,
		//   headMetaVerification: false,
		// },
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
.catch(console.err);`

// HTMLSessionByID template for the session informations and player
const HTMLSessionByID = `
<html>
	<head>
		<meta charset="utf-8"/>
		<meta name="viewport" content="width=device-width, initial-scale=1.0, minimum-scale=1.0, maximum-scale=2.0, user-scalable=yes" />
		<title>Play | Jornada</title>
		<link href="https://cdn.jsdelivr.net/npm/bootstrap@5.0.0-beta2/dist/css/bootstrap.min.css" rel="stylesheet" integrity="sha384-BmbxuPwQa2lc/FVzBcNJ7UAyJxM6wuqIj61tLrc4wSX0szH/Ev+nYRRuWlolflfl" crossorigin="anonymous" />
		<link href="https://cdn.jsdelivr.net/npm/rrweb-player@latest/dist/style.css" rel="stylesheet" />
	</head>
	<body>
		<div class="main container mt-3">
			<div class="row">
				<div class="col">
					<nav aria-label="breadcrumb">
						<ol class="breadcrumb">
							<li class="breadcrumb-item"><a href="/">Sessions</a></li>
							<li class="breadcrumb-item active" aria-current="page">Player</li>
						</ol>
					</nav>

					<h2 class="mb-3">Session re-play</h2>
					<div class="alert alert-warning" role="alert">
						Be aware this is just a proof of concept: the storage is not optimised, searching is not possible and it is not ready for production
					</div>
				</div>
			</div>
			<div class="row mb-3">
				<div class="col">
					<span class="badge bg-success">{{ .Session.OS }}</span>
					<span class="badge bg-primary">{{ .Session.Browser }} {{ .Session.Version }}</span>
					{{ range $k, $v := .Session.Meta }}
						<span class="badge bg-info">meta.{{ $k }} = '{{ $v }}'</span>
					{{ end }}
				</div>
			</div>
		</div>
		<div class="container mb-3" id="player">
		</div>
		<script type="application/javascript" src="https://cdn.jsdelivr.net/npm/rrweb-player@latest/dist/index.js" ></script>
		<script type="application/javascript" src="https://cdn.jsdelivr.net/npm/rrweb@0.9.14/dist/rrweb.min.js" ></script>
		<script type="application/javascript">
			fetch('/api/v1/sessions/{{ .ID }}/events', {
				method: 'GET',
			})
			.then(res => res.json())
			.then((res) => {
				new rrwebPlayer({
					target: document.getElementById("player"), // customizable root element
					props: {
						width: document.getElementById("player").offsetWidth,
						events: res,
					},
				});
			}).catch(console.error);
		</script>
	</body>
</html>
`

// HTMLSessionList template for the session list and for the service root (/)
const HTMLSessionList = `
<html>
	<head>
		<meta charset="utf-8"/>
		<meta name="viewport" content="width=device-width, initial-scale=1.0, minimum-scale=1.0, maximum-scale=2.0, user-scalable=yes" />
		<title>Sessions | Jornada</title>
		<link href="https://cdn.jsdelivr.net/npm/bootstrap@5.0.0-beta2/dist/css/bootstrap.min.css" rel="stylesheet" integrity="sha384-BmbxuPwQa2lc/FVzBcNJ7UAyJxM6wuqIj61tLrc4wSX0szH/Ev+nYRRuWlolflfl" crossorigin="anonymous">
	</head>
	<body>
		<div class="container mt-3">
			<nav aria-label="breadcrumb">
				<ol class="breadcrumb">
					<li class="breadcrumb-item active" aria-current="page">Sessions</li>
				</ol>
			</nav>
			<h2 class="mb-3">Sessions</h2>
			<div class="alert alert-warning" role="alert">
				Be aware this is just a proof of concept: the storage is not optimised, searching is not possible and it is not ready for production
			</div>

		<form action='/sessions' method='get'>
			<div class="input-group mb-3">
				<input type="text" class="form-control" placeholder="Query..." aria-label="Query" aria-describedby="button-addon2" name='q' value='{{ .Query }}'>
				<input type='submit' class="btn btn-primary" id="button-addon2" value='Search'/>
			</div>
		</form>

			<ul class="list-group mb-5">
			{{ range .Sessions }}
				<a href="/sessions/{{ .ID }}" class="list-group-item list-group-item-action">
					<div class="d-flex w-100 justify-content-between">
						{{ if .User.ID }}
						<h5 class="mb-2 mt-1"><span class="badge bg-dark">{{ .User.ID }}</span> {{ .User.Name }} </h5>
						{{ else }}
						<h5 class="mb-2 mt-1"><span class="badge bg-dark">Anonymous user</span></h5>
						{{ end }}
						<small class="text-muted">{{ .UpdatedAt.Format "Jan 02, 2006 15:04 UTC"  }}</small>
					</div>
					<p class="mb-1">
					<span class="badge bg-success">{{ .OS }}</span>
					<span class="badge bg-primary">{{ .Browser }} {{ .Version }}</span>
					{{ range $k, $v := .Meta }}
						<span class="badge bg-info">meta.{{ $k }} = '{{ $v }}'</span>
					{{ end }}
					</p>
				</a>
			{{ end }}
			</ul>

			<h2 class="mb-3">Start using</h2>
			<p>Insert the following snippet at the bottom of your <code>&lt;body&gt;</code> tag:</p>
			<pre>
&lt;script type=&quot;application/javascript&quot; src=&quot;{{ .URL }}/record.js&quot; &gt;&lt;/script&gt;
&lt;script type=&quot;application/javascript&quot;&gt;
window.recorder
  .setUser({id: 'USER_ID', email: 'test@test.com', name: 'Bruno Luiz Silva' })
  .setMeta({ foo: 'bar' })
  .setClientId('client-id')
&lt;/script&gt;
			</pre>
		</div>
	</body>
</html>
`
