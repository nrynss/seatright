import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';

/** Preprocess TypeScript inside Svelte components. The build stays a static Vite bundle. */
export default {
  preprocess: vitePreprocess(),
};
