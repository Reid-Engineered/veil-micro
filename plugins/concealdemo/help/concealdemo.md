# concealdemo

Demo plugin for the veil conceal API. In markdown buffers it shows task
checkboxes as glyphs:

    - [x] done   ->  ✔ done
    - [ ] todo   ->  ☐ todo

The raw markdown is shown on any line holding a cursor or touched by a
selection. Turn it off for a buffer with `set conceal off`, or disable the
plugin with `set concealdemo off`.

Colours come from the `comment` (done) and `special` (todo) colorscheme
groups.

The glyph is followed by `concealdemo.glyphpad` extra spaces (default `1`),
because fonts without `✔`/`☐` (such as Cascadia Mono) draw them from a
fallback font about two cells wide, which would cover the following space.
`set concealdemo.glyphpad 0` if your font draws them one cell wide.

