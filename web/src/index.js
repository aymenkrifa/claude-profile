// The front door for claudeprofile.aymenkrifa.com.
//
// install.sh lives in the repository and nowhere else. This proxies the raw
// file rather than keeping a copy of it, so there is one source of truth and
// fixing the installer needs no deploy here -- which also means this Worker
// cannot drift into serving a different script from the one people can read
// on GitHub.

const REPO = "aymenkrifa/claude-profile";
const RAW = `https://raw.githubusercontent.com/${REPO}/main/install.sh`;
const HOME = `https://github.com/${REPO}`;
// Spelled out rather than taken from the request, so the hint can never come
// back naming an http:// URL for a script someone is about to pipe to a shell.
const SELF = "https://claudeprofile.aymenkrifa.com/install.sh";

export default {
	async fetch(request) {
		const url = new URL(request.url);

		if (request.method !== "GET" && request.method !== "HEAD") {
			return text("method not allowed\n", 405, { allow: "GET, HEAD" });
		}

		switch (url.pathname) {
			case "/install.sh":
				return serveInstaller(request);
			case "/":
				return Response.redirect(HOME, 302);
			default:
				return text(`not found\n\nthe installer is at ${SELF}\n${HOME}\n`, 404);
		}
	},
};

async function serveInstaller(request) {
	let upstream;
	try {
		upstream = await fetch(RAW, {
			// Long enough to absorb a burst, short enough that a fix to the
			// installer is live within minutes.
			cf: { cacheTtl: 300, cacheEverything: true },
			headers: { "user-agent": "claudeprofile.aymenkrifa.com" },
		});
	} catch (err) {
		return text(`could not reach GitHub: ${err}\ntry ${RAW} directly.\n`, 502);
	}
	if (!upstream.ok) {
		return text(
			`could not fetch the installer (GitHub returned ${upstream.status}).\n` +
				`try ${RAW} directly.\n`,
			502,
		);
	}
	return new Response(request.method === "HEAD" ? null : upstream.body, {
		status: 200,
		headers: {
			// text/plain, not text/x-shellscript: this is a script people are
			// invited to pipe into a shell, so opening the URL in a browser to
			// read it first has to actually show it.
			"content-type": "text/plain; charset=utf-8",
			"cache-control": "public, max-age=300",
			"x-content-type-options": "nosniff",
		},
	});
}

function text(body, status, headers = {}) {
	return new Response(body, {
		status,
		headers: { "content-type": "text/plain; charset=utf-8", ...headers },
	});
}
