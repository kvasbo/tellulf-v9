// Sends a line to the server log, so what happens on the kiosk overnight
// (sleep, reloads, WebGL trouble) can be read from the container's log.
export function report(msg) {
	fetch('/log', { method: 'POST', body: String(msg), keepalive: true }).catch(
		() => {},
	);
}
