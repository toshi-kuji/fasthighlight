[fasthighlight](index.html) · [Contents](contents.html)

# Themes and CSS

`fhl` writes the code colors as `highlight.css` (or into `<style>` with `--single-html`). The library writes no CSS; use the one from `fhl`, or your own.

## Choose a theme

```sh
fhl --theme dark:github docs/       # GitHub, dark colors only
fhl --theme light docs/             # default, light colors only
fhl --theme both:tokyo-night docs/  # Tokyo Night, light and dark
fhl --list-themes
fhl --print-css --theme both:github > highlight.css  # only the CSS, for your own build
```

Every theme has light and dark colors. `--theme MODE:THEME` chooses which to write and the theme:

| `MODE` | Writes |
| --- | --- |
| `light` | the light colors only |
| `dark` | the dark colors only |
| `both` | the light colors, switched to the dark ones when the viewer's device prefers dark (`prefers-color-scheme`) |

`THEME` can be left out for `default`; without `--theme`, it is `both:default`. `both` follows the viewer's device setting by default.

For a site with its own light and dark switch, `--theme-selector MODE:SELECTOR` adds the `MODE` colors (`light` or `dark`) under your CSS selector. The device setting stays the starting point, and your selector wins over it. To add selectors for both light and dark, repeat the flag in the same command, once for each selector (the CSS of both comes out of one run):

| What decides the colors | Flags |
| --- | --- |
| The device setting only | none |
| The device setting, then the site's switch (`<html data-theme="light">` or `"dark"`) | `--theme-selector dark:':root[data-theme="dark"]' --theme-selector light:':root[data-theme="light"]'` |
| The site's switch only (`<html data-theme="dark">` or nothing) | `--theme-selector dark:':root[data-theme="dark"]' --theme-selector light:':root:not([data-theme="dark"])'` |

Tip: to override the device setting completely, give the light colors a selector with `:not(…)` that matches whenever the dark one does not, as in the last row. Then the device setting never shows, even though its `@media (prefers-color-scheme: dark)` block is still in the CSS.

`--no-frame` leaves out the background and text color of `pre.highlight`, so that your own styles for code blocks apply.

```sh
fhl --print-css --theme both:github \
  --theme-selector dark:':root[data-theme="dark"]' \
  --theme-selector light:':root[data-theme="light"]' --no-frame > highlight.css
```

One selector can still match several things: `dark:':root[data-theme="dark"], .dark-mode'` puts the dark colors under both, as a comma does in CSS.

The default theme, `default`, has fhl's own colors: Go's brand colors (Gopher Blue, Aqua, Fuchsia, Yellow) with pinks, and bold keywords. Every color in it has a contrast ratio of at least 6 against its background (above WCAG AA). The other themes copy the colors of the original themes as they are, so some of their colors have a lower contrast.

<!-- THEMES -->

## Samples

The same code in each theme, in its light colors (this site is light only).

<!-- THEME-SAMPLES -->

## How the CSS is made

```css
:root {
  --highlight-keyword: #a3195b; /* keyword (if, func, etc.) */
  --highlight-string: #17663a; /* string */
  /* … */
}
/* nf: function name (Name.Function) */
pre.highlight .nf { color: var(--highlight-function); }
```

- The colors are CSS variables on `:root`. Every class gets its color from one of them.
- Every variable and every class has a comment that says what it is, so the CSS explains itself to people and AI agents.
- Selectors start with `pre.highlight`, so other uses of a `highlight` class on your page are not affected.
- A code block is `<pre class="highlight" data-lang="…"><code class="language-…">`, and each token is a `<span>` with its class. Lines and whitespace are not wrapped.

## CSS variables

<!-- CSS-VARS -->

## Token classes

The class names are the short ones used by Pygments, Chroma and Rouge, so CSS written for those mostly works too.

<!-- TOKEN-CLASSES -->

## Make your own colors

Change the variables. You do not need to touch the class rules:

```css
:root {
  --highlight-background: #fffdf5;
  --highlight-keyword: #b00040;
}
```

Load your file after `highlight.css`, or edit the variables in `highlight.css` itself. To color one class differently from the others that share its variable, add a rule for that class, such as `pre.highlight .nb { color: … }`.

This site's samples above use the same idea: the variables of each theme are put under a class (`.theme-github` and so on) instead of `:root`, so each sample takes its own colors.
