import { mount, unmount } from 'svelte';
import type { Component, ComponentProps } from 'svelte';

export function render<T extends Component<any>>(component: T, props?: ComponentProps<T>) {
  const target = document.createElement('div');
  document.body.appendChild(target);
  const app = mount(component, { target, props: props as ComponentProps<T> });
  return {
    target,
    async cleanup() {
      await unmount(app);
      target.remove();
    },
  };
}
