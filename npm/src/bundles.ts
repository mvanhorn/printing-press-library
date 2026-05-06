// Curated multi-CLI bundles. Each value is the list of catalog `name`s the
// bundle expands to. `printing-press install <bundle>` resolves the bundle and
// installs each entry's binary + focused skill, in order.
//
// Bundle names must not collide with catalog `name`s. When adding a CLI to the
// catalog whose name matches a bundle key here, rename the bundle.

export const BUNDLES: Record<string, readonly string[]> = {
  "starter-pack": ["espn", "flight-goat", "movie-goat", "recipe-goat"],
};

export function isBundle(name: string): boolean {
  return Object.prototype.hasOwnProperty.call(BUNDLES, name);
}
