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

## Keys and commands

| Key   | Command          | Action                                   |
|-------|------------------|------------------------------------------|
| Alt-l | `kanban right`   | move the card under the cursor right     |
| Alt-h | `kanban left`    | move the card under the cursor left      |
| Alt-t | `kanban toggle`  | toggle the card's checkbox               |
|       | `kanban redraw`  | redraw boards (e.g. after a resize)      |

Keys are only bound if you haven't bound them yourself. A move is one undo
step. Disable the plugin with `set kanban off`.

## Colours

Borders use the `comment` colorscheme group, column titles and open
checkboxes use `special`, and done cards use `comment`.
