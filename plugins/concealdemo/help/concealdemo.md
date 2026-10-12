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
