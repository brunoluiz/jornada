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
			const events = [{{ events }}];

			const replayer = new rrweb.Replayer(events);
			replayer.play();
		}).catch(console.err);
};
