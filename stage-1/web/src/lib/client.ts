/** The only HTTP client the screens may use.
 *
 * S1-G does not call it. Live auth, search, booking and lookup go through
 * this same `api` once a service exists. Refusals keep Keel's stable codes
 * because `keelErrorParser` reads `{ error: { code, message } }`.
 */
export { api, ApiError, keelErrorParser } from '@nrynss/chaaya/keel';
