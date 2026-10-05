/** The only HTTP client the screens may use.
 *
 * Live auth and search call this `api`. `keelErrorParser` reads
 * `{ error: { code, message } }`, so a refusal keeps its stable code.
 */
export { api, ApiError, keelErrorParser } from '@nrynss/chaaya/keel';
