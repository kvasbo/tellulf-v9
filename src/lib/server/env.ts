// Read a required environment variable, throwing a clear error if it is unset.
// Used instead of non-null assertions on process.env so a missing variable
// fails loudly at the point of use rather than silently passing `undefined`.
export function requireEnv(name: string): string {
	const value = process.env[name];
	if (!value) {
		throw new Error(`Missing required environment variable: ${name}`);
	}
	return value;
}
