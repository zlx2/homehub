const root = document.querySelector<HTMLElement>("#app");
const basePath = (root?.dataset.basePath || "/drop").replace(/\/$/, "");

export function serviceURL(path: string): string {
  return `${basePath}${path.startsWith("/") ? path : `/${path}`}`;
}
