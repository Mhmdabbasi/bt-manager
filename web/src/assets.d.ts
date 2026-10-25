// Vite bundles CSS imports; this declaration tells TypeScript to accept them.
declare module '*.css';

declare module '*.css?url' {
  const href: string;
  export default href;
}
