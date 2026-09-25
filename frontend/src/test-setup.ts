/**
 * Setup global des tests unitaires, exécuté avant les fichiers de specs.
 *
 * jsdom n'implémente pas Canvas : sans stub, chaque rendu du composant de
 * détail affiche « Not implemented: HTMLCanvasElement's getContext() » et
 * Chart.js échoue silencieusement. Le bruit finit par masquer de vrais
 * avertissements.
 *
 * Le stub rend ce comportement explicite et déterministe. Il ne permet pas de
 * vérifier ce qui est réellement dessiné — pour cela il faudrait le mode
 * navigateur (`--browsers=ChromeHeadless` avec `@vitest/browser-playwright`).
 * Les specs testent donc les données du graphique, pas son rendu.
 */
const canvasStub = {
  canvas: { width: 0, height: 0 },
  save: () => {},
  restore: () => {},
  scale: () => {},
  translate: () => {},
  rotate: () => {},
  clearRect: () => {},
  fillRect: () => {},
  strokeRect: () => {},
  beginPath: () => {},
  closePath: () => {},
  moveTo: () => {},
  lineTo: () => {},
  arc: () => {},
  bezierCurveTo: () => {},
  quadraticCurveTo: () => {},
  fill: () => {},
  stroke: () => {},
  clip: () => {},
  fillText: () => {},
  strokeText: () => {},
  measureText: () => ({ width: 0, actualBoundingBoxAscent: 0, actualBoundingBoxDescent: 0 }),
  setLineDash: () => {},
  getLineDash: () => [],
  setTransform: () => {},
  createLinearGradient: () => ({ addColorStop: () => {} }),
  drawImage: () => {},
  putImageData: () => {},
  getImageData: () => ({ data: new Uint8ClampedArray() }),
};

HTMLCanvasElement.prototype.getContext = (() =>
  canvasStub) as unknown as HTMLCanvasElement['getContext'];

/**
 * jsdom n'implémente pas non plus matchMedia, dont le menubar se sert pour
 * replier la navigation sur un écran étroit. Le stub répond « écran large » :
 * les specs voient la barre dépliée.
 */
if (!window.matchMedia) {
  window.matchMedia = ((query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addEventListener: () => {},
    removeEventListener: () => {},
    addListener: () => {},
    removeListener: () => {},
    dispatchEvent: () => false,
  })) as unknown as typeof window.matchMedia;
}
