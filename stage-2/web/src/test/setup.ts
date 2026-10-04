function mockMatchMedia(matches: (query: string) => boolean): void {
  Object.defineProperty(window, 'matchMedia', {
    writable: true,
    configurable: true,
    value: (query: string) => ({
      matches: matches(query),
      media: query,
      onchange: null,
      addListener() {},
      removeListener() {},
      addEventListener() {},
      removeEventListener() {},
      dispatchEvent() {
        return false;
      },
    }),
  });
}

mockMatchMedia(() => false);

if (typeof SVGElement !== 'undefined') {
  const svgPrototype = SVGElement.prototype as SVGElement & { click?: () => void };
  if (typeof svgPrototype.click !== 'function') {
    svgPrototype.click = function click(this: SVGElement): void {
      this.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true }));
    };
  }
}

if (typeof Element !== 'undefined') {
  Element.prototype.animate = function animate(): Animation {
    const animation = {
      onfinish: null as null | (() => void),
      oncancel: null as null | (() => void),
      cancel() {},
      finish() {},
      play() {},
      pause() {},
      reverse() {},
      persist() {},
      updatePlaybackRate() {},
      addEventListener() {},
      removeEventListener() {},
      dispatchEvent() {
        return false;
      },
      currentTime: 0,
      startTime: 0,
      playbackRate: 1,
      playState: 'finished' as AnimationPlayState,
      pending: false,
      effect: null,
      timeline: null,
      id: '',
      replaceState: 'active' as AnimationReplaceState,
    };
    queueMicrotask(() => {
      animation.onfinish?.();
    });
    return animation as unknown as Animation;
  };
}

if (!window.requestAnimationFrame) {
  let frame = 0;
  window.requestAnimationFrame = (callback: FrameRequestCallback): number => {
    frame += 1;
    const id = frame;
    setTimeout(() => callback(performance.now()), 16);
    return id;
  };
  window.cancelAnimationFrame = () => {};
}
