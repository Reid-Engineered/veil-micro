# kanban

Draws markdown kanban boards inline. Needs the veil fork of micro (the
`conceal` option and virtual lines).

## Boards

Either a whole Obsidian Kanban file (front matter with `kanban-plugin:`), or a
fenced block anywhere in a markdown file:

    ```kanban
    ## Todo
    - [ ] write the parser
    ## Done
    - [x] storage
    ```

`## Heading` starts a column. `- [ ] text`, `- [x] text` or a plain `- text`
is a card; indented lines under a card belong to it.

While no cursor is inside a board, it is drawn as side-by-side columns. Move
the cursor into it (or click it) to edit the raw markdown.

The board stays in raw markdown while the cursor is within `kanban.margin`
lines of it (default `1`), so stepping one line past the board doesn't redraw
it right away; it redraws when you move two lines away. `set kanban.margin 0`
redraws as soon as you leave the board's own lines; open boards update
immediately.

## Keys and commands

With the cursor on a card (the board shows its markdown then):

| Key       | Command          | Action                                      |
|-----------|------------------|---------------------------------------------|
| Alt-Right | `kanban right`   | move the card to the next column            |
| Alt-Left  | `kanban left`    | move the card to the previous column        |
| Alt-Up    | `kanban up`      | move the card up within its column          |
| Alt-Down  | `kanban down`    | move the card down within its column        |
| Alt-d     | `kanban toggle`  | toggle the card's checkbox                  |
|           | `kanban redraw`  | redraw boards (e.g. after a resize)         |

Anywhere else, the arrow keys keep their usual meaning (Alt-Up/Down move
lines; Alt-Left/Right are start-of-text and end-of-line, or word left/right on
macOS): the binding is a chain that falls back to the default action.

Every move or toggle is a single undo step.

Keys you have bound yourself in `bindings.json` are never replaced. To skip the
plugin's bindings entirely, run `set kanban.keys off` and restart, then bind
the actions yourself, e.g. in `bindings.json`:

    "Alt-Right": "lua:kanban.moveRight|EndOfLine"

Actions: `kanban.moveRight`, `kanban.moveLeft`, `kanban.moveUp`,
`kanban.moveDown`, `kanban.toggle`. Each returns false when the cursor is not
on a card, so they chain with `|`.

Disable the plugin with `set kanban off`.

## Colours

Borders use the `comment` colorscheme group, column titles and open
checkboxes use `special`, and done cards use `comment`.
