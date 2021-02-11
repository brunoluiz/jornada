window.onload = function() {
	function injectScript(src) {
		return new Promise((resolve, reject) => {
			const script = document.createElement('script');
			script.src = src;
			script.addEventListener('load', resolve);
			script.addEventListener('error', e => reject(e.error));
			document.head.appendChild(script);
		});
	}

	const link = document.createElement("link");
	link.href = "https://cdn.jsdelivr.net/npm/rrweb@{{ rrweb_version }}/dist/rrweb.min.css";
	link.type = "text/css";
	link.rel = "stylesheet";
	document.getElementsByTagName("head")[0].appendChild(link);

	injectScript('https://cdn.jsdelivr.net/npm/rrweb@{{ rrweb_version }}/dist/rrweb.min.js')
		.then(() => {
			let events = [];

			rrweb.record({
				emit(event) {
					// push event into the events array
					events.push(event);
				},
			});

			// this function will send events to the backend and reset the events array
			function save() {
				const body = JSON.stringify({ events });
				events = [];
				fetch('{{ .URL }}/record', {
					method: 'POST',
					headers: {
						'Content-Type': 'application/json',
					},
					body,
				});
			}

			// save events every 10 seconds
			setInterval(save, 1 * 1000);
		}).catch(console.err);
};
